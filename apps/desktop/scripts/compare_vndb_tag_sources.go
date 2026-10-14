//go:build ignore

// compare_vndb_tag_sources compares the tag rows generated from a VNDB dump
// with the rows produced by internal/utils/metadata/metadata_vndb.go.
//
// It does not write to the LunaBox database or CSV export.
//
// Examples:
//
//	go run scripts/compare_vndb_tag_sources.go --ids v572 --dump build/bin/vndb-db-2026-07-02.tar.zst --tag-limit -1
//	go run scripts/compare_vndb_tag_sources.go --target build/bin/lunabox_2026-06-25T22-10-40/database --dump build/bin/vndb-db-2026-07-02.tar.zst --tag-limit -1 --max 20
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"lunabox/internal/utils/metadata"

	_ "github.com/duckdb/duckdb-go/v2"
)

const (
	vndbTagSource   = "vndb"
	defaultTagLimit = -1
	defaultMaxIDs   = 100
	weightEpsilon   = 0.000001
)

var vndbIDPattern = regexp.MustCompile(`^v[0-9]+$`)

type targetKind int

const (
	targetKindDuckDB targetKind = iota + 1
	targetKindCSVExport
)

type targetInfo struct {
	kind      targetKind
	inputPath string
	dbPath    string
	gamesCSV  string
}

type gameRef struct {
	GameID   string
	Name     string
	SourceID string
}

type tagInfo struct {
	Name         string
	DefaultSpoil int
}

type tagAccumulator struct {
	VNID       string
	TagID      string
	Order      int
	Positive   int
	VoteSum    int
	VoteCount  int
	SignSum    int
	SpoilerSum int
	SpoilerCnt int
	LieTrue    int
	LieFalse   int
}

type computedTag struct {
	Name      string
	Rating    float64
	Weight    float64
	IsSpoiler bool
	Order     int
}

type compareResult struct {
	SourceID    string
	GameNames   []string
	Matched     bool
	OrderOnly   bool
	Differences []string
	DumpTags    []computedTag
	RemoteTags  []computedTag
}

type archiveFormat int

const (
	archiveTarZst archiveFormat = iota + 1
	archiveTarGz
	archiveTarPlain
)

func main() {
	targetPath := flag.String("target", "", "optional LunaBox DuckDB file or CSV export database directory")
	dumpPath := flag.String("dump", "", "VNDB near-complete dump, for example vndb-db-2026-07-02.tar.zst")
	idsRaw := flag.String("ids", "", "comma-separated VNDB IDs to compare; bypasses --target discovery")
	tagLimit := flag.Int("tag-limit", defaultTagLimit, "maximum VNDB tags per game; -1 keeps all, 0 keeps none")
	language := flag.String("language", "", "language preference passed to NewVNDBInfoGetterWithLanguage")
	maxIDs := flag.Int("max", defaultMaxIDs, "maximum unique target VNDB IDs to compare when --ids is omitted; -1 compares all")
	show := flag.Int("show", 10, "maximum mismatched VN entries to print")
	debugTagsRaw := flag.String("debug-tags", "", "optional comma-separated dump tag IDs to print before comparison")
	strictOrder := flag.Bool("strict-order", false, "treat insertion order differences as mismatches")
	flag.Parse()

	if *dumpPath == "" {
		exitErr(errors.New("--dump is required"))
	}
	if *idsRaw == "" && *targetPath == "" {
		exitErr(errors.New("either --ids or --target is required"))
	}
	if *tagLimit < -1 {
		exitErr(errors.New("--tag-limit must be -1, 0, or a positive integer"))
	}
	if *maxIDs == 0 || *maxIDs < -1 {
		exitErr(errors.New("--max must be -1 or a positive integer"))
	}

	gamesBySource, sourceIDs, err := collectSourceIDs(*idsRaw, *targetPath, *maxIDs)
	if err != nil {
		exitErr(err)
	}
	if len(sourceIDs) == 0 {
		fmt.Println("No VNDB source IDs to compare.")
		return
	}

	sourceSet := make(map[string]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		sourceSet[id] = struct{}{}
	}

	fmt.Printf("Comparing %d VNDB IDs\n", len(sourceIDs))
	fmt.Printf("Dump: %s\n", *dumpPath)
	fmt.Printf("Tag limit: %d\n", *tagLimit)

	dumpTags, err := buildVNDBTagsFromDump(*dumpPath, sourceSet, *tagLimit)
	if err != nil {
		exitErr(err)
	}
	if strings.TrimSpace(*debugTagsRaw) != "" {
		if err := printDebugDumpTags(*dumpPath, sourceSet, *debugTagsRaw); err != nil {
			exitErr(err)
		}
	}
	remoteTags, err := fetchRemoteVNDBTags(sourceIDs, *language, *tagLimit)
	if err != nil {
		exitErr(err)
	}

	results := make([]compareResult, 0, len(sourceIDs))
	matched := 0
	orderOnly := 0
	for _, sourceID := range sourceIDs {
		result := compareTags(sourceID, gamesBySource[sourceID], dumpTags[sourceID], remoteTags[sourceID], *strictOrder)
		if result.Matched {
			matched++
		}
		if result.OrderOnly {
			orderOnly++
		}
		results = append(results, result)
	}

	mismatched := len(results) - matched
	fmt.Println()
	fmt.Println("Summary")
	fmt.Printf("  Compared VNDB IDs: %d\n", len(results))
	fmt.Printf("  Matched:           %d\n", matched)
	fmt.Printf("  Mismatched:        %d\n", mismatched)
	fmt.Printf("  Order-only diffs:  %d\n", orderOnly)

	if mismatched == 0 {
		return
	}

	printed := 0
	for _, result := range results {
		if result.Matched {
			continue
		}
		if printed >= *show {
			break
		}
		printCompareResult(result)
		printed++
	}
	if mismatched > printed {
		fmt.Printf("\n... %d more mismatched VN entries omitted; increase --show to print more.\n", mismatched-printed)
	}
}

func collectSourceIDs(idsRaw string, targetPath string, maxIDs int) (map[string][]string, []string, error) {
	gamesBySource := make(map[string][]string)
	if strings.TrimSpace(idsRaw) != "" {
		sourceIDs := make([]string, 0)
		seen := make(map[string]struct{})
		for _, part := range strings.Split(idsRaw, ",") {
			id := normalizeVNID(part)
			if id == "" {
				return nil, nil, fmt.Errorf("invalid VNDB ID in --ids: %q", part)
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			sourceIDs = append(sourceIDs, id)
		}
		return gamesBySource, sourceIDs, nil
	}

	target, err := resolveTarget(targetPath)
	if err != nil {
		return nil, nil, err
	}
	games, err := loadGames(context.Background(), target)
	if err != nil {
		return nil, nil, err
	}
	sourceIDs := make([]string, 0)
	seen := make(map[string]struct{})
	for _, game := range games {
		gamesBySource[game.SourceID] = append(gamesBySource[game.SourceID], game.Name)
		if _, exists := seen[game.SourceID]; exists {
			continue
		}
		seen[game.SourceID] = struct{}{}
		sourceIDs = append(sourceIDs, game.SourceID)
	}
	sort.Strings(sourceIDs)
	if maxIDs > 0 && len(sourceIDs) > maxIDs {
		sourceIDs = sourceIDs[:maxIDs]
	}
	return gamesBySource, sourceIDs, nil
}

func fetchRemoteVNDBTags(sourceIDs []string, language string, tagLimit int) (map[string][]computedTag, error) {
	getter := metadata.NewVNDBInfoGetterWithLanguage(language, metadata.WithTagLimit(tagLimit))
	results, err := getter.FetchMetadataBatch(sourceIDs, "")
	if err != nil {
		return nil, err
	}

	tagsByVN := make(map[string][]computedTag, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		result, ok := results[sourceID]
		if !ok {
			tagsByVN[sourceID] = nil
			continue
		}
		tags := make([]computedTag, 0, len(result.Tags))
		for index, tag := range result.Tags {
			if strings.TrimSpace(tag.Name) == "" {
				continue
			}
			tags = append(tags, computedTag{
				Name:      tag.Name,
				Rating:    tag.Weight * 3.0,
				Weight:    tag.Weight,
				IsSpoiler: tag.IsSpoiler,
				Order:     index,
			})
		}
		tagsByVN[sourceID] = tags
	}
	return tagsByVN, nil
}

func compareTags(sourceID string, gameNames []string, dumpTags []computedTag, remoteTags []computedTag, strictOrder bool) compareResult {
	result := compareResult{
		SourceID:   sourceID,
		GameNames:  gameNames,
		DumpTags:   dumpTags,
		RemoteTags: remoteTags,
	}

	dumpByName := tagsByName(dumpTags)
	remoteByName := tagsByName(remoteTags)
	names := make([]string, 0, len(dumpByName)+len(remoteByName))
	seen := make(map[string]struct{})
	for _, tag := range dumpTags {
		key := tagKey(tag.Name)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			names = append(names, key)
		}
	}
	for _, tag := range remoteTags {
		key := tagKey(tag.Name)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			names = append(names, key)
		}
	}
	sort.Strings(names)

	for _, key := range names {
		dumpTag, inDump := dumpByName[key]
		remoteTag, inRemote := remoteByName[key]
		switch {
		case !inDump:
			result.Differences = append(result.Differences, fmt.Sprintf("missing in dump: %s", remoteTag.Name))
		case !inRemote:
			result.Differences = append(result.Differences, fmt.Sprintf("extra in dump: %s", dumpTag.Name))
		default:
			if math.Abs(dumpTag.Weight-remoteTag.Weight) > weightEpsilon || dumpTag.IsSpoiler != remoteTag.IsSpoiler {
				result.Differences = append(result.Differences, fmt.Sprintf(
					"field diff: %s dump(weight=%.6f spoiler=%t rating=%.6f) remote(weight=%.6f spoiler=%t rating=%.6f)",
					dumpTag.Name,
					dumpTag.Weight,
					dumpTag.IsSpoiler,
					dumpTag.Rating,
					remoteTag.Weight,
					remoteTag.IsSpoiler,
					remoteTag.Rating,
				))
			}
		}
	}

	if len(result.Differences) == 0 && !sameTagOrder(dumpTags, remoteTags) {
		result.OrderOnly = true
		if strictOrder {
			result.Differences = append(result.Differences, "order diff: same tag fields, different insertion order")
		}
	}
	result.Matched = len(result.Differences) == 0
	return result
}

func printCompareResult(result compareResult) {
	fmt.Println()
	title := result.SourceID
	if len(result.GameNames) > 0 {
		title += " " + strings.Join(result.GameNames, " / ")
	}
	fmt.Println(title)
	for _, diff := range result.Differences {
		fmt.Printf("  - %s\n", diff)
	}
	fmt.Printf("  dump tags (%d):   %s\n", len(result.DumpTags), joinTagNames(result.DumpTags))
	fmt.Printf("  remote tags (%d): %s\n", len(result.RemoteTags), joinTagNames(result.RemoteTags))
}

func printDebugDumpTags(dumpPath string, sourceIDs map[string]struct{}, tagsRaw string) error {
	wantedTags := make(map[string]struct{})
	for _, part := range strings.Split(tagsRaw, ",") {
		tagID := strings.TrimSpace(part)
		if tagID != "" {
			wantedTags[tagID] = struct{}{}
		}
	}
	if len(wantedTags) == 0 {
		return nil
	}

	tagInfos, err := readTagInfo(dumpPath)
	if err != nil {
		return err
	}
	disabledUsers, err := readDisabledTagUsers(dumpPath)
	if err != nil {
		return err
	}
	accs, err := readTagVotes(dumpPath, sourceIDs, disabledUsers)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Debug dump tag accumulators")
	for _, acc := range accs {
		if _, wanted := wantedTags[acc.TagID]; !wanted {
			continue
		}
		info := tagInfos[acc.TagID]
		positiveAverage := 3.0
		if acc.Positive > 0 {
			positiveAverage = float64(acc.VoteSum) / float64(acc.Positive)
		}
		rating := 0.0
		if acc.VoteCount > 0 {
			rating = positiveAverage * float64(acc.SignSum) / float64(acc.VoteCount)
		}
		fmt.Printf("  %s/%s %s: rating=%.6f positive=%d vote_count=%d sign_sum=%d lie_true=%d lie_false=%d default_spoil=%d\n",
			acc.VNID,
			acc.TagID,
			info.Name,
			rating,
			acc.Positive,
			acc.VoteCount,
			acc.SignSum,
			acc.LieTrue,
			acc.LieFalse,
			info.DefaultSpoil,
		)
	}
	return nil
}

func buildVNDBTagsFromDump(dumpPath string, sourceIDs map[string]struct{}, tagLimit int) (map[string][]computedTag, error) {
	tags, err := readTagInfo(dumpPath)
	if err != nil {
		return nil, err
	}
	disabledTagUsers, err := readDisabledTagUsers(dumpPath)
	if err != nil {
		return nil, err
	}
	accs, err := readTagVotes(dumpPath, sourceIDs, disabledTagUsers)
	if err != nil {
		return nil, err
	}

	byVN := make(map[string][]computedTag, len(sourceIDs))
	seenName := make(map[string]map[string]struct{})
	for _, acc := range accs {
		info, ok := tags[acc.TagID]
		if !ok || strings.TrimSpace(info.Name) == "" {
			continue
		}
		if acc.VoteCount == 0 || acc.SignSum <= 0 {
			continue
		}

		positiveAverage := 3.0
		if acc.Positive > 0 {
			positiveAverage = float64(acc.VoteSum) / float64(acc.Positive)
		}
		rating := positiveAverage * float64(acc.SignSum) / float64(acc.VoteCount)
		if rating <= 0 {
			continue
		}
		if rating > 3 {
			rating = 3
		}

		spoilerLevel := info.DefaultSpoil
		if acc.SpoilerCnt > 0 {
			spoilerAverage := float64(acc.SpoilerSum) / float64(acc.SpoilerCnt)
			switch {
			case spoilerAverage > 1.3:
				spoilerLevel = 2
			case spoilerAverage > 0.4:
				spoilerLevel = 1
			default:
				spoilerLevel = 0
			}
		}
		lie := acc.LieTrue > 0 && acc.LieTrue >= acc.LieFalse
		if lie {
			continue
		}

		nameKey := tagKey(info.Name)
		if seenName[acc.VNID] == nil {
			seenName[acc.VNID] = make(map[string]struct{})
		}
		if _, exists := seenName[acc.VNID][nameKey]; exists {
			continue
		}
		seenName[acc.VNID][nameKey] = struct{}{}

		byVN[acc.VNID] = append(byVN[acc.VNID], computedTag{
			Name:      info.Name,
			Rating:    rating,
			Weight:    rating / 3.0,
			IsSpoiler: spoilerLevel >= 2,
			Order:     acc.Order,
		})
	}

	for vnid, tags := range byVN {
		sort.SliceStable(tags, func(i, j int) bool {
			return tags[i].Order < tags[j].Order
		})
		sortVNDBTagsLikeMetadataGetter(tags)
		if tagLimit >= 0 && len(tags) > tagLimit {
			tags = tags[:tagLimit]
		}
		byVN[vnid] = tags
	}
	return byVN, nil
}

func sortVNDBTagsLikeMetadataGetter(tags []computedTag) {
	for i := 0; i < len(tags)-1; i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[j].Rating > tags[i].Rating {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}
}

func readTagInfo(dumpPath string) (map[string]tagInfo, error) {
	reader, cleanup, err := openDumpTable(dumpPath, "db/tags")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	result := make(map[string]tagInfo)
	scanner := newCopyScanner(reader)
	for scanner.Scan() {
		fields := scanner.Fields()
		if len(fields) < 8 {
			return nil, fmt.Errorf("db/tags row has %d fields, expected at least 8", len(fields))
		}
		id := fields[0]
		defaultSpoil, err := strconv.Atoi(nullToZero(fields[2]))
		if err != nil {
			return nil, fmt.Errorf("parse default spoiler %q for tag %s: %w", fields[2], id, err)
		}
		name := strings.TrimSpace(fields[5])
		if id == "" || name == "" {
			continue
		}
		result[id] = tagInfo{Name: name, DefaultSpoil: defaultSpoil}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read db/tags: %w", err)
	}
	return result, nil
}

func readDisabledTagUsers(dumpPath string) (map[string]struct{}, error) {
	reader, cleanup, err := openDumpTable(dumpPath, "db/users")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	result := make(map[string]struct{})
	scanner := newCopyScanner(reader)
	for scanner.Scan() {
		fields := scanner.Fields()
		if len(fields) < 4 {
			return nil, fmt.Errorf("db/users row has %d fields, expected at least 4", len(fields))
		}
		userID := strings.TrimSpace(fields[0])
		if userID == "" || userID == `\N` {
			continue
		}
		if !parseCopyBool(fields[3]) {
			result[userID] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read db/users: %w", err)
	}
	return result, nil
}

func readTagVotes(dumpPath string, sourceIDs map[string]struct{}, disabledTagUsers map[string]struct{}) (map[string]tagAccumulator, error) {
	reader, cleanup, err := openDumpTable(dumpPath, "db/tags_vn")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	accs := make(map[string]tagAccumulator)
	nextOrder := 0
	scanner := newCopyScanner(reader)
	for scanner.Scan() {
		fields := scanner.Fields()
		if len(fields) < 8 {
			return nil, fmt.Errorf("db/tags_vn row has %d fields, expected at least 8", len(fields))
		}
		tagID := fields[1]
		vnID := normalizeVNID(fields[2])
		if _, wanted := sourceIDs[vnID]; !wanted {
			continue
		}
		if parseCopyBool(fields[6]) {
			continue
		}
		userID := strings.TrimSpace(fields[3])
		if userID != "" && userID != `\N` {
			if _, disabled := disabledTagUsers[userID]; disabled {
				continue
			}
		}

		vote, err := strconv.Atoi(nullToZero(fields[4]))
		if err != nil {
			return nil, fmt.Errorf("parse tag vote %q for %s/%s: %w", fields[4], vnID, tagID, err)
		}

		key := vnID + "\x00" + tagID
		acc := accs[key]
		if acc.TagID == "" {
			acc.VNID = vnID
			acc.TagID = tagID
			acc.Order = nextOrder
			nextOrder++
		}
		acc.VoteCount++
		acc.SignSum += signInt(vote)
		if vote > 0 {
			acc.Positive++
			acc.VoteSum += vote
		}
		if fields[5] != "" && fields[5] != `\N` {
			spoiler, err := strconv.Atoi(fields[5])
			if err != nil {
				return nil, fmt.Errorf("parse spoiler vote %q for %s/%s: %w", fields[5], vnID, tagID, err)
			}
			acc.SpoilerSum += spoiler
			acc.SpoilerCnt++
		}
		if lie, ok := parseNullableCopyBool(fields[7]); ok {
			if lie {
				acc.LieTrue++
			} else {
				acc.LieFalse++
			}
		}
		accs[key] = acc
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read db/tags_vn: %w", err)
	}
	return accs, nil
}

func resolveTarget(rawPath string) (targetInfo, error) {
	abs, err := filepath.Abs(rawPath)
	if err != nil {
		return targetInfo{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return targetInfo{}, fmt.Errorf("target not found: %w", err)
	}
	if !info.IsDir() {
		return targetInfo{kind: targetKindDuckDB, inputPath: abs, dbPath: abs}, nil
	}

	candidates := []string{
		filepath.Join(abs, "database"),
		abs,
	}
	for _, dir := range candidates {
		gamesCSV := filepath.Join(dir, "games.csv")
		if fileExists(gamesCSV) {
			return targetInfo{kind: targetKindCSVExport, inputPath: abs, gamesCSV: gamesCSV}, nil
		}
	}

	dbCandidates, _ := filepath.Glob(filepath.Join(abs, "*.db"))
	if len(dbCandidates) == 1 {
		return targetInfo{kind: targetKindDuckDB, inputPath: abs, dbPath: dbCandidates[0]}, nil
	}
	return targetInfo{}, fmt.Errorf("target directory is neither a LunaBox CSV export nor a directory with exactly one .db file: %s", abs)
}

func loadGames(ctx context.Context, target targetInfo) ([]gameRef, error) {
	switch target.kind {
	case targetKindCSVExport:
		return loadGamesFromCSV(target.gamesCSV)
	case targetKindDuckDB:
		db, err := sql.Open("duckdb", target.dbPath)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return loadGamesFromDuckDB(ctx, db)
	default:
		return nil, fmt.Errorf("unsupported target kind: %d", target.kind)
	}
}

func loadGamesFromCSV(path string) ([]gameRef, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open games csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read games csv header: %w", err)
	}
	index, err := headerIndex(header, "id", "name", "source_type", "source_id")
	if err != nil {
		return nil, err
	}

	var games []gameRef
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read games csv: %w", err)
		}
		sourceType := strings.ToLower(strings.TrimSpace(csvValue(record, index["source_type"])))
		sourceID := normalizeVNID(csvValue(record, index["source_id"]))
		if sourceType != vndbTagSource || sourceID == "" {
			continue
		}
		games = append(games, gameRef{
			GameID:   strings.TrimSpace(csvValue(record, index["id"])),
			Name:     strings.TrimSpace(csvValue(record, index["name"])),
			SourceID: sourceID,
		})
	}
	return games, nil
}

func loadGamesFromDuckDB(ctx context.Context, db *sql.DB) ([]gameRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), COALESCE(source_id, '')
		FROM games
		WHERE LOWER(COALESCE(source_type, '')) = 'vndb'
			AND COALESCE(source_id, '') <> ''
	`)
	if err != nil {
		return nil, fmt.Errorf("query VNDB games: %w", err)
	}
	defer rows.Close()

	var games []gameRef
	for rows.Next() {
		var game gameRef
		if err := rows.Scan(&game.GameID, &game.Name, &game.SourceID); err != nil {
			return nil, fmt.Errorf("scan VNDB game: %w", err)
		}
		game.SourceID = normalizeVNID(game.SourceID)
		if game.SourceID == "" {
			continue
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func openDumpTable(dumpPath string, tableName string) (io.Reader, func(), error) {
	format, err := detectArchiveFormat(dumpPath)
	if err != nil {
		return nil, nil, err
	}

	var source io.Reader
	var cleanup func()
	file, err := os.Open(dumpPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open dump: %w", err)
	}
	cleanup = func() { file.Close() }

	switch format {
	case archiveTarPlain:
		source = file
	case archiveTarGz:
		gz, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("open gzip dump: %w", err)
		}
		source = gz
		cleanup = func() {
			gz.Close()
			file.Close()
		}
	case archiveTarZst:
		cmd := exec.Command("zstd", "-dc", dumpPath)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("open zstd stdout: %w", err)
		}
		if err := cmd.Start(); err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("start zstd -dc: %w", err)
		}
		file.Close()
		source = stdout
		cleanup = func() {
			stdout.Close()
			if err := cmd.Wait(); err != nil {
				_ = err
			}
		}
	default:
		file.Close()
		return nil, nil, fmt.Errorf("unsupported archive format")
	}

	tarReader := tar.NewReader(source)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			cleanup()
			return nil, nil, fmt.Errorf("table %s not found in dump", tableName)
		}
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("read tar header: %w", err)
		}
		if filepath.ToSlash(header.Name) == tableName {
			return tarReader, cleanup, nil
		}
	}
}

func detectArchiveFormat(path string) (archiveFormat, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.zst") || strings.HasSuffix(lower, ".tzst"):
		if _, err := exec.LookPath("zstd"); err != nil {
			return 0, errors.New("zstd executable is required to read .tar.zst dumps")
		}
		return archiveTarZst, nil
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return archiveTarGz, nil
	case strings.HasSuffix(lower, ".tar"):
		return archiveTarPlain, nil
	default:
		return 0, fmt.Errorf("unsupported dump format: %s", path)
	}
}

type copyScanner struct {
	scanner *bufio.Scanner
	fields  []string
	err     error
}

func newCopyScanner(r io.Reader) *copyScanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	return &copyScanner{scanner: scanner}
}

func (s *copyScanner) Scan() bool {
	if s.err != nil {
		return false
	}
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			s.err = err
		}
		return false
	}
	record := strings.Split(s.scanner.Text(), "\t")
	for i := range record {
		record[i] = unescapeCopyValue(record[i])
	}
	s.fields = record
	return true
}

func (s *copyScanner) Fields() []string {
	return s.fields
}

func (s *copyScanner) Err() error {
	return s.err
}

func unescapeCopyValue(value string) string {
	if value == `\N` {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch != '\\' || i+1 >= len(value) {
			b.WriteByte(ch)
			continue
		}
		i++
		switch value[i] {
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

func normalizeVNID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "https://vndb.org/")
	value = strings.TrimPrefix(value, "http://vndb.org/")
	value = strings.TrimPrefix(value, "vndb.org/")
	value = strings.Trim(value, "/")
	if strings.HasPrefix(value, "v") {
		parts := strings.Split(value, "/")
		value = parts[0]
	}
	if !vndbIDPattern.MatchString(value) {
		return ""
	}
	return value
}

func headerIndex(header []string, required ...string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return nil, fmt.Errorf("missing required CSV column %q", name)
		}
	}
	return index, nil
}

func csvValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func parseCopyBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t", "true", "1":
		return true
	default:
		return false
	}
}

func parseNullableCopyBool(value string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == `\N` {
		return false, false
	}
	switch strings.ToLower(trimmed) {
	case "t", "true", "1":
		return true, true
	case "f", "false", "0":
		return false, true
	default:
		return false, false
	}
}

func signInt(value int) int {
	switch {
	case value > 0:
		return 1
	case value < 0:
		return -1
	default:
		return 0
	}
}

func nullToZero(value string) string {
	if value == "" || value == `\N` {
		return "0"
	}
	return value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func tagKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func tagsByName(tags []computedTag) map[string]computedTag {
	result := make(map[string]computedTag, len(tags))
	for _, tag := range tags {
		result[tagKey(tag.Name)] = tag
	}
	return result
}

func sameTagOrder(left []computedTag, right []computedTag) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if tagKey(left[i].Name) != tagKey(right[i].Name) {
			return false
		}
	}
	return true
}

func joinTagNames(tags []computedTag) string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return strings.Join(names, " | ")
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
