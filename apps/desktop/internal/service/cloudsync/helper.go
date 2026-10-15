package cloudsync

import (
	"context"
	"database/sql"
	"sync"

	"lunabox/internal/appconf"
	"lunabox/internal/common/dto"
	"lunabox/internal/service/gamehelper"
)

type Snapshot = dto.CloudSyncSnapshot
type Game = dto.CloudSyncGame
type Category = dto.CloudSyncCategory
type Relation = dto.CloudSyncRelation
type PlaySession = dto.CloudSyncPlaySession
type GameProgress = dto.CloudSyncGameProgress
type GameReview = dto.CloudSyncGameReview
type GameTag = dto.CloudSyncGameTag
type MetadataSource = dto.CloudSyncGameMetadataSource
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
	SchemaVersion   = 4
	SchemaVersionV2 = 2
	SchemaVersionV3 = 3
	SchemaVersionV4 = 4

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

	entityGame               = EntityGame
	entityCategory           = EntityCategory
	entityGameCategory       = EntityGameCategory
	entityPlaySession        = EntityPlaySession
	entityGameProgress       = EntityGameProgress
	entityGameReview         = EntityGameReview
	entityGameTag            = EntityGameTag
	entityGameMetadataSource = EntityGameMetadataSource

	// EntityKey 在 manifest.buckets 与 BucketContent 中的命名（snake_case）
	EntityKeyGames               = "games"
	EntityKeyPlaySessions        = "play_sessions"
	EntityKeyGameProgresses      = "game_progresses"
	EntityKeyGameReviews         = "game_reviews"
	EntityKeyGameTags            = "game_tags"
	EntityKeyGameCategories      = "game_categories"
	EntityKeyGameMetadataSources = "game_metadata_sources"

	// Singleton key
	SingletonCategories = "categories"
	SingletonTombstones = "tombstones"

	systemFavoritesCategoryID = gamehelper.SystemFavoritesCategoryID
)

// EntitySubDirs 给出每个实体类型对应的远端子目录名（相对 LibraryDir）。
// SyncNow 启动时一次性 EnsureDir 这些目录（OneDrive 路径成本最大化收敛）。
var EntitySubDirs = map[string]string{
	EntityKeyGames:               "games",
	EntityKeyPlaySessions:        "play_sessions",
	EntityKeyGameProgresses:      "game_progresses",
	EntityKeyGameReviews:         "game_reviews",
	EntityKeyGameTags:            "game_tags",
	EntityKeyGameCategories:      "game_categories",
	EntityKeyGameMetadataSources: "game_metadata_sources",
}

// EntityKeys 返回稳定顺序的实体类型列表，便于在 diff/sort 中产生确定性结果。
func EntityKeys() []string {
	return []string{
		EntityKeyGames,
		EntityKeyPlaySessions,
		EntityKeyGameProgresses,
		EntityKeyGameReviews,
		EntityKeyGameTags,
		EntityKeyGameCategories,
		EntityKeyGameMetadataSources,
	}
}

type Helper struct {
	ctx      context.Context
	db       *sql.DB
	config   *appconf.AppConfig
	progress func(stage, detail string, current, total int)
}

func NewHelper(ctx context.Context, db *sql.DB, config *appconf.AppConfig) *Helper {
	return &Helper{
		ctx:    ctx,
		db:     db,
		config: config,
	}
}

// SetProgressReporter lets the service surface human-readable sync phases.
func (h *Helper) SetProgressReporter(progress func(stage, detail string, current, total int)) {
	h.progress = progress
}

func (h *Helper) reportProgress(stage, detail string, current, total int) {
	if h.progress != nil {
		h.progress(stage, detail, current, total)
	}
}

func (h *Helper) startCountedProgress(stage, detail string, total int) func(int) {
	h.reportProgress(stage, detail, 0, total)
	var mu sync.Mutex
	current := 0
	return func(completed int) {
		mu.Lock()
		defer mu.Unlock()
		current = min(current+completed, total)
		h.reportProgress(stage, detail, current, total)
	}
}
