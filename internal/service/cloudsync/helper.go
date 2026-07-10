package cloudsync

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/common/dto"
)

type Snapshot = dto.CloudSyncSnapshot
type Game = dto.CloudSyncGame
type Category = dto.CloudSyncCategory
type Relation = dto.CloudSyncRelation
type PlaySession = dto.CloudSyncPlaySession
type GameProgress = dto.CloudSyncGameProgress
type GameTag = dto.CloudSyncGameTag
type CoverAsset = dto.CloudSyncCoverAsset
type LocalCover = dto.CloudSyncLocalCover
type LocalState = dto.CloudSyncLocalState
type Tombstone = dto.CloudSyncTombstone
type Candidate = dto.CloudSyncCandidate
type Manifest = dto.CloudSyncManifest
type BucketRef = dto.CloudSyncBucketRef
type BucketFile = dto.CloudSyncBucketFile
type CoverRef = dto.CloudSyncCoverRef

const (
	SchemaVersion   = 1
	SchemaVersionV2 = 2

	// v1 全量快照路径（仅在迁移期使用）
	SnapshotKey = "sync/library/latest.json"

	LibraryDir = "sync/library"
	CoverDir   = "sync/covers"

	// v2 入口与单文件
	ManifestKey       = "sync/library/manifest.json"
	CategoriesFileKey = "sync/library/categories.json"
	TombstonesFileKey = "sync/library/tombstones.json"

	// v2 分桶：每个实体类型 16 个桶，按 game_id 首个 hex 字符路由
	BucketCount       = 16
	BucketHexAlphabet = "0123456789abcdef"

	// 桶 payload 上限（OneDrive 单 PUT 4MB；留 headroom）
	BucketSizeWarnBytes = int64(3_500_000)

	// 并发上限
	ConcurrencyOneDrive = 4
	ConcurrencyS3       = 16
	ConcurrencyUmbra    = 6

	entityGame         = "game"
	entityCategory     = "category"
	entityGameCategory = "game_category"
	entityPlaySession  = "play_session"
	entityGameProgress = "game_progress"
	entityGameTag      = "game_tag"

	// EntityKey 在 manifest.buckets 与 BucketContent 中的命名（snake_case）
	EntityKeyGames          = "games"
	EntityKeyPlaySessions   = "play_sessions"
	EntityKeyGameProgresses = "game_progresses"
	EntityKeyGameTags       = "game_tags"
	EntityKeyGameCategories = "game_categories"

	// Singleton key
	SingletonCategories = "categories"
	SingletonTombstones = "tombstones"

	systemFavoritesCategoryID = "system:favorites"
)

// EntitySubDirs 给出每个实体类型对应的远端子目录名（相对 LibraryDir）。
// SyncNow 启动时一次性 EnsureDir 这些目录（OneDrive 路径成本最大化收敛）。
var EntitySubDirs = map[string]string{
	EntityKeyGames:          "games",
	EntityKeyPlaySessions:   "play_sessions",
	EntityKeyGameProgresses: "game_progresses",
	EntityKeyGameTags:       "game_tags",
	EntityKeyGameCategories: "game_categories",
}

// EntityKeys 返回稳定顺序的实体类型列表，便于在 diff/sort 中产生确定性结果。
func EntityKeys() []string {
	return []string{
		EntityKeyGames,
		EntityKeyPlaySessions,
		EntityKeyGameProgresses,
		EntityKeyGameTags,
		EntityKeyGameCategories,
	}
}

type Helper struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewHelper(ctx context.Context, db *sql.DB, config *appconf.AppConfig) *Helper {
	return &Helper{
		ctx:    ctx,
		db:     db,
		config: config,
	}
}

func relationTombstoneID(gameID, categoryID string) string {
	return gameID + "::" + categoryID
}

func tagTombstoneID(gameID, source, name string) string {
	return gameID + "::" + source + "::" + name
}
