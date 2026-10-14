//go:build ignore

// build_game_id_mapper builds the embedded game ID mapping from the
// HikariNagi CSV export, the Bangumi JSON-lines dump, the VNDB database dump,
// and the PotatoDBMapper SQLite export. Existing mappings may be supplied as
// weak fallback candidates.
//
// The output guarantees one row per VNDB ID and at most one row for every
// non-null Bangumi or Steam ID.
//
// Example:
//
//	go run scripts/build_game_id_mapper.go \
//	  --bangumi dump-2026-08-18.210343Z \
//	  --vndb vndb-db-2026-08-23.tar.zst \
//	  --hikarinagi galgames-production-20260802.csv \
//	  --vn-mapper vn_mapper.db --steam-similarity 0.70 \
//	  --existing internal/service/gamehelper/idmapper/game_id_mapper.db \
//	  --output internal/service/gamehelper/idmapper/game_id_mapper.db
package main

import (
	"archive/tar"
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/klauspost/compress/zstd"
	_ "github.com/mattn/go-sqlite3"
)

type mapping struct {
	VNDBID    int64
	BangumiID int64
	SteamID   int64
}

type candidate struct {
	VNDBID       int64
	BangumiID    int64
	HikariTitles []string
	HikariDate   int
	FromHikari   bool
	FromExisting bool
	Score        int
}

type vndbMetadata struct {
	OfficialTitles map[string]struct{}
	AllTitles      map[string]struct{}
	FirstRelease   int
	SteamIDs       map[int64]struct{}
}

type bangumiMetadata struct {
	Type   int
	Titles map[string]struct{}
	Date   int
}

type buildStats struct {
	CSVRows                  int
	ExistingRows             int
	CandidateVNDB            int
	AmbiguousVNDB            int
	SelectedBangumi          int
	UnresolvedBangumi        int
	DroppedBangumiCollisions int
	SelectedSteam            int
	UnresolvedSteam          int
	DroppedSteamCollisions   int
	VNMapperSteamCandidates  int
}

var bracketAliasPattern = regexp.MustCompile(`\[([^\[\]\r\n]+)\]`)

func main() {
	var bangumiDir string
	var vndbDump string
	var hikariCSV string
	var vnMapperDB string
	var existingDB string
	var outputDB string
	var steamSimilarity float64
	flag.StringVar(&bangumiDir, "bangumi", "dump-2026-08-18.210343Z", "Bangumi dump directory")
	flag.StringVar(&vndbDump, "vndb", "vndb-db-2026-08-23.tar.zst", "VNDB tar.zst dump")
	flag.StringVar(&hikariCSV, "hikarinagi", "galgames-production-20260802.csv", "HikariNagi CSV export")
	flag.StringVar(&vnMapperDB, "vn-mapper", "vn_mapper.db", "PotatoDBMapper SQLite database")
	flag.Float64Var(&steamSimilarity, "steam-similarity", 0.70, "minimum PotatoDBMapper Steam similarity")
	flag.StringVar(&existingDB, "existing", "internal/service/gamehelper/idmapper/game_id_mapper.db", "existing mapper used as fallback")
	flag.StringVar(&outputDB, "output", "internal/service/gamehelper/idmapper/game_id_mapper.db", "output SQLite database")
	flag.Parse()

	if err := run(bangumiDir, vndbDump, hikariCSV, vnMapperDB, existingDB, outputDB, steamSimilarity); err != nil {
		fmt.Fprintln(os.Stderr, "build game ID mapper:", err)
		os.Exit(1)
	}
}

func run(bangumiDir, vndbDump, hikariCSV, vnMapperDB, existingDB, outputDB string, steamSimilarity float64) error {
	if steamSimilarity < 0 || steamSimilarity > 1 {
		return fmt.Errorf("Steam similarity must be between 0 and 1")
	}
	stats := buildStats{}
	candidatesByVNDB := make(map[int64]map[int64]*candidate)
	existingSteam := make(map[int64]int64)

	if strings.TrimSpace(existingDB) != "" {
		existing, err := readExistingMappings(existingDB)
		if err != nil {
			return err
		}
		stats.ExistingRows = len(existing)
		for _, item := range existing {
			if item.VNDBID <= 0 {
				continue
			}
			if candidatesByVNDB[item.VNDBID] == nil {
				candidatesByVNDB[item.VNDBID] = make(map[int64]*candidate)
			}
			if item.BangumiID > 0 {
				entry := ensureCandidate(candidatesByVNDB, item.VNDBID, item.BangumiID)
				entry.FromExisting = true
			}
			if item.SteamID > 0 {
				existingSteam[item.VNDBID] = item.SteamID
			}
		}
	}

	csvRows, err := readHikariCandidates(hikariCSV, candidatesByVNDB)
	if err != nil {
		return err
	}
	stats.CSVRows = csvRows
	vnMapperSteam, err := readVNMapperSteamCandidates(vnMapperDB, steamSimilarity)
	if err != nil {
		return err
	}
	stats.VNMapperSteamCandidates = len(vnMapperSteam)
	for vndbID := range vnMapperSteam {
		if candidatesByVNDB[vndbID] == nil {
			candidatesByVNDB[vndbID] = make(map[int64]*candidate)
		}
	}
	stats.CandidateVNDB = len(candidatesByVNDB)
	for _, candidates := range candidatesByVNDB {
		if len(candidates) > 1 {
			stats.AmbiguousVNDB++
		}
	}

	vndbMeta, err := readVNDBMetadata(vndbDump, candidatesByVNDB)
	if err != nil {
		return err
	}

	wantedBangumi := make(map[int64]struct{})
	for _, entries := range candidatesByVNDB {
		for bangumiID := range entries {
			wantedBangumi[bangumiID] = struct{}{}
		}
	}
	bangumiMeta, err := readBangumiMetadata(filepath.Join(bangumiDir, "subject.jsonlines"), wantedBangumi)
	if err != nil {
		return err
	}

	allCandidates := scoreCandidates(candidatesByVNDB, vndbMeta, bangumiMeta)
	selectedBangumi, droppedBangumi := selectUniqueBangumi(allCandidates)
	stats.DroppedBangumiCollisions = droppedBangumi
	stats.SelectedBangumi = len(selectedBangumi)
	stats.UnresolvedBangumi = stats.CandidateVNDB - stats.SelectedBangumi

	selectedSteam, unresolvedSteam, droppedSteam := selectUniqueSteam(vndbMeta, existingSteam, vnMapperSteam)
	stats.SelectedSteam = len(selectedSteam)
	stats.UnresolvedSteam = unresolvedSteam
	stats.DroppedSteamCollisions = droppedSteam

	rows := make([]mapping, 0, len(candidatesByVNDB))
	for vndbID := range candidatesByVNDB {
		rows = append(rows, mapping{
			VNDBID:    vndbID,
			BangumiID: selectedBangumi[vndbID],
			SteamID:   selectedSteam[vndbID],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].VNDBID < rows[j].VNDBID })

	if err := writeDatabase(outputDB, rows); err != nil {
		return err
	}
	if err := verifyDatabase(outputDB, len(rows)); err != nil {
		return err
	}
	printStats(stats, len(rows))
	return nil
}

type steamEvidence struct {
	SteamID    int64
	Similarity float64
}

func readVNMapperSteamCandidates(path string, minimumSimilarity float64) (map[int64]steamEvidence, error) {
	if strings.TrimSpace(path) == "" {
		return map[int64]steamEvidence{}, nil
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("open VN mapper database: %w", err)
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT VndbId, SteamId, SteamSimilarity
		FROM map
		WHERE SteamId IS NOT NULL
		  AND SteamSimilarity >= ?
		ORDER BY VndbId
	`, minimumSimilarity)
	if err != nil {
		return nil, fmt.Errorf("query VN mapper Steam candidates: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]steamEvidence)
	for rows.Next() {
		var vndbID int64
		var steamID int64
		var similarity float64
		if err := rows.Scan(&vndbID, &steamID, &similarity); err != nil {
			return nil, fmt.Errorf("scan VN mapper Steam candidate: %w", err)
		}
		if vndbID > 0 && steamID > 0 {
			result[vndbID] = steamEvidence{SteamID: steamID, Similarity: similarity}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate VN mapper Steam candidates: %w", err)
	}
	return result, nil
}

func readExistingMappings(path string) ([]mapping, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat existing mapper: %w", err)
	}

	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("open existing mapper: %w", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT vndb_id, bangumi_id, steam_id FROM id_map ORDER BY vndb_id`)
	if err != nil {
		return nil, fmt.Errorf("query existing mapper: %w", err)
	}
	defer rows.Close()

	result := make([]mapping, 0, 30000)
	for rows.Next() {
		var vndbID, bangumiID, steamID sql.NullInt64
		if err := rows.Scan(&vndbID, &bangumiID, &steamID); err != nil {
			return nil, fmt.Errorf("scan existing mapper: %w", err)
		}
		result = append(result, mapping{
			VNDBID:    positiveNullInt(vndbID),
			BangumiID: positiveNullInt(bangumiID),
			SteamID:   positiveNullInt(steamID),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing mapper: %w", err)
	}
	return result, nil
}

func positiveNullInt(value sql.NullInt64) int64 {
	if !value.Valid || value.Int64 <= 0 {
		return 0
	}
	return value.Int64
}

func readHikariCandidates(path string, target map[int64]map[int64]*candidate) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open HikariNagi CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReaderSize(file, 1024*1024))
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read HikariNagi CSV header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[strings.TrimSpace(name)] = index
	}
	required := []string{"vndb_id", "bangumi_game_id", "trans_title", "origin_title", "release_date", "aliases"}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return 0, fmt.Errorf("HikariNagi CSV is missing column %q", name)
		}
	}

	count := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read HikariNagi CSV row: %w", err)
		}
		vndbID := parsePositiveInt(csvValue(record, columns["vndb_id"]))
		bangumiID := parsePositiveInt(csvValue(record, columns["bangumi_game_id"]))
		if vndbID == 0 || bangumiID == 0 {
			continue
		}

		entry := ensureCandidate(target, vndbID, bangumiID)
		entry.FromHikari = true
		entry.HikariDate = parseDate(csvValue(record, columns["release_date"]))
		entry.HikariTitles = appendUniqueStrings(entry.HikariTitles,
			csvValue(record, columns["origin_title"]),
			csvValue(record, columns["trans_title"]),
		)
		entry.HikariTitles = append(entry.HikariTitles, splitLooseAliases(csvValue(record, columns["aliases"]))...)
		count++
	}
	return count, nil
}

func csvValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func ensureCandidate(target map[int64]map[int64]*candidate, vndbID, bangumiID int64) *candidate {
	entries := target[vndbID]
	if entries == nil {
		entries = make(map[int64]*candidate)
		target[vndbID] = entries
	}
	entry := entries[bangumiID]
	if entry == nil {
		entry = &candidate{VNDBID: vndbID, BangumiID: bangumiID}
		entries[bangumiID] = entry
	}
	return entry
}

func readVNDBMetadata(path string, wanted map[int64]map[int64]*candidate) (map[int64]*vndbMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open VNDB dump: %w", err)
	}
	defer file.Close()
	decoder, err := zstd.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open VNDB zstd stream: %w", err)
	}
	defer decoder.Close()

	metadata := make(map[int64]*vndbMetadata, len(wanted))
	for vndbID := range wanted {
		metadata[vndbID] = &vndbMetadata{
			OfficialTitles: make(map[string]struct{}),
			AllTitles:      make(map[string]struct{}),
			SteamIDs:       make(map[int64]struct{}),
		}
	}
	releaseDates := make(map[string]int)
	steamLinks := make(map[int64]int64)

	tarReader := tar.NewReader(decoder)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read VNDB tar entry: %w", err)
		}
		switch header.Name {
		case "db/extlinks":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 3 || fields[1] != "steam" {
					return nil
				}
				linkID := parsePositiveInt(fields[0])
				steamID := parsePositiveInt(fields[2])
				if linkID > 0 && steamID > 0 {
					steamLinks[linkID] = steamID
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB extlinks: %w", err)
			}
		case "db/releases":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 22 || fields[18] == "t" || fields[21] != "t" {
					return nil
				}
				date := parseDate(fields[3])
				if date > 0 {
					releaseDates[fields[0]] = date
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB releases: %w", err)
			}
		case "db/releases_vn":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 2 {
					return nil
				}
				vndbID := parsePrefixedID(fields[1], 'v')
				item := metadata[vndbID]
				if item == nil {
					return nil
				}
				date := releaseDates[fields[0]]
				if date > 0 && (item.FirstRelease == 0 || comparePartialDate(date, item.FirstRelease) < 0) {
					item.FirstRelease = date
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB release relations: %w", err)
			}
		case "db/vn":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 12 {
					return nil
				}
				item := metadata[parsePrefixedID(fields[0], 'v')]
				if item == nil {
					return nil
				}
				for _, alias := range strings.Split(unescapePostgres(fields[11]), "\n") {
					addNormalizedTitle(item.AllTitles, alias)
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB visual novels: %w", err)
			}
		case "db/vn_extlinks":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 2 {
					return nil
				}
				item := metadata[parsePrefixedID(fields[0], 'v')]
				if item == nil {
					return nil
				}
				if steamID := steamLinks[parsePositiveInt(fields[1])]; steamID > 0 {
					item.SteamIDs[steamID] = struct{}{}
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB visual novel links: %w", err)
			}
		case "db/vn_titles":
			if err := scanTSV(tarReader, func(fields []string) error {
				if len(fields) < 5 {
					return nil
				}
				item := metadata[parsePrefixedID(fields[0], 'v')]
				if item == nil {
					return nil
				}
				addNormalizedTitle(item.AllTitles, fields[3])
				addNormalizedTitle(item.AllTitles, nullablePostgres(fields[4]))
				if fields[2] == "t" {
					addNormalizedTitle(item.OfficialTitles, fields[3])
					addNormalizedTitle(item.OfficialTitles, nullablePostgres(fields[4]))
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("read VNDB titles: %w", err)
			}
		}
	}
	return metadata, nil
}

func scanTSV(reader io.Reader, visit func([]string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		if err := visit(strings.Split(scanner.Text(), "\t")); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func readBangumiMetadata(path string, wanted map[int64]struct{}) (map[int64]bangumiMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Bangumi subjects: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 4*1024*1024)
	result := make(map[int64]bangumiMetadata, len(wanted))
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			id := leadingJSONID(line)
			if _, ok := wanted[id]; ok {
				var subject struct {
					ID      int64  `json:"id"`
					Type    int    `json:"type"`
					Name    string `json:"name"`
					NameCN  string `json:"name_cn"`
					Infobox string `json:"infobox"`
					Date    string `json:"date"`
				}
				if decodeErr := json.Unmarshal(line, &subject); decodeErr != nil {
					return nil, fmt.Errorf("decode Bangumi subject %d: %w", id, decodeErr)
				}
				titles := make(map[string]struct{})
				addNormalizedTitle(titles, subject.Name)
				addNormalizedTitle(titles, subject.NameCN)
				for _, match := range bracketAliasPattern.FindAllStringSubmatch(subject.Infobox, -1) {
					addNormalizedTitle(titles, match[1])
				}
				result[id] = bangumiMetadata{Type: subject.Type, Titles: titles, Date: parseDate(subject.Date)}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read Bangumi subjects: %w", err)
		}
	}
	return result, nil
}

func leadingJSONID(line []byte) int64 {
	const prefix = `{"id":`
	if len(line) <= len(prefix) || string(line[:len(prefix)]) != prefix {
		return 0
	}
	end := len(prefix)
	for end < len(line) && line[end] >= '0' && line[end] <= '9' {
		end++
	}
	value, _ := strconv.ParseInt(string(line[len(prefix):end]), 10, 64)
	return value
}

func scoreCandidates(
	byVNDB map[int64]map[int64]*candidate,
	vndbMeta map[int64]*vndbMetadata,
	bangumiMeta map[int64]bangumiMetadata,
) []*candidate {
	result := make([]*candidate, 0)
	for vndbID, entries := range byVNDB {
		vn := vndbMeta[vndbID]
		for _, entry := range entries {
			bgm, hasBangumi := bangumiMeta[entry.BangumiID]
			score := 0
			if entry.FromHikari {
				score += 40
			}
			if entry.FromExisting {
				score += 10
			}
			if hasBangumi {
				score += 20
				if bgm.Type == 4 {
					score += 30
				}
			} else {
				score -= 500
			}

			candidateTitles := cloneTitleSet(bgm.Titles)
			for _, title := range entry.HikariTitles {
				addNormalizedTitle(candidateTitles, title)
			}
			if vn != nil {
				score += titleMatchScore(vn, candidateTitles)
				candidateDate := bgm.Date
				if candidateDate == 0 {
					candidateDate = entry.HikariDate
				}
				score += dateMatchScore(vn.FirstRelease, candidateDate)
			}
			entry.Score = score
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if result[i].FromHikari != result[j].FromHikari {
			return result[i].FromHikari
		}
		if result[i].VNDBID != result[j].VNDBID {
			return result[i].VNDBID < result[j].VNDBID
		}
		return result[i].BangumiID < result[j].BangumiID
	})
	return result
}

func titleMatchScore(vn *vndbMetadata, candidateTitles map[string]struct{}) int {
	if intersects(vn.OfficialTitles, candidateTitles) {
		return 600
	}
	if intersects(vn.AllTitles, candidateTitles) {
		return 500
	}
	best := 0.0
	for left := range vn.AllTitles {
		for right := range candidateTitles {
			if similarity := diceSimilarity(left, right); similarity > best {
				best = similarity
			}
		}
	}
	return int(best * 300)
}

func dateMatchScore(vndbDate, bangumiDate int) int {
	if vndbDate == 0 || bangumiDate == 0 {
		return 0
	}
	if vndbDate == bangumiDate {
		return 400
	}
	vYear, vMonth, _ := splitDate(vndbDate)
	bYear, bMonth, _ := splitDate(bangumiDate)
	if vYear == bYear && vMonth > 0 && vMonth == bMonth {
		return 260
	}
	if vYear == bYear {
		return 160
	}
	difference := vYear - bYear
	if difference < 0 {
		difference = -difference
	}
	if difference == 1 {
		return 40
	}
	return -difference * 5
}

func selectUniqueBangumi(candidates []*candidate) (map[int64]int64, int) {
	selected := make(map[int64]int64)
	usedBangumi := make(map[int64]int64)
	dropped := 0
	for _, entry := range candidates {
		if _, exists := selected[entry.VNDBID]; exists {
			continue
		}
		if _, exists := usedBangumi[entry.BangumiID]; exists {
			dropped++
			continue
		}
		if entry.Score < 0 {
			continue
		}
		selected[entry.VNDBID] = entry.BangumiID
		usedBangumi[entry.BangumiID] = entry.VNDBID
	}
	return selected, dropped
}

func selectUniqueSteam(
	vndbMeta map[int64]*vndbMetadata,
	existing map[int64]int64,
	vnMapper map[int64]steamEvidence,
) (map[int64]int64, int, int) {
	type steamCandidate struct {
		vndbID     int64
		steamID    int64
		confidence int
		similarity float64
	}
	candidates := make([]steamCandidate, 0)
	unresolved := 0
	for vndbID, metadata := range vndbMeta {
		existingID := existing[vndbID]
		vnMapperCandidate := vnMapper[vndbID]
		switch {
		case existingID > 0:
			candidates = append(candidates, steamCandidate{vndbID: vndbID, steamID: existingID, confidence: 3})
		case len(metadata.SteamIDs) == 1:
			for steamID := range metadata.SteamIDs {
				candidates = append(candidates, steamCandidate{vndbID: vndbID, steamID: steamID, confidence: 2, similarity: 1})
			}
		case vnMapperCandidate.SteamID > 0:
			candidates = append(candidates, steamCandidate{
				vndbID: vndbID, steamID: vnMapperCandidate.SteamID,
				confidence: 1, similarity: vnMapperCandidate.Similarity,
			})
		case len(metadata.SteamIDs) > 1:
			unresolved++
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].confidence != candidates[j].confidence {
			return candidates[i].confidence > candidates[j].confidence
		}
		if candidates[i].similarity != candidates[j].similarity {
			return candidates[i].similarity > candidates[j].similarity
		}
		return candidates[i].vndbID < candidates[j].vndbID
	})
	selected := make(map[int64]int64)
	used := make(map[int64]struct{})
	dropped := 0
	for _, entry := range candidates {
		if _, exists := used[entry.steamID]; exists {
			dropped++
			continue
		}
		selected[entry.vndbID] = entry.steamID
		used[entry.steamID] = struct{}{}
	}
	return selected, unresolved, dropped
}

func writeDatabase(path string, rows []mapping) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create mapper directory: %w", err)
	}
	tempPath := path + ".new"
	backupPath := path + ".bak"
	_ = os.Remove(tempPath)
	_ = os.Remove(backupPath)

	db, err := sql.Open("sqlite3", tempPath)
	if err != nil {
		return fmt.Errorf("create mapper database: %w", err)
	}
	succeeded := false
	defer func() {
		_ = db.Close()
		if !succeeded {
			_ = os.Remove(tempPath)
		}
	}()

	statements := []string{
		`PRAGMA journal_mode = DELETE`,
		`PRAGMA synchronous = OFF`,
		`CREATE TABLE id_map (
			vndb_id INTEGER NOT NULL PRIMARY KEY CHECK (vndb_id > 0),
			bangumi_id INTEGER CHECK (bangumi_id IS NULL OR bangumi_id > 0),
			steam_id INTEGER CHECK (steam_id IS NULL OR steam_id > 0)
		)`,
		`CREATE UNIQUE INDEX idx_id_map_bangumi_id ON id_map (bangumi_id) WHERE bangumi_id IS NOT NULL`,
		`CREATE UNIQUE INDEX idx_id_map_steam_id ON id_map (steam_id) WHERE steam_id IS NOT NULL`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("initialize mapper database: %w", err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin mapper insert: %w", err)
	}
	statement, err := tx.Prepare(`INSERT INTO id_map (vndb_id, bangumi_id, steam_id) VALUES (?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare mapper insert: %w", err)
	}
	for _, item := range rows {
		if _, err := statement.Exec(item.VNDBID, nullableID(item.BangumiID), nullableID(item.SteamID)); err != nil {
			_ = statement.Close()
			_ = tx.Rollback()
			return fmt.Errorf("insert VNDB v%d mapping: %w", item.VNDBID, err)
		}
	}
	if err := statement.Close(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("close mapper insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mapper insert: %w", err)
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		return fmt.Errorf("vacuum mapper database: %w", err)
	}
	if _, err := db.Exec(`ANALYZE`); err != nil {
		return fmt.Errorf("analyze mapper database: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close mapper database: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("backup previous mapper database: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat previous mapper database: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return fmt.Errorf("activate mapper database: %w", err)
	}
	_ = os.Remove(backupPath)
	succeeded = true
	return nil
}

func verifyDatabase(path string, expectedRows int) error {
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro&_query_only=1")
	if err != nil {
		return fmt.Errorf("open generated mapper for verification: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("verify mapper integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("mapper integrity check returned %q", integrity)
	}
	var rows, vndbIDs, bangumiIDs, steamIDs int
	if err := db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT vndb_id),
		       COUNT(DISTINCT bangumi_id), COUNT(DISTINCT steam_id)
		FROM id_map
	`).Scan(&rows, &vndbIDs, &bangumiIDs, &steamIDs); err != nil {
		return fmt.Errorf("verify mapper counts: %w", err)
	}
	if rows != expectedRows || rows != vndbIDs {
		return fmt.Errorf("VNDB uniqueness failed: rows=%d distinct=%d expected=%d", rows, vndbIDs, expectedRows)
	}
	var duplicateBangumi, duplicateSteam int
	if err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM (SELECT bangumi_id FROM id_map WHERE bangumi_id IS NOT NULL GROUP BY bangumi_id HAVING COUNT(*) > 1)),
			(SELECT COUNT(*) FROM (SELECT steam_id FROM id_map WHERE steam_id IS NOT NULL GROUP BY steam_id HAVING COUNT(*) > 1))
	`).Scan(&duplicateBangumi, &duplicateSteam); err != nil {
		return fmt.Errorf("verify mapper reverse uniqueness: %w", err)
	}
	if duplicateBangumi != 0 || duplicateSteam != 0 {
		return fmt.Errorf("reverse uniqueness failed: bangumi=%d steam=%d", duplicateBangumi, duplicateSteam)
	}
	return nil
}

func printStats(stats buildStats, rows int) {
	fmt.Printf("rows=%d\n", rows)
	fmt.Printf("csv_rows=%d\n", stats.CSVRows)
	fmt.Printf("existing_rows=%d\n", stats.ExistingRows)
	fmt.Printf("candidate_vndb=%d\n", stats.CandidateVNDB)
	fmt.Printf("ambiguous_vndb_before=%d\n", stats.AmbiguousVNDB)
	fmt.Printf("selected_bangumi=%d\n", stats.SelectedBangumi)
	fmt.Printf("unresolved_bangumi=%d\n", stats.UnresolvedBangumi)
	fmt.Printf("dropped_bangumi_collisions=%d\n", stats.DroppedBangumiCollisions)
	fmt.Printf("selected_steam=%d\n", stats.SelectedSteam)
	fmt.Printf("unresolved_steam=%d\n", stats.UnresolvedSteam)
	fmt.Printf("dropped_steam_collisions=%d\n", stats.DroppedSteamCollisions)
	fmt.Printf("vn_mapper_steam_candidates=%d\n", stats.VNMapperSteamCandidates)
}

func nullableID(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func parsePositiveInt(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func parsePrefixedID(value string, prefix byte) int64 {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != prefix {
		return 0
	}
	return parsePositiveInt(value[1:])
}

func parseDate(value string) int {
	digits := make([]byte, 0, 8)
	for index := 0; index < len(value) && len(digits) < 8; index++ {
		if value[index] >= '0' && value[index] <= '9' {
			digits = append(digits, value[index])
		}
	}
	if len(digits) < 4 {
		return 0
	}
	for len(digits) < 8 {
		digits = append(digits, '0')
	}
	parsed, _ := strconv.Atoi(string(digits))
	return parsed
}

func comparePartialDate(left, right int) int {
	leftComparable := comparableDate(left)
	rightComparable := comparableDate(right)
	switch {
	case leftComparable < rightComparable:
		return -1
	case leftComparable > rightComparable:
		return 1
	default:
		return 0
	}
}

func comparableDate(value int) int {
	year, month, day := splitDate(value)
	if month == 0 {
		month = 1
	}
	if day == 0 {
		day = 1
	}
	return year*10000 + month*100 + day
}

func splitDate(value int) (int, int, int) {
	return value / 10000, (value / 100) % 100, value % 100
}

func normalizeTitle(value string) string {
	return strings.Map(func(char rune) rune {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			return unicode.ToLower(char)
		default:
			return -1
		}
	}, strings.TrimSpace(value))
}

func addNormalizedTitle(target map[string]struct{}, value string) {
	if normalized := normalizeTitle(nullablePostgres(unescapePostgres(value))); normalized != "" {
		target[normalized] = struct{}{}
	}
}

func cloneTitleSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}

func intersects(left, right map[string]struct{}) bool {
	if len(left) > len(right) {
		left, right = right, left
	}
	for value := range left {
		if _, ok := right[value]; ok {
			return true
		}
	}
	return false
}

func diceSimilarity(left, right string) float64 {
	if left == right && left != "" {
		return 1
	}
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) < 2 || len(rightRunes) < 2 {
		return 0
	}
	leftPairs := make(map[string]int, len(leftRunes)-1)
	for index := 0; index < len(leftRunes)-1; index++ {
		leftPairs[string(leftRunes[index:index+2])]++
	}
	intersection := 0
	for index := 0; index < len(rightRunes)-1; index++ {
		pair := string(rightRunes[index : index+2])
		if leftPairs[pair] > 0 {
			intersection++
			leftPairs[pair]--
		}
	}
	return float64(2*intersection) / float64(len(leftRunes)+len(rightRunes)-2)
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func splitLooseAliases(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "{}" {
		return nil
	}
	value = strings.TrimPrefix(value, "{")
	value = strings.TrimSuffix(value, "}")
	parts := strings.FieldsFunc(value, func(char rune) bool {
		return char == ',' || char == '\n' || char == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"`)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func nullablePostgres(value string) string {
	if value == `\N` {
		return ""
	}
	return value
}

func unescapePostgres(value string) string {
	replacer := strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t", `\\`, `\`)
	return replacer.Replace(value)
}
