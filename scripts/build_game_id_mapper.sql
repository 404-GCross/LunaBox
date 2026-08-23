-- Builds game_id_mapper.db from the checked-in source exports.
-- Production CSV rows take precedence for VNDB-to-Bangumi associations.
-- PotatoVN associations are retained only for VNDB IDs absent from the CSV,
-- and only where the source similarity is at least 0.95.

PRAGMA journal_mode = DELETE;
PRAGMA synchronous = OFF;

.mode csv
.import 'galgames-production-20260802.csv' source_csv

ATTACH 'PotatoVN/GalgameManager/Assets/Data/vn_mapper.db' AS potato;

CREATE TEMP TABLE csv_mapping (
    vndb_id INTEGER NOT NULL,
    bangumi_id INTEGER NOT NULL,
    PRIMARY KEY (vndb_id, bangumi_id)
) WITHOUT ROWID;

INSERT OR IGNORE INTO csv_mapping (vndb_id, bangumi_id)
SELECT
    CAST(TRIM(vndb_id) AS INTEGER),
    CAST(TRIM(bangumi_game_id) AS INTEGER)
FROM source_csv
WHERE TRIM(vndb_id) <> ''
  AND TRIM(bangumi_game_id) <> ''
  AND TRIM(vndb_id) NOT GLOB '*[^0-9]*'
  AND TRIM(bangumi_game_id) NOT GLOB '*[^0-9]*';

CREATE TEMP TABLE csv_vndb (
    vndb_id INTEGER PRIMARY KEY
) WITHOUT ROWID;

INSERT INTO csv_vndb (vndb_id)
SELECT DISTINCT vndb_id
FROM csv_mapping;

CREATE TABLE id_map (
    vndb_id INTEGER,
    bangumi_id INTEGER,
    steam_id INTEGER
);

-- The CSV is authoritative for its VNDB-Bangumi pairs.  Steam is filled from
-- the PotatoVN record for the same VNDB ID only at high similarity.
INSERT INTO id_map (vndb_id, bangumi_id, steam_id)
SELECT DISTINCT
    c.vndb_id,
    c.bangumi_id,
    CASE
        WHEN p.SteamSimilarity >= 0.95 THEN p.SteamId
    END
FROM csv_mapping AS c
LEFT JOIN potato.map AS p
    ON p.VndbId = c.vndb_id;

-- Fill coverage for VNDB IDs missing from the production export.  Low-score
-- PotatoVN associations are excluded to prevent incorrect enrichment.
INSERT INTO id_map (vndb_id, bangumi_id, steam_id)
SELECT
    p.VndbId,
    CASE WHEN p.BgmSimilarity >= 0.95 THEN p.BgmId END,
    CASE WHEN p.SteamSimilarity >= 0.95 THEN p.SteamId END
FROM potato.map AS p
LEFT JOIN csv_vndb AS c ON c.vndb_id = p.VndbId
WHERE c.vndb_id IS NULL
  AND (p.BgmSimilarity >= 0.95 OR p.SteamSimilarity >= 0.95);

CREATE INDEX idx_id_map_vndb_id ON id_map (vndb_id);
CREATE INDEX idx_id_map_bangumi_id ON id_map (bangumi_id);
CREATE INDEX idx_id_map_steam_id ON id_map (steam_id);

DROP TABLE source_csv;
DETACH DATABASE potato;
VACUUM;
ANALYZE;
