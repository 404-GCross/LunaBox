package importer

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/importpath"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/cloudsync"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/dbutils"
	"lunabox/internal/utils/metadata"
	"strings"
	"sync"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
)

const (
	importStatusNew               = "new"
	importStatusExistsPath        = "exists_path"
	importStatusExistsSource      = "exists_source"
	importStatusExistsNamePath    = "exists_name_path"
	importStatusPossibleDuplicate = "possible_duplicate"
	importCoverWorkerCount        = 4
)

type GameRef struct {
	ID         string
	Name       string
	Path       string
	SourceType enums.SourceType
	SourceID   string
	CreatedAt  time.Time
}

func gameRefFromModel(game models.Game) GameRef {
	return GameRef{
		ID:         game.ID,
		Name:       game.Name,
		Path:       game.Path,
		SourceType: game.SourceType,
		SourceID:   game.SourceID,
		CreatedAt:  game.CreatedAt,
	}
}

func (ref GameRef) game() models.Game {
	return models.Game{
		ID:         ref.ID,
		Name:       ref.Name,
		Path:       ref.Path,
		SourceType: ref.SourceType,
		SourceID:   ref.SourceID,
		CreatedAt:  ref.CreatedAt,
	}
}

type Index struct {
	byPath     map[string]GameRef
	bySource   map[string]GameRef
	byNamePath map[string]GameRef
	byName     map[string]GameRef
}

type CommitItem struct {
	Game                    models.Game
	Tags                    []metadata.TagItem
	Sessions                []models.PlaySession
	Source                  enums.SourceType
	Action                  string
	UpdateLocalLaunchFields bool
	CoverLoader             func(models.Game) (string, error)
}

type CoverDownloadItem struct {
	GameID   string
	GameName string
	CoverURL string
}

type CommitDependencies struct {
	Ctx                    context.Context
	DB                     *sql.DB
	AllowDuplicateMetadata bool
	StartCoverDownloads    func([]CoverDownloadItem) string
	UpdateCoverURL         func(gameID string, coverURL string) error
}

type Committer struct {
	ctx                    context.Context
	db                     *sql.DB
	allowDuplicateMetadata bool
	startCoverDownloads    func([]CoverDownloadItem) string
	updateCoverURL         func(gameID string, coverURL string) error
}

func NewCommitter(deps CommitDependencies) *Committer {
	return &Committer{
		ctx:                    deps.Ctx,
		db:                     deps.DB,
		allowDuplicateMetadata: deps.AllowDuplicateMetadata,
		startCoverDownloads:    deps.StartCoverDownloads,
		updateCoverURL:         deps.UpdateCoverURL,
	}
}

func importPathContainsNormalized(parentPath string, childPath string) bool {
	return importpath.ContainsNormalized(parentPath, childPath)
}

func normalizeImportName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func importSourceKey(source enums.SourceType, sourceID string) string {
	sourceID = strings.ToLower(strings.TrimSpace(sourceID))
	if source == "" || sourceID == "" {
		return ""
	}
	return strings.ToLower(string(source)) + "\x00" + sourceID
}

func importNamePathKey(name string, path string) string {
	nameKey := normalizeImportName(name)
	pathKey := normalizeImportPath(path)
	if nameKey == "" || pathKey == "" {
		return ""
	}
	return nameKey + "\x00" + pathKey
}

func NewIndex(refs []GameRef) Index {
	idx := Index{
		byPath:     make(map[string]GameRef, len(refs)),
		bySource:   make(map[string]GameRef, len(refs)),
		byNamePath: make(map[string]GameRef, len(refs)),
		byName:     make(map[string]GameRef, len(refs)),
	}

	for _, ref := range refs {
		idx.Add(ref)
	}
	return idx
}

func (idx Index) Add(ref GameRef) {
	if key := normalizeImportPath(ref.Path); key != "" {
		if _, exists := idx.byPath[key]; !exists {
			idx.byPath[key] = ref
		}
	}
	if key := importSourceKey(ref.SourceType, ref.SourceID); key != "" {
		if _, exists := idx.bySource[key]; !exists {
			idx.bySource[key] = ref
		}
	}
	if key := importNamePathKey(ref.Name, ref.Path); key != "" {
		if _, exists := idx.byNamePath[key]; !exists {
			idx.byNamePath[key] = ref
		}
	}
	if key := normalizeImportName(ref.Name); key != "" {
		if _, exists := idx.byName[key]; !exists {
			idx.byName[key] = ref
		}
	}
}

func (idx Index) FindByPath(path string) (GameRef, bool) {
	ref, ok := idx.byPath[normalizeImportPath(path)]
	return ref, ok
}

func (idx Index) FindByPathConflict(path string) (GameRef, bool) {
	pathKey := normalizeImportPath(path)
	if pathKey == "" {
		return GameRef{}, false
	}

	if ref, ok := idx.byPath[pathKey]; ok {
		return ref, true
	}

	for existingPath, ref := range idx.byPath {
		if importPathContainsNormalized(pathKey, existingPath) || importPathContainsNormalized(existingPath, pathKey) {
			return ref, true
		}
	}
	return GameRef{}, false
}

func (idx Index) FindBySource(source enums.SourceType, sourceID string) (GameRef, bool) {
	key := importSourceKey(source, sourceID)
	if key == "" {
		return GameRef{}, false
	}
	ref, ok := idx.bySource[key]
	return ref, ok
}

func (idx Index) FindByNamePath(name string, path string) (GameRef, bool) {
	key := importNamePathKey(name, path)
	if key == "" {
		return GameRef{}, false
	}
	ref, ok := idx.byNamePath[key]
	return ref, ok
}

func (idx Index) FindByName(name string) (GameRef, bool) {
	ref, ok := idx.byName[normalizeImportName(name)]
	return ref, ok
}

func (c *Committer) listGameRefs() ([]GameRef, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := c.db.QueryContext(c.ctx, `
		SELECT
			g.id,
			COALESCE(g.name, ''),
			COALESCE(g.path, ''),
			COALESCE(s.source_type, g.source_type, ''),
			COALESCE(s.source_id, g.source_id, ''),
			COALESCE(g.created_at, g.cached_at, g.updated_at, CURRENT_TIMESTAMP)
		FROM games g
		LEFT JOIN game_metadata_sources s ON s.game_id = g.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query import game refs: %w", err)
	}
	defer rows.Close()

	refs := make([]GameRef, 0)
	for rows.Next() {
		var ref GameRef
		var sourceType string
		if err := rows.Scan(&ref.ID, &ref.Name, &ref.Path, &sourceType, &ref.SourceID, &ref.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan import game ref: %w", err)
		}
		ref.SourceType = enums.SourceType(sourceType)
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import game refs: %w", err)
	}
	return refs, nil
}

func (c *Committer) LoadIndex() (Index, error) {
	refs, err := c.listGameRefs()
	if err != nil {
		return Index{}, err
	}
	return NewIndex(refs), nil
}

func (c *Committer) ListGames() ([]models.Game, error) {
	refs, err := c.listGameRefs()
	if err != nil {
		return nil, err
	}
	games := make([]models.Game, 0, len(refs))
	for _, ref := range refs {
		games = append(games, models.Game{
			ID:         ref.ID,
			Name:       ref.Name,
			Path:       ref.Path,
			SourceType: ref.SourceType,
			SourceID:   ref.SourceID,
			CreatedAt:  ref.CreatedAt,
		})
	}
	return games, nil
}

func (c *Committer) AddItems(items []ImportItem) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}
	if len(items) == 0 {
		return result, nil
	}

	startedAt := time.Now()
	stepStartedAt := time.Now()
	idx, err := c.LoadIndex()
	if err != nil {
		return result, fmt.Errorf("加载导入索引失败: %w", err)
	}
	applog.LogInfof(c.ctx, "addImporterItems: loaded import index for items=%d elapsed=%s", len(items), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	toCommit := make([]CommitItem, 0, len(items))
	for _, item := range items {
		game := item.Source.Game
		displayName := item.DisplayName
		if displayName == "" {
			displayName = game.Name
		}
		if item.Path == "" {
			item.Path = game.Path
		}

		source := item.Source.Source
		if source == "" {
			source = game.SourceType
		}
		if game.SourceType == "" {
			game.SourceType = source
		}
		if game.ID == "" {
			game.ID = uuid.New().String()
		}

		action := item.Action
		if action == "" {
			action = ImportActionCreate
		}
		if TargetsExistingGame(action) {
			if item.ExistingGameID == "" {
				result.Failed++
				result.FailedNames = append(result.FailedNames, displayName)
				continue
			}
			game.ID = item.ExistingGameID
			for i := range item.Sessions {
				item.Sessions[i].GameID = item.ExistingGameID
			}
		} else {
			if ref, ok := idx.FindByPathConflict(item.Path); ok {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, displayName+" (路径已存在: "+ref.Name+")")
				continue
			}
		}

		if !c.allowDuplicateMetadata {
			sourceRefs := append([]models.GameMetadataSource(nil), game.MetadataSources...)
			if len(sourceRefs) == 0 {
				sourceRefs = append(sourceRefs, models.GameMetadataSource{SourceType: source, SourceID: game.SourceID})
			}
			duplicateFound := false
			for _, sourceRef := range sourceRefs {
				if ref, ok := idx.FindBySource(sourceRef.SourceType, sourceRef.SourceID); ok && (!TargetsExistingGame(action) || ref.ID != item.ExistingGameID) {
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, displayName+" (元数据已存在: "+ref.Name+")")
					duplicateFound = true
					break
				}
			}
			if duplicateFound {
				continue
			}
		}
		if ref, ok := idx.FindByNamePath(game.Name, item.Path); ok && (!TargetsExistingGame(action) || ref.ID != item.ExistingGameID) {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, displayName+" (已存在: "+ref.Name+")")
			continue
		}

		converted := CommitItem{
			Game:                    game,
			Tags:                    item.Source.Tags,
			Sessions:                item.Sessions,
			Source:                  source,
			Action:                  action,
			UpdateLocalLaunchFields: item.UpdateLocalLaunchFields,
			CoverLoader:             item.CoverLoader,
		}
		toCommit = append(toCommit, converted)
		if action == ImportActionCreate {
			baseRef := GameRef{
				ID:         game.ID,
				Name:       game.Name,
				Path:       game.Path,
				SourceType: game.SourceType,
				SourceID:   game.SourceID,
				CreatedAt:  game.CreatedAt,
			}
			idx.Add(baseRef)
			for _, sourceRef := range game.MetadataSources {
				baseRef.SourceType = sourceRef.SourceType
				baseRef.SourceID = sourceRef.SourceID
				idx.Add(baseRef)
			}
		}
	}
	applog.LogInfof(c.ctx, "addImporterItems: filtered commit_items=%d skipped=%d elapsed=%s", len(toCommit), result.Skipped, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	success, sessionsImported, err := c.CommitItems(toCommit)
	if err != nil {
		result.Failed += len(toCommit)
		for _, item := range toCommit {
			result.FailedNames = append(result.FailedNames, item.Game.Name)
		}
		return result, err
	}
	result.Success += success
	result.SessionsImported += sessionsImported
	applog.LogInfof(c.ctx, "addImporterItems: committed success=%d skipped=%d failed=%d sessions=%d elapsed=%s total=%s", result.Success, result.Skipped, result.Failed, result.SessionsImported, time.Since(stepStartedAt), time.Since(startedAt))
	return result, nil
}

func AnnotateScanCandidate(candidate vo.BatchImportCandidate, idx Index, allowDuplicateMetadata bool) vo.BatchImportCandidate {
	candidate.ImportStatus = importStatusNew
	candidate.IsSelected = true

	pathsToCheck := candidate.Executables
	if len(pathsToCheck) == 0 && candidate.SelectedExe != "" {
		pathsToCheck = []string{candidate.SelectedExe}
	}
	for _, exePath := range pathsToCheck {
		if ref, ok := idx.FindByPathConflict(exePath); ok {
			candidate.ImportStatus = importStatusExistsPath
			candidate.IsSelected = false
			candidate.ExistingID = ref.ID
			candidate.ExistingName = ref.Name
			candidate.SkipReason = "路径已存在: " + ref.Name
			return candidate
		}
	}

	if ref, ok := idx.FindByPathConflict(candidate.SelectedExe); ok {
		candidate.ImportStatus = importStatusExistsPath
		candidate.IsSelected = false
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "路径已存在: " + ref.Name
		return candidate
	}

	if ref, ok := idx.FindByNamePath(candidate.SearchName, candidate.SelectedExe); ok {
		candidate.ImportStatus = importStatusExistsNamePath
		candidate.IsSelected = false
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "已存在: " + ref.Name
		return candidate
	}

	if !allowDuplicateMetadata {
		if ref, ok := idx.FindBySource(candidate.SourceType, candidate.SourceID); ok {
			candidate.ImportStatus = importStatusExistsSource
			candidate.IsSelected = false
			candidate.ExistingID = ref.ID
			candidate.ExistingName = ref.Name
			candidate.SkipReason = "元数据已存在: " + ref.Name
			return candidate
		}
	}

	if ref, ok := idx.FindByName(candidate.SearchName); ok && normalizeImportPath(ref.Path) != normalizeImportPath(candidate.SelectedExe) {
		candidate.ImportStatus = importStatusPossibleDuplicate
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "存在同名游戏: " + ref.Name
	}

	return candidate
}

func SplitScanCandidates(candidates []vo.BatchImportCandidate, idx Index, allowDuplicateMetadata ...bool) vo.BatchImportScanResult {
	result := vo.BatchImportScanResult{
		Candidates:        make([]vo.BatchImportCandidate, 0, len(candidates)),
		SkippedCandidates: make([]vo.BatchImportCandidate, 0),
		TotalDetected:     len(candidates),
	}

	allowDuplicate := false
	if len(allowDuplicateMetadata) > 0 {
		allowDuplicate = allowDuplicateMetadata[0]
	}

	for _, candidate := range candidates {
		annotated := AnnotateScanCandidate(candidate, idx, allowDuplicate)
		switch annotated.ImportStatus {
		case importStatusExistsPath, importStatusExistsNamePath, importStatusExistsSource:
			result.SkippedCandidates = append(result.SkippedCandidates, annotated)
		default:
			result.Candidates = append(result.Candidates, annotated)
		}
	}
	result.Skipped = len(result.SkippedCandidates)
	return result
}

func appendImportRows(ctx context.Context, conn *sql.Conn, table string, appendRows func(*duckdb.Appender) error) error {
	return conn.Raw(func(driverConn any) error {
		duckConn, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("duckdb raw connection has unexpected type %T", driverConn)
		}
		appender, err := duckdb.NewAppenderFromConn(duckConn, "", table)
		if err != nil {
			return fmt.Errorf("create appender for %s: %w", table, err)
		}
		if err := appendRows(appender); err != nil {
			_ = appender.Close()
			return err
		}
		if err := appender.Close(); err != nil {
			return fmt.Errorf("close appender for %s: %w", table, err)
		}
		return nil
	})
}

func splitCommitItemsByAction(items []CommitItem) ([]CommitItem, []CommitItem) {
	createItems := make([]CommitItem, 0, len(items))
	updateItems := make([]CommitItem, 0)
	for _, item := range items {
		switch item.Action {
		case ImportActionUpdateExisting:
			updateItems = append(updateItems, item)
		case ImportActionCreate:
			createItems = append(createItems, item)
		}
	}
	return createItems, updateItems
}

func commitItemsWithMetadata(items []CommitItem) []CommitItem {
	filtered := make([]CommitItem, 0, len(items))
	for _, item := range items {
		if item.Action != ImportActionMergeSessions {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (c *Committer) addImportedItems(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_games`); err != nil {
		return 0, fmt.Errorf("drop temp_import_games: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_games (
		id TEXT,
		name TEXT,
		cover_url TEXT,
		cover_source_url TEXT,
		company TEXT,
		summary TEXT,
		rating DOUBLE,
		release_date TEXT,
		path TEXT,
		game_directory TEXT,
		save_path TEXT,
		process_name TEXT,
		wine_runner TEXT,
		wine_args TEXT,
		wine_prefix TEXT,
		launch_mode TEXT,
		steam_launch_id TEXT,
		steam_launch_kind TEXT,
		steam_user_id TEXT,
		steam_launch_options TEXT,
		source_type TEXT,
		cached_at TIMESTAMPTZ,
		source_id TEXT,
		created_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ,
		use_locale_emulator BOOLEAN,
		use_magpie BOOLEAN,
		is_nsfw BOOLEAN
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_games: %w", err)
	}

	now := time.Now()
	if err := appendImportRows(ctx, conn, "temp_import_games", func(appender *duckdb.Appender) error {
		for i := range items {
			game := items[i].Game
			if game.ID == "" {
				game.ID = uuid.New().String()
			}
			if game.CreatedAt.IsZero() {
				game.CreatedAt = now
			}
			if game.CachedAt.IsZero() {
				game.CachedAt = now
			}
			if game.UpdatedAt.IsZero() {
				game.UpdatedAt = now
			}
			if game.SourceType == "" {
				game.SourceType = items[i].Source
			}
			if game.GameDirectory == "" {
				game.GameDirectory = gamehelper.DefaultGameDirectory(game.Path)
			}
			if game.CoverSourceURL == "" && gamehelper.IsDownloadableCoverURL(game.CoverURL) {
				game.CoverSourceURL = strings.TrimSpace(game.CoverURL)
			}
			game.LaunchMode = enums.NormalizeLaunchMode(game.LaunchMode)
			items[i].Game = game

			if err := appender.AppendRow(
				game.ID,
				game.Name,
				game.CoverURL,
				game.CoverSourceURL,
				game.Company,
				game.Summary,
				game.Rating,
				game.ReleaseDate,
				game.Path,
				game.GameDirectory,
				game.SavePath,
				game.ProcessName,
				game.WineRunner,
				game.WineArgs,
				game.WinePrefix,
				string(game.LaunchMode),
				game.SteamLaunchID,
				game.SteamLaunchKind,
				game.SteamUserID,
				game.SteamLaunchOptions,
				string(game.SourceType),
				game.CachedAt,
				game.SourceID,
				game.CreatedAt,
				game.UpdatedAt,
				game.UseLocaleEmulator,
				game.UseMagpie,
				game.IsNSFW,
			); err != nil {
				return fmt.Errorf("append imported game %s: %w", game.Name, err)
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO games (
		id, name, cover_url, cover_source_url, company, summary, rating, release_date, path, game_directory,
		save_path, process_name, wine_runner, wine_args, wine_prefix, launch_mode,
		steam_launch_id, steam_launch_kind, steam_user_id, steam_launch_options,
		source_type, cached_at, source_id, created_at, updated_at,
		use_locale_emulator, use_magpie, is_nsfw
	)
	SELECT
		id, name, cover_url, cover_source_url, company, summary, rating, release_date, path, game_directory,
		save_path, process_name, wine_runner, wine_args, wine_prefix, launch_mode,
		steam_launch_id, steam_launch_kind, steam_user_id, steam_launch_options,
		source_type, cached_at, source_id, created_at, updated_at,
		use_locale_emulator, use_magpie, is_nsfw
	FROM temp_import_games`); err != nil {
		return 0, fmt.Errorf("insert imported games from staging: %w", err)
	}

	return len(items), nil
}

func (c *Committer) updateImportedItemMetadata(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_update_import_games`); err != nil {
		return 0, fmt.Errorf("drop temp_update_import_games: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_update_import_games (
		id TEXT,
		name TEXT,
		cover_url TEXT,
		cover_source_url TEXT,
		company TEXT,
		summary TEXT,
		rating DOUBLE,
		release_date TEXT,
		source_type TEXT,
		cached_at TIMESTAMPTZ,
		source_id TEXT,
		updated_at TIMESTAMPTZ,
		is_nsfw BOOLEAN
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_update_import_games: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_update_import_games", func(appender *duckdb.Appender) error {
		for i := range items {
			game := items[i].Game
			if game.ID == "" {
				continue
			}
			if game.CachedAt.IsZero() {
				game.CachedAt = now
			}
			if game.UpdatedAt.IsZero() {
				game.UpdatedAt = now
			}
			if game.SourceType == "" {
				game.SourceType = items[i].Source
			}
			if game.CoverSourceURL == "" && gamehelper.IsDownloadableCoverURL(game.CoverURL) {
				game.CoverSourceURL = strings.TrimSpace(game.CoverURL)
			}
			items[i].Game = game
			if err := appender.AppendRow(
				game.ID,
				game.Name,
				game.CoverURL,
				game.CoverSourceURL,
				game.Company,
				game.Summary,
				game.Rating,
				game.ReleaseDate,
				string(game.SourceType),
				game.CachedAt,
				game.SourceID,
				game.UpdatedAt,
				game.IsNSFW,
			); err != nil {
				return fmt.Errorf("append import metadata update %s: %w", game.Name, err)
			}
			inserted++
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if inserted == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `
		UPDATE games
		SET
			name = temp_update_import_games.name,
			cover_url = CASE
				WHEN temp_update_import_games.cover_url <> '' THEN temp_update_import_games.cover_url
				ELSE games.cover_url
			END,
			cover_source_url = CASE
				WHEN temp_update_import_games.cover_source_url <> '' THEN temp_update_import_games.cover_source_url
				ELSE games.cover_source_url
			END,
			company = temp_update_import_games.company,
			summary = temp_update_import_games.summary,
			rating = temp_update_import_games.rating,
			release_date = temp_update_import_games.release_date,
			source_type = temp_update_import_games.source_type,
			cached_at = temp_update_import_games.cached_at,
			source_id = temp_update_import_games.source_id,
			updated_at = temp_update_import_games.updated_at,
			is_nsfw = CASE
				WHEN temp_update_import_games.source_type IN ('bangumi', 'vndb') THEN temp_update_import_games.is_nsfw
				ELSE games.is_nsfw
			END
		FROM temp_update_import_games
		WHERE games.id = temp_update_import_games.id
	`); err != nil {
		return inserted, fmt.Errorf("update imported game metadata from staging: %w", err)
	}

	return inserted, nil
}

func (c *Committer) upsertImportedItemMetadataSources(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	count := 0
	now := time.Now()
	for _, item := range items {
		game := item.Game
		sources := append([]models.GameMetadataSource(nil), game.MetadataSources...)
		if len(sources) == 0 && game.SourceType != "" && game.SourceType != enums.Local && strings.TrimSpace(game.SourceID) != "" {
			sources = append(sources, models.GameMetadataSource{SourceType: game.SourceType, SourceID: game.SourceID})
		}
		seen := make(map[enums.SourceType]struct{}, len(sources))
		for _, source := range sources {
			sourceType, sourceID, normalizeErr := gamehelper.NormalizeMetadataSource(source.SourceType, source.SourceID)
			if normalizeErr != nil {
				continue
			}
			if _, exists := seen[sourceType]; exists {
				continue
			}
			seen[sourceType] = struct{}{}
			cachedAt := source.CachedAt
			if cachedAt.IsZero() {
				cachedAt = game.CachedAt
			}
			if cachedAt.IsZero() {
				cachedAt = now
			}
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO game_metadata_sources (
					game_id, source_type, source_id, cached_at, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (game_id, source_type) DO UPDATE SET
					source_id = EXCLUDED.source_id,
					cached_at = EXCLUDED.cached_at,
					updated_at = EXCLUDED.updated_at
			`, game.ID, string(sourceType), sourceID, cachedAt, now, now); err != nil {
				return count, fmt.Errorf("upsert imported metadata source %s for %s: %w", sourceType, game.Name, err)
			}
			if err := cloudsync.DeleteTombstone(ctx, conn, cloudsync.EntityGameMetadataSource, cloudsync.MetadataSourceTombstoneID(game.ID, string(sourceType))); err != nil {
				return count, err
			}
			count++
		}

		defaultSource := game.SourceType
		if defaultSource == enums.Local {
			defaultSource = ""
		}
		if defaultSource != "" {
			for _, source := range sources {
				if gamehelper.NormalizeMetadataSourceType(source.SourceType) != gamehelper.NormalizeMetadataSourceType(defaultSource) {
					continue
				}
				if _, err := conn.ExecContext(ctx, `
					UPDATE games SET source_type = ?, source_id = ? WHERE id = ?
				`, string(defaultSource), strings.TrimSpace(source.SourceID), game.ID); err != nil {
					return count, fmt.Errorf("set imported default metadata source for %s: %w", game.Name, err)
				}
				break
			}
		}
	}
	return count, nil
}

func (c *Committer) updateImportedItemLocalLaunchFields(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	hasUpdates := false
	for _, item := range items {
		if item.UpdateLocalLaunchFields && item.Game.ID != "" {
			hasUpdates = true
			break
		}
	}
	if !hasUpdates {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_update_import_launch_fields`); err != nil {
		return 0, fmt.Errorf("drop temp_update_import_launch_fields: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_update_import_launch_fields (
		id TEXT,
		path TEXT,
		game_directory TEXT,
		process_name TEXT,
		launch_mode TEXT,
		steam_launch_id TEXT,
		steam_launch_kind TEXT,
		steam_user_id TEXT
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_update_import_launch_fields: %w", err)
	}

	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_update_import_launch_fields", func(appender *duckdb.Appender) error {
		for _, item := range items {
			if !item.UpdateLocalLaunchFields || item.Game.ID == "" {
				continue
			}
			game := item.Game
			if game.GameDirectory == "" {
				game.GameDirectory = gamehelper.DefaultGameDirectory(game.Path)
			}
			game.LaunchMode = enums.NormalizeLaunchMode(game.LaunchMode)
			if err := appender.AppendRow(
				game.ID,
				game.Path,
				game.GameDirectory,
				game.ProcessName,
				string(game.LaunchMode),
				game.SteamLaunchID,
				game.SteamLaunchKind,
				game.SteamUserID,
			); err != nil {
				return fmt.Errorf("append import launch field update %s: %w", game.Name, err)
			}
			inserted++
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if inserted == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `
		UPDATE games
		SET
			path = temp_update_import_launch_fields.path,
			game_directory = temp_update_import_launch_fields.game_directory,
			process_name = temp_update_import_launch_fields.process_name,
			launch_mode = temp_update_import_launch_fields.launch_mode,
			steam_launch_id = temp_update_import_launch_fields.steam_launch_id,
			steam_launch_kind = temp_update_import_launch_fields.steam_launch_kind,
			steam_user_id = temp_update_import_launch_fields.steam_user_id
		FROM temp_update_import_launch_fields
		WHERE games.id = temp_update_import_launch_fields.id
		  AND COALESCE(TRIM(games.path), '') = ''
	`); err != nil {
		return inserted, fmt.Errorf("update imported game launch fields from staging: %w", err)
	}

	return inserted, nil
}

func (c *Committer) addImportedItemTags(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	total := 0
	for _, item := range items {
		total += len(item.Tags)
	}
	if total == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_game_tags`); err != nil {
		return 0, fmt.Errorf("drop temp_import_game_tags: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_game_tags (
		id TEXT,
		game_id TEXT,
		name TEXT,
		source TEXT,
		weight DOUBLE,
		is_spoiler BOOLEAN,
		created_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_game_tags: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_import_game_tags", func(appender *duckdb.Appender) error {
		for _, item := range items {
			for _, tag := range item.Tags {
				name := strings.TrimSpace(tag.Name)
				source := strings.TrimSpace(tag.Source)
				if name == "" {
					continue
				}
				if source == "" {
					source = "user"
				}
				id := uuid.New().String()
				if err := appender.AppendRow(id, item.Game.ID, name, source, tag.Weight, tag.IsSpoiler, now, now); err != nil {
					return fmt.Errorf("append imported tag %s for %s: %w", name, item.Game.Name, err)
				}
				inserted++
			}
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		SELECT id, game_id, name, source, weight, is_spoiler, created_at, updated_at
		FROM temp_import_game_tags
		ON CONFLICT (game_id, name, source) DO UPDATE SET
			id = EXCLUDED.id,
			weight = EXCLUDED.weight,
			is_spoiler = EXCLUDED.is_spoiler,
			updated_at = EXCLUDED.updated_at
	`); err != nil {
		return inserted, fmt.Errorf("insert imported tags from staging: %w", err)
	}
	return inserted, nil
}

func (c *Committer) addImportedItemSessions(ctx context.Context, conn *sql.Conn, items []CommitItem) (int, error) {
	total := 0
	for _, item := range items {
		total += len(item.Sessions)
	}
	if total == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_play_sessions`); err != nil {
		return 0, fmt.Errorf("drop temp_import_play_sessions: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_play_sessions (
		id TEXT,
		game_id TEXT,
		start_time TIMESTAMPTZ,
		end_time TIMESTAMPTZ,
		duration INTEGER,
		updated_at TIMESTAMPTZ
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_play_sessions: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_import_play_sessions", func(appender *duckdb.Appender) error {
		for itemIndex := range items {
			for sessionIndex := range items[itemIndex].Sessions {
				session := items[itemIndex].Sessions[sessionIndex]
				if session.ID == "" {
					session.ID = uuid.New().String()
				}
				if session.GameID == "" {
					session.GameID = items[itemIndex].Game.ID
				}
				if session.UpdatedAt.IsZero() {
					session.UpdatedAt = now
				}
				items[itemIndex].Sessions[sessionIndex] = session
				if err := appender.AppendRow(session.ID, session.GameID, session.StartTime, session.EndTime, int64(session.Duration), session.UpdatedAt); err != nil {
					return fmt.Errorf("append imported session for %s: %w", items[itemIndex].Game.Name, err)
				}
				inserted++
			}
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_play_sessions_dedup`); err != nil {
		return inserted, fmt.Errorf("drop temp_import_play_sessions_dedup: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TEMP TABLE temp_import_play_sessions_dedup AS
		SELECT id, game_id, start_time, end_time, duration, updated_at
		FROM (
			SELECT
				id,
				game_id,
				start_time,
				end_time,
				duration,
				updated_at,
				ROW_NUMBER() OVER (
					PARTITION BY game_id, start_time, end_time
					ORDER BY updated_at DESC, id
				) AS row_num
			FROM temp_import_play_sessions
		)
		WHERE row_num = 1
	`); err != nil {
		return inserted, fmt.Errorf("deduplicate imported sessions: %w", err)
	}

	var toInsert int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM temp_import_play_sessions_dedup t
		WHERE NOT EXISTS (
			SELECT 1
			FROM play_sessions p
			WHERE p.game_id = t.game_id
			  AND p.start_time = t.start_time
			  AND p.end_time = t.end_time
		)
	`).Scan(&toInsert); err != nil {
		return inserted, fmt.Errorf("count deduplicated imported sessions: %w", err)
	}
	if toInsert == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		SELECT id, game_id, start_time, end_time, duration, updated_at
		FROM temp_import_play_sessions_dedup t
		WHERE NOT EXISTS (
			SELECT 1
			FROM play_sessions p
			WHERE p.game_id = t.game_id
			  AND p.start_time = t.start_time
			  AND p.end_time = t.end_time
		)
	`); err != nil {
		return inserted, fmt.Errorf("insert imported sessions from staging: %w", err)
	}
	return toInsert, nil
}

func (c *Committer) deleteImportedItemTombstones(ctx context.Context, conn *sql.Conn, items []CommitItem) error {
	if len(items) == 0 {
		return nil
	}
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_tombstones`); err != nil {
		return fmt.Errorf("drop temp_import_tombstones: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_tombstones (
		entity_type TEXT,
		entity_id TEXT
	)`); err != nil {
		return fmt.Errorf("create temp_import_tombstones: %w", err)
	}

	count := 0
	if err := appendImportRows(ctx, conn, "temp_import_tombstones", func(appender *duckdb.Appender) error {
		for _, item := range items {
			if item.Action != ImportActionMergeSessions {
				if item.Game.ID != "" {
					if err := appender.AppendRow(cloudsync.EntityGame, item.Game.ID); err != nil {
						return fmt.Errorf("append game tombstone %s: %w", item.Game.ID, err)
					}
					count++
				}
				for _, tag := range item.Tags {
					name := strings.TrimSpace(tag.Name)
					source := strings.TrimSpace(tag.Source)
					if name == "" {
						continue
					}
					if source == "" {
						source = "user"
					}
					if err := appender.AppendRow(cloudsync.EntityGameTag, cloudsync.TagTombstoneID(item.Game.ID, source, name)); err != nil {
						return fmt.Errorf("append tag tombstone %s/%s: %w", item.Game.ID, name, err)
					}
					count++
				}
				for _, source := range item.Game.MetadataSources {
					if source.SourceType == "" || strings.TrimSpace(source.SourceID) == "" {
						continue
					}
					if err := appender.AppendRow(cloudsync.EntityGameMetadataSource, cloudsync.MetadataSourceTombstoneID(item.Game.ID, string(source.SourceType))); err != nil {
						return fmt.Errorf("append metadata source tombstone %s/%s: %w", item.Game.ID, source.SourceType, err)
					}
					count++
				}
			}
			for _, session := range item.Sessions {
				if session.ID == "" {
					continue
				}
				if err := appender.AppendRow(cloudsync.EntityPlaySession, session.ID); err != nil {
					return fmt.Errorf("append session tombstone %s: %w", session.ID, err)
				}
				count++
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `
		DELETE FROM sync_tombstones
		WHERE EXISTS (
			SELECT 1
			FROM temp_import_tombstones t
			WHERE t.entity_type = sync_tombstones.entity_type
			  AND t.entity_id = sync_tombstones.entity_id
		)
	`); err != nil {
		return fmt.Errorf("delete import tombstones from staging: %w", err)
	}
	return nil
}

func (c *Committer) CommitItems(items []CommitItem) (int, int, error) {
	if len(items) == 0 {
		return 0, 0, nil
	}

	startedAt := time.Now()
	applog.LogInfof(c.ctx, "commitImportedItems: start items=%d", len(items))

	conn, err := c.db.Conn(c.ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取导入数据库连接失败: %w", err)
	}
	defer conn.Close()
	defer CleanupStagingTables(c.ctx, conn)

	if _, err := conn.ExecContext(c.ctx, `BEGIN TRANSACTION`); err != nil {
		return 0, 0, fmt.Errorf("开始导入事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(c.ctx, `ROLLBACK`)
		}
	}()

	createItems, updateItems := splitCommitItemsByAction(items)
	metadataItems := commitItemsWithMetadata(items)

	stepStartedAt := time.Now()
	insertedGames, err := c.addImportedItems(c.ctx, conn, createItems)
	if err != nil {
		return 0, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: staged and inserted games=%d elapsed=%s", insertedGames, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	updatedGames, err := c.updateImportedItemMetadata(c.ctx, conn, updateItems)
	if err != nil {
		return insertedGames, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: staged and updated games=%d elapsed=%s", updatedGames, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	updatedLaunchFields, err := c.updateImportedItemLocalLaunchFields(c.ctx, conn, updateItems)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: staged local launch field updates=%d elapsed=%s", updatedLaunchFields, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	upsertedSources, err := c.upsertImportedItemMetadataSources(c.ctx, conn, metadataItems)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: upserted metadata sources=%d elapsed=%s", upsertedSources, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	insertedTags, err := c.addImportedItemTags(c.ctx, conn, metadataItems)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: staged and upserted tags=%d elapsed=%s", insertedTags, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	insertedSessions, err := c.addImportedItemSessions(c.ctx, conn, items)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: staged and inserted sessions=%d elapsed=%s", insertedSessions, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	if err := c.deleteImportedItemTombstones(c.ctx, conn, items); err != nil {
		return insertedGames + updatedGames, insertedSessions, err
	}
	applog.LogInfof(c.ctx, "commitImportedItems: deleted sync tombstones elapsed=%s", time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	if _, err := conn.ExecContext(c.ctx, `COMMIT`); err != nil {
		return 0, 0, fmt.Errorf("提交导入事务失败: %w", err)
	}
	committed = true
	applog.LogInfof(c.ctx, "commitImportedItems: committed elapsed=%s total=%s", time.Since(stepStartedAt), time.Since(startedAt))
	if err := dbutils.CheckpointDuckDB(c.ctx, c.db); err != nil {
		applog.LogWarningf(c.ctx, "commitImportedItems: checkpoint failed; committed import remains in WAL: %v", err)
	} else {
		applog.LogInfof(c.ctx, "commitImportedItems: checkpoint completed total=%s", time.Since(startedAt))
	}

	c.startImportCoverProcessing(metadataItems)
	return insertedGames + updatedGames + len(items) - len(metadataItems), insertedSessions, nil
}

func CleanupStagingTables(ctx context.Context, conn *sql.Conn) {
	for _, tableName := range stagingTableNames() {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tableName)); err != nil {
			applog.LogWarningf(ctx, "cleanupImportStagingTables: failed to drop %s: %v", tableName, err)
		}
	}
}

func stagingTableNames() []string {
	return []string{
		"temp_import_games",
		"temp_update_import_games",
		"temp_update_import_launch_fields",
		"temp_import_game_tags",
		"temp_import_play_sessions",
		"temp_import_play_sessions_dedup",
		"temp_import_tombstones",
	}
}

func (c *Committer) startImportCoverProcessing(items []CommitItem) {
	if c.startCoverDownloads == nil && c.updateCoverURL == nil {
		return
	}

	imageItems := make([]CoverDownloadItem, 0)
	loaderJobs := make([]CommitItem, 0)
	for _, item := range items {
		if gamehelper.IsDownloadableCoverURL(item.Game.CoverURL) {
			imageItems = append(imageItems, CoverDownloadItem{
				GameID:   item.Game.ID,
				GameName: item.Game.Name,
				CoverURL: item.Game.CoverURL,
			})
		}
		if item.CoverLoader != nil {
			loaderJobs = append(loaderJobs, item)
		}
	}

	if len(imageItems) > 0 {
		if c.startCoverDownloads != nil {
			taskID := c.startCoverDownloads(imageItems)
			if strings.TrimSpace(taskID) == "" {
				applog.LogWarningf(c.ctx, "import cover processing: failed to create image download task")
			}
		} else {
			applog.LogWarningf(c.ctx, "import cover processing: image download task starter is not initialized")
		}
	}

	if len(loaderJobs) == 0 {
		return
	}

	workerCount := importCoverWorkerCount
	if len(loaderJobs) < workerCount {
		workerCount = len(loaderJobs)
	}

	go func() {
		startedAt := time.Now()
		applog.LogInfof(c.ctx, "import cover processing: start loader_jobs=%d workers=%d", len(loaderJobs), workerCount)

		jobCh := make(chan CommitItem)
		var wg sync.WaitGroup
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range jobCh {
					c.processImportCover(item)
				}
			}()
		}
		for _, item := range loaderJobs {
			jobCh <- item
		}
		close(jobCh)
		wg.Wait()

		applog.LogInfof(c.ctx, "import cover processing: complete loader_jobs=%d elapsed=%s", len(loaderJobs), time.Since(startedAt))
	}()
}

func (c *Committer) processImportCover(item CommitItem) {
	if item.CoverLoader == nil {
		return
	}

	coverURL, err := item.CoverLoader(item.Game)
	if err != nil {
		applog.LogWarningf(c.ctx, "import cover processing failed for %s: %v", item.Game.Name, err)
		return
	}
	if coverURL == "" {
		return
	}
	if c.updateCoverURL != nil {
		if err := c.updateCoverURL(item.Game.ID, coverURL); err != nil {
			applog.LogWarningf(c.ctx, "import cover update failed for %s: %v", item.Game.Name, err)
		}
	}
}
