package cloudsync

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SyncStateRow 是 cloud_sync_state 表的一行。
// bucket_key 形如 "games/3"、"_singleton/categories"、"_manifest"。
type SyncStateRow struct {
	BucketKey        string
	LocalHash        string
	RemoteHash       string
	RemoteRevisionID string
	UpdatedAt        time.Time
}

// 在 cloud_sync_state 表中用于单文件 / manifest 自身的特殊 bucket_key 前缀
const (
	StateKeySingletonPrefix = "_singleton/"
	StateKeyManifest        = "_manifest"
)

// SingletonStateKey 返回单文件（categories / tombstones）在 state 表中的 key。
func SingletonStateKey(name string) string {
	return StateKeySingletonPrefix + name
}

// LoadSyncState 把整张 cloud_sync_state 表读到内存 map。
// 表为空时返回空 map（不返回 error）。
func LoadSyncState(ctx context.Context, db *sql.DB) (map[string]SyncStateRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT bucket_key, local_hash, remote_hash, remote_revision_id, updated_at FROM cloud_sync_state`)
	if err != nil {
		return nil, fmt.Errorf("query cloud_sync_state: %w", err)
	}
	defer rows.Close()

	out := make(map[string]SyncStateRow)
	for rows.Next() {
		var r SyncStateRow
		if err := rows.Scan(&r.BucketKey, &r.LocalHash, &r.RemoteHash, &r.RemoteRevisionID, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cloud_sync_state: %w", err)
		}
		out[r.BucketKey] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud_sync_state: %w", err)
	}
	return out, nil
}

// SaveSyncState 在单个事务内 upsert 所有给定行。
// 调用方应当只在 SyncNow 完整成功后调用这个函数，事务的原子性保证
// "全部成功才更新"，任何中途 error 都会让 cloud_sync_state 保持本次同步前的状态。
func SaveSyncState(ctx context.Context, db *sql.DB, rows []SyncStateRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud_sync_state tx: %w", err)
	}
	defer tx.Rollback()

	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO cloud_sync_state (bucket_key, local_hash, remote_hash, remote_revision_id, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (bucket_key) DO UPDATE SET
				local_hash = EXCLUDED.local_hash,
				remote_hash = EXCLUDED.remote_hash,
				remote_revision_id = EXCLUDED.remote_revision_id,
				updated_at = EXCLUDED.updated_at
		`, r.BucketKey, r.LocalHash, r.RemoteHash, r.RemoteRevisionID, r.UpdatedAt); err != nil {
			return fmt.Errorf("upsert cloud_sync_state %s: %w", r.BucketKey, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud_sync_state tx: %w", err)
	}
	return nil
}
