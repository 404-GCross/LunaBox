package cloudsync

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	EntityGame               = "game"
	EntityCategory           = "category"
	EntityGameCategory       = "game_category"
	EntityPlaySession        = "play_session"
	EntityGameProgress       = "game_progress"
	EntityGameReview         = "game_review"
	EntityGameTag            = "game_tag"
	EntityGameMetadataSource = "game_metadata_source"
)

type ExecContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func UpsertTombstone(ctx context.Context, exec ExecContexter, entityType, entityID string, deletedAt time.Time) error {
	if entityID == "" {
		return nil
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at)
		VALUES (?, ?, '', '', ?)
		ON CONFLICT (entity_type, entity_id, parent_id, secondary_id) DO UPDATE SET deleted_at = EXCLUDED.deleted_at
	`, entityType, entityID, deletedAt)
	if err != nil {
		return fmt.Errorf("upsert sync tombstone %s/%s: %w", entityType, entityID, err)
	}
	return nil
}

func DeleteTombstone(ctx context.Context, exec ExecContexter, entityType, entityID string) error {
	if entityID == "" {
		return nil
	}

	_, err := exec.ExecContext(ctx, `DELETE FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	if err != nil {
		return fmt.Errorf("delete sync tombstone %s/%s: %w", entityType, entityID, err)
	}
	return nil
}

func RelationTombstoneID(gameID, categoryID string) string {
	return gameID + "::" + categoryID
}

func TagTombstoneID(gameID, source, name string) string {
	return gameID + "::" + source + "::" + name
}

func MetadataSourceTombstoneID(gameID string, source string) string {
	return gameID + "::" + source
}
