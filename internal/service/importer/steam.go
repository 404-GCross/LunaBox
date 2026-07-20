package importer

import (
	"encoding/binary"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/metadata"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const steamFullyInstalledFlag = 4

const (
	binaryVDFObject     = 0x00
	binaryVDFString     = 0x01
	binaryVDFInt32      = 0x02
	binaryVDFFloat32    = 0x03
	binaryVDFPointer    = 0x04
	binaryVDFWideString = 0x05
	binaryVDFColor      = 0x06
	binaryVDFUInt64     = 0x07
	binaryVDFEnd        = 0x08
	binaryVDFInt64      = 0x0A
)

type SteamImporter struct {
	deps Dependencies
}

type SteamImportOptions struct {
	IncludeNonSteam bool
}

type SteamLocalGame struct {
	AppID         string   `json:"app_id"`
	Name          string   `json:"name"`
	InstallDir    string   `json:"install_dir"`
	GameDir       string   `json:"game_dir"`
	LibraryPath   string   `json:"library_path"`
	ManifestPath  string   `json:"manifest_path"`
	SizeOnDisk    int64    `json:"size_on_disk"`
	StateFlags    int      `json:"state_flags"`
	Executables   []string `json:"executables"`
	SelectedExe   string   `json:"selected_exe"`
	IsShortcut    bool     `json:"is_shortcut"`
	SteamUserID   string   `json:"steam_user_id"`
	ShortcutID    string   `json:"shortcut_id"`
	SteamLaunchID string   `json:"steam_launch_id"`
}

type vdfNode struct {
	Value    string
	Children map[string]*vdfNode
}

func NewSteamImporter(deps Dependencies) *SteamImporter {
	return &SteamImporter{deps: deps}
}

func (s *SteamImporter) Preview() ([]PreviewGame, error) {
	return s.PreviewWithOptions(SteamImportOptions{})
}

func (s *SteamImporter) PreviewWithOptions(options SteamImportOptions) ([]PreviewGame, error) {
	games, err := s.ScanLocalGamesWithOptions(options)
	if err != nil {
		return nil, err
	}

	existingGames, _, _, err := s.deps.existingGames("PreviewSteamLocalImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(games))
	for _, game := range games {
		sourceType, sourceID := steamLocalGameSource(game)
		importPath := steamLocalGameImportPath(game)
		conflict := previewConflict(existingIndex, game.Name, importPath, sourceType, sourceID)
		developer := ""
		if game.IsShortcut {
			developer = "Steam 快捷方式"
		}
		previews = append(previews, PreviewGame{
			Name:         game.Name,
			Developer:    developer,
			SourceType:   sourceType,
			SourceID:     sourceID,
			Path:         importPath,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      time.Now(),
			HasPath:      importPath != "",
		})
	}
	return previews, nil
}

func (s *SteamImporter) Import(skipNoPath bool, samePathAction string, language string, getterOptions ...metadata.GetterOption) (ImportResult, error) {
	return s.ImportSelectedWithOptions(skipNoPath, samePathAction, nil, SteamImportOptions{}, language, getterOptions...)
}

func (s *SteamImporter) ImportSelected(skipNoPath bool, samePathAction string, selections []vo.ImportSelection, language string, getterOptions ...metadata.GetterOption) (ImportResult, error) {
	return s.ImportSelectedWithOptions(skipNoPath, samePathAction, selections, SteamImportOptions{}, language, getterOptions...)
}

func (s *SteamImporter) ImportSelectedWithOptions(skipNoPath bool, samePathAction string, selections []vo.ImportSelection, options SteamImportOptions, language string, getterOptions ...metadata.GetterOption) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

	games, err := s.ScanLocalGamesWithOptions(options)
	if err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := s.deps.existingGames("ImportFromSteamLocal")
	if err != nil {
		return result, err
	}

	existingIndex := newExistingPreviewIndex(existingGames)
	getter := metadata.NewSteamInfoGetterWithLanguage(language, getterOptions...)
	items := make([]ImportItem, 0, len(games))
	for _, localGame := range games {
		sourceType, sourceID := steamLocalGameSource(localGame)
		importPath := steamLocalGameImportPath(localGame)
		if !selectionFilter.includes(localGame.Name, importPath, sourceType, sourceID) {
			continue
		}
		if skipNoPath && strings.TrimSpace(importPath) == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, localGame.Name+" (无路径)")
			continue
		}

		conflict := previewConflict(existingIndex, localGame.Name, importPath, sourceType, sourceID)
		action := ImportActionCreate
		existingGameID := ""
		if conflict.Type != ConflictTypeNone {
			if conflict.Type != ConflictTypeSamePath || samePathAction != SamePathActionMerge {
				result.Skipped++
				switch conflict.Type {
				case ConflictTypeSource:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (元数据已存在: "+conflict.Game.Name+")")
				case ConflictTypeNameAndPath:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (已存在)")
				default:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			existingGameID = conflict.Game.ID
		}

		game, tags := s.fetchSteamGameMetadata(getter, localGame)
		if action == ImportActionUpdateExisting {
			game.ID = existingGameID
		}
		game.SourceType = enums.SourceType(sourceType)
		game.SourceID = sourceID

		source := vo.GameMetadataFromWebVO{
			Source: enums.SourceType(sourceType),
			Game:   game,
			Tags:   tags,
		}
		items = append(items, ImportItem{
			Source:         source,
			DisplayName:    localGame.Name,
			Path:           importPath,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, game.Name, importPath)
			existingIndex.byPath[normalizeImportPath(importPath)] = game
			existingIndex.bySource[previewSourceKey(sourceType, sourceID)] = game
			existingIndex.byNamePath[previewNamePathKey(game.Name, importPath)] = game
		}
	}

	batchResult, err := addImportedItems(s.deps, items)
	if err != nil {
		return result, err
	}
	result.Success += batchResult.Success
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.SessionsImported += batchResult.SessionsImported
	result.SkippedNames = append(result.SkippedNames, batchResult.SkippedNames...)
	result.FailedNames = append(result.FailedNames, batchResult.FailedNames...)
	return result, nil
}

func (s *SteamImporter) ScanLocalGames() ([]SteamLocalGame, error) {
	return s.ScanLocalGamesWithOptions(SteamImportOptions{})
}

func (s *SteamImporter) ScanLocalGamesWithOptions(options SteamImportOptions) ([]SteamLocalGame, error) {
	steamPath, err := findSteamInstallPath()
	if err != nil {
		return nil, err
	}

	libraryPaths, err := loadSteamLibraryPaths(steamPath)
	if err != nil {
		return nil, err
	}

	games := make([]SteamLocalGame, 0)
	seenAppIDs := make(map[string]bool)
	for _, libraryPath := range libraryPaths {
		manifestPaths, err := filepath.Glob(filepath.Join(libraryPath, "steamapps", "appmanifest_*.acf"))
		if err != nil {
			applog.LogWarningf(s.deps.Ctx, "Steam scan: failed to glob manifests in %s: %v", libraryPath, err)
			continue
		}
		sort.Strings(manifestPaths)
		for _, manifestPath := range manifestPaths {
			game, err := readSteamManifest(libraryPath, manifestPath)
			if err != nil {
				applog.LogWarningf(s.deps.Ctx, "Steam scan: failed to read manifest %s: %v", manifestPath, err)
				continue
			}
			if game.AppID == "" || seenAppIDs[game.AppID] || !isImportableSteamGame(game) {
				continue
			}
			seenAppIDs[game.AppID] = true
			games = append(games, game)
		}
	}

	if options.IncludeNonSteam {
		shortcuts, err := scanSteamShortcutGames(steamPath)
		if err != nil {
			applog.LogWarningf(s.deps.Ctx, "Steam scan: failed to scan non-Steam shortcuts: %v", err)
		}
		seenShortcuts := make(map[string]bool, len(shortcuts))
		for _, game := range shortcuts {
			sourceType, sourceID := steamLocalGameSource(game)
			uniqueKey := previewSourceKey(sourceType, sourceID)
			if uniqueKey == "" {
				uniqueKey = normalizeImportPath(game.SelectedExe)
			}
			if uniqueKey == "" || seenShortcuts[uniqueKey] || !isImportableSteamShortcut(game) {
				continue
			}
			seenShortcuts[uniqueKey] = true
			games = append(games, game)
		}
	}

	sort.Slice(games, func(i, j int) bool {
		return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name)
	})
	return games, nil
}

func (s *SteamImporter) fetchSteamGameMetadata(getter *metadata.SteamInfoGetter, localGame SteamLocalGame) (models.Game, []metadata.TagItem) {
	gameID := uuid.New().String()
	now := time.Now()
	sourceType, sourceID := steamLocalGameSource(localGame)
	game := models.Game{
		ID:            gameID,
		Name:          localGame.Name,
		Path:          steamLocalGameImportPath(localGame),
		GameDirectory: defaultSteamLocalGameDirectory(localGame),
		LaunchMode:    enums.LaunchModeSteam,
		SourceType:    enums.SourceType(sourceType),
		SourceID:      sourceID,
		CreatedAt:     now,
		CachedAt:      now,
		UpdatedAt:     now,
	}

	var metaResult metadata.MetadataResult
	var err error
	if localGame.IsShortcut {
		metaResult, err = getter.FetchMetadataByName(localGame.Name, "")
	} else {
		metaResult, err = getter.FetchMetadata(localGame.AppID, "")
	}
	if err != nil {
		applog.LogWarningf(s.deps.Ctx, "ImportFromSteamLocal: failed to fetch Steam metadata for %s/%s: %v", localGame.AppID, localGame.Name, err)
		return game, nil
	}

	if metaResult.Game.Name != "" {
		game = metaResult.Game
		game.ID = gameID
		game.Path = steamLocalGameImportPath(localGame)
		game.GameDirectory = defaultSteamLocalGameDirectory(localGame)
		game.LaunchMode = enums.LaunchModeSteam
		game.SourceType = enums.SourceType(sourceType)
		game.SourceID = sourceID
		game.CreatedAt = now
		game.CachedAt = now
		game.UpdatedAt = now
	}
	return game, metaResult.Tags
}

func loadSteamLibraryPaths(steamPath string) ([]string, error) {
	libraryPaths := []string{steamPath}
	libraryFile := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")
	root, err := parseVDFFile(libraryFile)
	if err != nil {
		return uniqueExistingSteamLibraries(libraryPaths), nil
	}

	libraryFolders := child(root, "libraryfolders")
	if libraryFolders == nil {
		libraryFolders = root
	}
	for _, node := range libraryFolders.Children {
		if pathValue := strings.TrimSpace(node.Value); pathValue != "" {
			libraryPaths = append(libraryPaths, normalizeSteamPath(pathValue))
			continue
		}
		if pathNode := child(node, "path"); pathNode != nil && strings.TrimSpace(pathNode.Value) != "" {
			libraryPaths = append(libraryPaths, normalizeSteamPath(pathNode.Value))
		}
	}
	return uniqueExistingSteamLibraries(libraryPaths), nil
}

func uniqueExistingSteamLibraries(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		cleaned, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
		if err != nil || cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if seen[key] {
			continue
		}
		if info, err := os.Stat(filepath.Join(cleaned, "steamapps")); err == nil && info.IsDir() {
			seen[key] = true
			result = append(result, cleaned)
		}
	}
	return result
}

func readSteamManifest(libraryPath string, manifestPath string) (SteamLocalGame, error) {
	root, err := parseVDFFile(manifestPath)
	if err != nil {
		return SteamLocalGame{}, err
	}
	appState := child(root, "AppState")
	if appState == nil {
		appState = root
	}

	appID := nodeValue(appState, "appid")
	name := strings.TrimSpace(nodeValue(appState, "name"))
	installDirName := strings.TrimSpace(nodeValue(appState, "installdir"))
	stateFlags := parseInt(nodeValue(appState, "StateFlags"))
	sizeOnDisk := parseInt64(nodeValue(appState, "SizeOnDisk"))
	installDir := filepath.Join(libraryPath, "steamapps", "common", installDirName)

	executables := apputils.FindExecutables(installDir, steamExecutableExcludeKeywords())
	selectedExe := installDir
	if len(executables) > 0 {
		selectedExe = apputils.SelectBestExecutable(executables, name)
	}

	return SteamLocalGame{
		AppID:        strings.TrimSpace(appID),
		Name:         name,
		InstallDir:   installDir,
		LibraryPath:  libraryPath,
		ManifestPath: manifestPath,
		SizeOnDisk:   sizeOnDisk,
		StateFlags:   stateFlags,
		Executables:  executables,
		SelectedExe:  selectedExe,
	}, nil
}

func scanSteamShortcutGames(steamPath string) ([]SteamLocalGame, error) {
	userDataPath := filepath.Join(steamPath, "userdata")
	userEntries, err := os.ReadDir(userDataPath)
	if err != nil {
		return nil, fmt.Errorf("read Steam userdata dir: %w", err)
	}

	games := make([]SteamLocalGame, 0)
	for _, entry := range userEntries {
		if !entry.IsDir() {
			continue
		}
		userID := entry.Name()
		shortcutsPath := filepath.Join(userDataPath, userID, "config", "shortcuts.vdf")
		if info, err := os.Stat(shortcutsPath); err != nil || info.IsDir() {
			continue
		}

		shortcuts, err := parseSteamShortcutsVDF(shortcutsPath)
		if err != nil {
			continue
		}
		for shortcutID, values := range shortcuts {
			name := strings.TrimSpace(values["appname"])
			exePath := normalizeSteamShortcutFilePath(values["exe"])
			startDir := normalizeSteamShortcutFilePath(values["startdir"])
			if startDir == "" && exePath != "" {
				startDir = filepath.Dir(exePath)
			}
			appID := strings.TrimSpace(values["appid"])
			launchID := steamShortcutRunGameID(appID)

			games = append(games, SteamLocalGame{
				AppID:         appID,
				Name:          name,
				InstallDir:    startDir,
				GameDir:       startDir,
				ManifestPath:  shortcutsPath,
				SelectedExe:   exePath,
				IsShortcut:    true,
				SteamUserID:   userID,
				ShortcutID:    shortcutID,
				SteamLaunchID: launchID,
			})
		}
	}
	return games, nil
}

func parseSteamShortcutsVDF(path string) (map[string]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Steam shortcuts VDF: %w", err)
	}
	reader := binaryVDFReader{data: data}
	if len(data) == 0 {
		return map[string]map[string]string{}, nil
	}

	rootType, err := reader.readByte()
	if err != nil {
		return nil, err
	}
	rootName, err := reader.readNullTerminatedString()
	if err != nil {
		return nil, err
	}
	if rootType != binaryVDFObject || !strings.EqualFold(rootName, "shortcuts") {
		return nil, fmt.Errorf("invalid Steam shortcuts VDF root: %s", rootName)
	}

	shortcuts := make(map[string]map[string]string)
	for reader.remaining() > 0 {
		valueType, err := reader.readByte()
		if err != nil {
			return nil, err
		}
		if valueType == binaryVDFEnd {
			break
		}

		key, err := reader.readNullTerminatedString()
		if err != nil {
			return nil, err
		}
		if valueType != binaryVDFObject {
			if err := reader.skipValue(valueType); err != nil {
				return nil, err
			}
			continue
		}

		values, err := reader.readObjectValues()
		if err != nil {
			return nil, err
		}
		shortcuts[key] = values
	}
	return shortcuts, nil
}

type binaryVDFReader struct {
	data []byte
	pos  int
}

func (r *binaryVDFReader) remaining() int {
	return len(r.data) - r.pos
}

func (r *binaryVDFReader) readByte() (byte, error) {
	if r.remaining() < 1 {
		return 0, fmt.Errorf("unexpected end of binary VDF")
	}
	value := r.data[r.pos]
	r.pos++
	return value, nil
}

func (r *binaryVDFReader) readBytes(count int) ([]byte, error) {
	if count < 0 || r.remaining() < count {
		return nil, fmt.Errorf("unexpected end of binary VDF")
	}
	value := r.data[r.pos : r.pos+count]
	r.pos += count
	return value, nil
}

func (r *binaryVDFReader) readNullTerminatedString() (string, error) {
	start := r.pos
	for r.pos < len(r.data) {
		if r.data[r.pos] == 0 {
			value := string(r.data[start:r.pos])
			r.pos++
			return value, nil
		}
		r.pos++
	}
	return "", fmt.Errorf("unterminated binary VDF string")
}

func (r *binaryVDFReader) skipWideString() error {
	for r.remaining() >= 2 {
		value, err := r.readBytes(2)
		if err != nil {
			return err
		}
		if binary.LittleEndian.Uint16(value) == 0 {
			return nil
		}
	}
	return fmt.Errorf("unterminated binary VDF wide string")
}

func (r *binaryVDFReader) readObjectValues() (map[string]string, error) {
	values := make(map[string]string)
	for r.remaining() > 0 {
		valueType, err := r.readByte()
		if err != nil {
			return nil, err
		}
		if valueType == binaryVDFEnd {
			break
		}

		name, err := r.readNullTerminatedString()
		if err != nil {
			return nil, err
		}
		name = strings.ToLower(strings.TrimSpace(name))

		switch valueType {
		case binaryVDFObject:
			if err := r.skipObject(); err != nil {
				return nil, err
			}
		case binaryVDFString:
			value, err := r.readNullTerminatedString()
			if err != nil {
				return nil, err
			}
			values[name] = value
		case binaryVDFInt32:
			raw, err := r.readBytes(4)
			if err != nil {
				return nil, err
			}
			values[name] = strconv.FormatUint(uint64(binary.LittleEndian.Uint32(raw)), 10)
		case binaryVDFUInt64:
			raw, err := r.readBytes(8)
			if err != nil {
				return nil, err
			}
			values[name] = strconv.FormatUint(binary.LittleEndian.Uint64(raw), 10)
		case binaryVDFInt64:
			raw, err := r.readBytes(8)
			if err != nil {
				return nil, err
			}
			values[name] = strconv.FormatInt(int64(binary.LittleEndian.Uint64(raw)), 10)
		default:
			if err := r.skipValue(valueType); err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}

func (r *binaryVDFReader) skipObject() error {
	for r.remaining() > 0 {
		valueType, err := r.readByte()
		if err != nil {
			return err
		}
		if valueType == binaryVDFEnd {
			return nil
		}
		if _, err := r.readNullTerminatedString(); err != nil {
			return err
		}
		if err := r.skipValue(valueType); err != nil {
			return err
		}
	}
	return fmt.Errorf("unterminated binary VDF object")
}

func (r *binaryVDFReader) skipValue(valueType byte) error {
	switch valueType {
	case binaryVDFObject:
		return r.skipObject()
	case binaryVDFString:
		_, err := r.readNullTerminatedString()
		return err
	case binaryVDFInt32, binaryVDFFloat32, binaryVDFPointer, binaryVDFColor:
		_, err := r.readBytes(4)
		return err
	case binaryVDFWideString:
		return r.skipWideString()
	case binaryVDFUInt64, binaryVDFInt64:
		_, err := r.readBytes(8)
		return err
	default:
		return fmt.Errorf("unsupported binary VDF value type: 0x%02x", valueType)
	}
}

func isImportableSteamGame(game SteamLocalGame) bool {
	if game.StateFlags&steamFullyInstalledFlag == 0 {
		return false
	}
	if strings.TrimSpace(game.Name) == "" || strings.TrimSpace(game.InstallDir) == "" {
		return false
	}
	if info, err := os.Stat(game.InstallDir); err != nil || !info.IsDir() {
		return false
	}

	name := strings.ToLower(strings.TrimSpace(game.Name))
	if game.AppID == "228980" ||
		strings.Contains(name, "steamworks common redistributables") ||
		strings.Contains(name, "redistributable") ||
		strings.Contains(name, "dedicated server") {
		return false
	}
	return true
}

func isImportableSteamShortcut(game SteamLocalGame) bool {
	if strings.TrimSpace(game.Name) == "" || strings.TrimSpace(game.SelectedExe) == "" || strings.TrimSpace(game.SteamLaunchID) == "" {
		return false
	}
	if info, err := os.Stat(game.SelectedExe); err != nil || info.IsDir() {
		return false
	}
	return true
}

func steamLocalGameImportPath(game SteamLocalGame) string {
	if game.IsShortcut {
		return strings.TrimSpace(game.SelectedExe)
	}
	return strings.TrimSpace(game.InstallDir)
}

func defaultSteamLocalGameDirectory(game SteamLocalGame) string {
	if strings.TrimSpace(game.GameDir) != "" {
		return strings.TrimSpace(game.GameDir)
	}
	if strings.TrimSpace(game.InstallDir) != "" {
		return strings.TrimSpace(game.InstallDir)
	}
	if strings.TrimSpace(game.SelectedExe) != "" {
		return filepath.Dir(game.SelectedExe)
	}
	return ""
}

func steamLocalGameSource(game SteamLocalGame) (string, string) {
	if game.IsShortcut {
		return string(enums.SteamShortcut), strings.TrimSpace(game.SteamLaunchID)
	}
	return string(enums.Steam), strings.TrimSpace(game.AppID)
}

func steamShortcutRunGameID(appID string) string {
	value, err := strconv.ParseUint(strings.TrimSpace(appID), 10, 32)
	if err != nil || value == 0 {
		return ""
	}
	return strconv.FormatUint((value<<32)|0x02000000, 10)
}

func normalizeSteamShortcutFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "\"") {
		rest := strings.TrimPrefix(path, "\"")
		if end := strings.Index(rest, "\""); end >= 0 {
			path = rest[:end]
		}
	} else if idx := strings.Index(strings.ToLower(path), ".exe"); idx >= 0 {
		path = path[:idx+len(".exe")]
	}
	path = strings.Trim(strings.TrimSpace(path), "\"")
	path = strings.ReplaceAll(path, "/", string(os.PathSeparator))
	if abs, err := filepath.Abs(filepath.Clean(path)); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func steamExecutableExcludeKeywords() []string {
	return []string{
		"unins", "setup", "config", "patch", "update", "crashpad",
		"vc_redist", "dxwebsetup", "directx", "vcredist", "dotnet",
		"redistributable", "installer", "launcher_helper", "crashreporter",
		"updater", "uninstall", "删除", "卸载",
	}
}

func parseVDFFile(path string) (*vdfNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read VDF file: %w", err)
	}
	tokens, err := tokenizeVDF(string(data))
	if err != nil {
		return nil, err
	}
	root := &vdfNode{Children: make(map[string]*vdfNode)}
	if err := parseVDFObject(root, tokens, 0); err != nil {
		return nil, err
	}
	return root, nil
}

func parseVDFObject(target *vdfNode, tokens []string, start int) error {
	for i := start; i < len(tokens); {
		token := tokens[i]
		if token == "}" {
			return nil
		}
		if token == "{" {
			return fmt.Errorf("unexpected VDF object start")
		}
		key := token
		i++
		if i >= len(tokens) {
			return fmt.Errorf("missing VDF value for key %s", key)
		}
		if tokens[i] == "{" {
			childNode := &vdfNode{Children: make(map[string]*vdfNode)}
			next, err := parseVDFObjectWithIndex(childNode, tokens, i+1)
			if err != nil {
				return err
			}
			target.Children[key] = childNode
			i = next
			continue
		}
		if tokens[i] == "}" {
			return fmt.Errorf("missing VDF value for key %s", key)
		}
		target.Children[key] = &vdfNode{Value: tokens[i]}
		i++
	}
	return nil
}

func parseVDFObjectWithIndex(target *vdfNode, tokens []string, start int) (int, error) {
	for i := start; i < len(tokens); {
		token := tokens[i]
		if token == "}" {
			return i + 1, nil
		}
		if token == "{" {
			return i, fmt.Errorf("unexpected VDF object start")
		}
		key := token
		i++
		if i >= len(tokens) {
			return i, fmt.Errorf("missing VDF value for key %s", key)
		}
		if tokens[i] == "{" {
			childNode := &vdfNode{Children: make(map[string]*vdfNode)}
			next, err := parseVDFObjectWithIndex(childNode, tokens, i+1)
			if err != nil {
				return next, err
			}
			target.Children[key] = childNode
			i = next
			continue
		}
		if tokens[i] == "}" {
			return i, fmt.Errorf("missing VDF value for key %s", key)
		}
		target.Children[key] = &vdfNode{Value: tokens[i]}
		i++
	}
	return len(tokens), nil
}

func tokenizeVDF(input string) ([]string, error) {
	tokens := make([]string, 0)
	for i := 0; i < len(input); {
		ch := input[i]
		if ch == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '{' || ch == '}' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		if isVDFWhitespace(ch) {
			i++
			continue
		}
		if ch == '"' {
			value, next, err := readVDFQuoted(input, i+1)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, value)
			i = next
			continue
		}
		start := i
		for i < len(input) && !isVDFWhitespace(input[i]) && input[i] != '{' && input[i] != '}' {
			i++
		}
		tokens = append(tokens, input[start:i])
	}
	return tokens, nil
}

func readVDFQuoted(input string, start int) (string, int, error) {
	var builder strings.Builder
	for i := start; i < len(input); i++ {
		ch := input[i]
		if ch == '"' {
			return builder.String(), i + 1, nil
		}
		if ch == '\\' && i+1 < len(input) {
			next := input[i+1]
			if next == '"' || next == '\\' {
				builder.WriteByte(next)
				i++
				continue
			}
		}
		builder.WriteByte(ch)
	}
	return "", len(input), fmt.Errorf("unterminated VDF quoted string")
}

func isVDFWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

func child(node *vdfNode, key string) *vdfNode {
	if node == nil {
		return nil
	}
	if item, ok := node.Children[key]; ok {
		return item
	}
	lowerKey := strings.ToLower(key)
	for name, item := range node.Children {
		if strings.ToLower(name) == lowerKey {
			return item
		}
	}
	return nil
}

func nodeValue(node *vdfNode, key string) string {
	if item := child(node, key); item != nil {
		return strings.TrimSpace(item.Value)
	}
	return ""
}

func parseInt(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	return value
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func normalizeSteamPath(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "/", string(os.PathSeparator))
}
