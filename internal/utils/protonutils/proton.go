package protonutils

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const runnerPrefix = "proton"

var (
	protonDisplayNamePattern  = regexp.MustCompile(`(?i)"display_name"\s*"([^"]*)"`)
	protonInternalNamePattern = regexp.MustCompile(`(?i)"([^"]+)"\s*//\s*Internal name of this tool`)
	steamLibraryPathPattern   = regexp.MustCompile(`(?i)"path"\s*"([^"]+)"`)
)

type Tool struct {
	ID          string
	Name        string
	DisplayName string
	Path        string
	ProtonPath  string
	Source      string
	BuiltIn     bool
}

type toolRoot struct {
	path    string
	source  string
	builtIn bool
}

type discoverOptions struct {
	home string
}

func DiscoverTools() []Tool {
	home, _ := os.UserHomeDir()
	return discoverTools(discoverOptions{home: home})
}

func SelectTool(selector string) (Tool, error) {
	tools := DiscoverTools()
	if len(tools) == 0 {
		return Tool{}, fmt.Errorf("未找到本机 Proton 兼容工具，请先安装 Steam Proton、GE-Proton、DW-Proton 等兼容工具")
	}

	selector = strings.TrimSpace(selector)
	selector = strings.TrimPrefix(selector, runnerPrefix+":")
	if selector == "" || selector == runnerPrefix || selector == "auto" {
		return tools[0], nil
	}

	for _, tool := range tools {
		if tool.ID == selector || tool.Name == selector || tool.DisplayName == selector || tool.Path == selector || tool.ProtonPath == selector {
			return tool, nil
		}
	}
	return Tool{}, fmt.Errorf("未找到指定 Proton 兼容工具: %s", selector)
}

func IsProtonRunner(runner string) bool {
	runner = strings.ToLower(strings.TrimSpace(runner))
	return runner == runnerPrefix || strings.HasPrefix(runner, runnerPrefix+":")
}

func RunnerSelector(runner string) string {
	runner = strings.TrimSpace(runner)
	if strings.HasPrefix(strings.ToLower(runner), runnerPrefix+":") {
		return strings.TrimSpace(runner[len(runnerPrefix)+1:])
	}
	if strings.EqualFold(runner, runnerPrefix) {
		return ""
	}
	return runner
}

func DefaultCompatDataPath(gameID string) (string, error) {
	dataHome, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	gameID = stableCompatDataID(gameID)
	return filepath.Join(dataHome, "LunaBox", "proton-compatdata", gameID), nil
}

func NormalizeCompatDataPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if strings.EqualFold(filepath.Base(path), "pfx") {
		return filepath.Dir(path)
	}
	return path
}

func StableAppID(gameID string, sourceID string) string {
	if appID := positiveNumericID(sourceID); appID != "" {
		return appID
	}
	if appID := positiveNumericID(gameID); appID != "" {
		return appID
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(gameID)))
	value := hash.Sum32()
	if value < 100000 {
		value += 100000
	}
	return strconv.FormatUint(uint64(value), 10)
}

func discoverTools(options discoverOptions) []Tool {
	home := strings.TrimSpace(options.home)
	if home == "" {
		return nil
	}

	roots := protonToolRoots(home)
	seen := map[string]struct{}{}
	tools := make([]Tool, 0)
	for _, root := range roots {
		for _, tool := range scanToolRoot(root) {
			key := canonicalToolPath(tool.Path)
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			tools = append(tools, tool)
		}
	}
	assignToolIDs(tools)
	sortTools(tools)
	return tools
}

func protonToolRoots(home string) []toolRoot {
	steamRoots := []string{
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "debian-installation"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".steam", "root"),
	}

	roots := make([]toolRoot, 0)
	for _, steamRoot := range steamRoots {
		roots = append(roots,
			toolRoot{path: filepath.Join(steamRoot, "compatibilitytools.d"), source: "steam-compat"},
		)
		for _, library := range steamLibraries(steamRoot) {
			roots = append(roots, toolRoot{
				path:    filepath.Join(library, "steamapps", "common"),
				source:  "steam",
				builtIn: true,
			})
		}
	}

	roots = append(roots,
		toolRoot{path: filepath.Join(home, ".config", "heroic", "tools", "proton"), source: "heroic"},
		toolRoot{path: filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic", "tools", "proton"), source: "heroic"},
		toolRoot{path: filepath.Join(home, ".local", "share", "lutris", "runners", "proton"), source: "lutris"},
		toolRoot{path: filepath.Join(home, ".local", "share", "lutris", "runners", "wine"), source: "lutris"},
		toolRoot{path: filepath.Join(home, ".var", "app", "net.lutris.Lutris", "data", "lutris", "runners", "proton"), source: "lutris"},
		toolRoot{path: filepath.Join(home, ".var", "app", "net.lutris.Lutris", "data", "lutris", "runners", "wine"), source: "lutris"},
		toolRoot{path: filepath.Join(home, ".local", "share", "bottles", "runners"), source: "bottles"},
		toolRoot{path: filepath.Join(home, ".var", "app", "com.usebottles.bottles", "data", "bottles", "runners"), source: "bottles"},
	)
	return roots
}

func steamLibraries(steamRoot string) []string {
	steamRoot = strings.TrimSpace(steamRoot)
	if steamRoot == "" {
		return nil
	}

	libraries := []string{steamRoot}
	data, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libraries
	}
	for _, match := range steamLibraryPathPattern.FindAllSubmatch(data, -1) {
		if len(match) != 2 {
			continue
		}
		path := strings.TrimSpace(string(match[1]))
		path = strings.ReplaceAll(path, `\\`, `\`)
		if path == "" {
			continue
		}
		libraries = append(libraries, path)
	}
	return uniqueStrings(libraries)
}

func scanToolRoot(root toolRoot) []Tool {
	if _, ok := readProtonTool(root.path, root.source, root.builtIn); ok {
		tool, _ := readProtonTool(root.path, root.source, root.builtIn)
		return []Tool{tool}
	}

	entries, err := os.ReadDir(root.path)
	if err != nil {
		return nil
	}
	tools := make([]Tool, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if tool, ok := readProtonTool(filepath.Join(root.path, entry.Name()), root.source, root.builtIn); ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

func readProtonTool(path string, source string, builtIn bool) (Tool, bool) {
	protonPath := filepath.Join(path, "proton")
	info, err := os.Stat(protonPath)
	if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return Tool{}, false
	}

	name := filepath.Base(path)
	displayName := name
	if data, err := os.ReadFile(filepath.Join(path, "compatibilitytool.vdf")); err == nil {
		if value := regexpValue(protonInternalNamePattern, data); value != "" {
			name = value
		}
		if value := regexpValue(protonDisplayNamePattern, data); value != "" {
			displayName = value
		}
	}
	return Tool{
		Name:        strings.TrimSpace(name),
		DisplayName: strings.TrimSpace(displayName),
		Path:        path,
		ProtonPath:  protonPath,
		Source:      source,
		BuiltIn:     builtIn,
	}, true
}

func assignToolIDs(tools []Tool) {
	used := map[string]int{}
	for index := range tools {
		base := normalizeToolID(tools[index].Source + ":" + tools[index].Name)
		if base == "" {
			base = normalizeToolID(tools[index].Source + ":" + filepath.Base(tools[index].Path))
		}
		id := base
		if count := used[base]; count > 0 {
			id = fmt.Sprintf("%s-%d", base, count+1)
		}
		used[base]++
		tools[index].ID = id
	}
}

func sortTools(tools []Tool) {
	sort.SliceStable(tools, func(i, j int) bool {
		leftSource := sourcePriority(tools[i].Source)
		rightSource := sourcePriority(tools[j].Source)
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		leftName := toolNamePriority(tools[i])
		rightName := toolNamePriority(tools[j])
		if leftName != rightName {
			return leftName < rightName
		}
		return strings.ToLower(tools[i].DisplayName) < strings.ToLower(tools[j].DisplayName)
	})
}

func sourcePriority(source string) int {
	switch source {
	case "heroic", "lutris", "bottles", "steam-compat":
		return 0
	case "steam":
		return 1
	default:
		return 2
	}
}

func toolNamePriority(tool Tool) int {
	name := strings.ToLower(tool.Name + " " + tool.DisplayName + " " + filepath.Base(tool.Path))
	switch {
	case strings.Contains(name, "dw-proton"):
		return 0
	case strings.Contains(name, "ge-proton") || strings.Contains(name, "proton-ge"):
		return 1
	case strings.Contains(name, "experimental"):
		return 2
	case strings.Contains(name, "hotfix"):
		return 3
	default:
		return 4
	}
}

func regexpValue(pattern *regexp.Regexp, data []byte) string {
	match := pattern.FindSubmatch(data)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func canonicalToolPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if abs, err := filepath.Abs(filepath.Clean(path)); err == nil {
		path = abs
	}
	return path
}

func normalizeToolID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := canonicalToolPath(value)
		if key == "" {
			key = value
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func positiveNumericID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	number, err := strconv.ParseUint(value, 10, 32)
	if err != nil || number == 0 {
		return ""
	}
	return strconv.FormatUint(number, 10)
}

func stableCompatDataID(gameID string) string {
	gameID = normalizeToolID(gameID)
	if gameID == "" {
		return "default"
	}
	return gameID
}
