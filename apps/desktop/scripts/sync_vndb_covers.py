#!/usr/bin/env python3
"""
Download VNDB cover images for every VNDB-backed game in a LunaBox DuckDB database.

This script is intended to run on Linux where rsync is available:

  python3 -m pip install duckdb
  python3 scripts/sync_vndb_covers.py --db /path/to/lunabox.db

It extracts VNDB source IDs from games.source_id, fetches fresh image URLs from
the VNDB API, then downloads only the referenced official image files. API image
URLs look like:

  https://t.vndb.org/cv/51/96151.jpg

The official VNDB rsync endpoint is asked for only those relative files:

  cv/51/96151.jpg

Existing local covers are overwritten. The downloaded files are copied into the
LunaBox covers directory as <game-id>.<ext>, and games.cover_url is updated to
/local/covers/<game-id>.<ext>.
"""

from __future__ import annotations

import argparse
import csv
import json
import os
import re
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import unquote, urlparse
from urllib.request import Request, urlopen


VNDB_RSYNC_ROOT = "rsync://dl.vndb.org/vndb-img/"
VNDB_API_VN_URL = "https://api.vndb.org/kana/vn"
MANAGED_COVER_EXTENSIONS = (".webp", ".jpg", ".jpeg", ".png", ".gif", ".bmp")
SAFE_GAME_ID = re.compile(r"^[A-Za-z0-9._-]+$")
VNDB_ID = re.compile(r"^v\d+$", re.IGNORECASE)


@dataclass(frozen=True)
class GameRow:
    game_id: str
    name: str
    source_id: str
    previous_cover_url: str


@dataclass(frozen=True)
class CoverRow:
    game_id: str
    name: str
    source_id: str
    previous_cover_url: str
    cover_url: str
    rel_path: str


def import_duckdb():
    try:
        import duckdb  # type: ignore
    except ModuleNotFoundError:
        print(
            "Missing Python package: duckdb\n"
            "Install it on Linux with: python3 -m pip install duckdb",
            file=sys.stderr,
        )
        raise SystemExit(2)
    return duckdb


def parse_vndb_image_path(url: str, include_thumbnails: bool) -> str | None:
    parsed = urlparse(url.strip())
    if parsed.scheme not in ("http", "https"):
        return None

    path = unquote(parsed.path).lstrip("/")
    allowed_prefixes = ("cv/", "cv.t/") if include_thumbnails else ("cv/",)
    if not path.startswith(allowed_prefixes):
        return None

    parts = path.split("/")
    if len(parts) != 3:
        return None

    bucket, subdir, filename = parts
    if bucket not in ("cv", "cv.t"):
        return None
    if not re.fullmatch(r"\d{2}", subdir):
        return None
    if not re.fullmatch(r"\d+\.(jpg|jpeg|png|webp|gif)", filename, re.IGNORECASE):
        return None

    return f"{bucket}/{subdir}/{filename}"


def ensure_safe_game_id(game_id: str) -> None:
    if not SAFE_GAME_ID.fullmatch(game_id):
        raise ValueError(f"unsafe game id for filename: {game_id!r}")


def fetch_vndb_games(db_path: Path) -> list[GameRow]:
    duckdb = import_duckdb()
    conn = duckdb.connect(str(db_path), read_only=True)
    try:
        rows = conn.execute(
            """
            SELECT id, name, source_id, cover_url
            FROM games
            WHERE LOWER(COALESCE(source_type, '')) = 'vndb'
              AND COALESCE(source_id, '') <> ''
            ORDER BY id
            """
        ).fetchall()
    finally:
        conn.close()

    result: list[GameRow] = []
    for game_id, name, source_id, cover_url in rows:
        normalized_source_id = str(source_id or "").strip().lower()
        if not VNDB_ID.fullmatch(normalized_source_id):
            continue
        ensure_safe_game_id(str(game_id))
        result.append(
            GameRow(
                game_id=str(game_id),
                name=str(name or ""),
                source_id=normalized_source_id,
                previous_cover_url=str(cover_url or ""),
            )
        )
    return result


def batched(values: list[str], size: int) -> list[list[str]]:
    return [values[index : index + size] for index in range(0, len(values), size)]


def request_vndb_batch(source_ids: list[str], include_thumbnails: bool, retries: int) -> dict[str, str]:
    filters: list[object]
    if len(source_ids) == 1:
        filters = ["id", "=", source_ids[0]]
    else:
        filters = ["or"]
        filters.extend([["id", "=", source_id] for source_id in source_ids])

    payload = {
        "filters": filters,
        "fields": "id, image.url, image.thumbnail",
        "results": len(source_ids),
    }
    body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    last_error: BaseException | None = None

    for attempt in range(retries + 1):
        req = Request(
            VNDB_API_VN_URL,
            data=body,
            headers={
                "Content-Type": "application/json",
                "User-Agent": "LunaBox VNDB cover sync script",
            },
            method="POST",
        )
        try:
            with urlopen(req, timeout=45) as resp:
                data = json.loads(resp.read().decode("utf-8"))
            result: dict[str, str] = {}
            for item in data.get("results", []):
                source_id = str(item.get("id", "")).strip().lower()
                image = item.get("image") or {}
                image_url = str(image.get("url") or "").strip()
                if not image_url and include_thumbnails:
                    image_url = str(image.get("thumbnail") or "").strip()
                if source_id and image_url:
                    result[source_id] = image_url
            return result
        except HTTPError as err:
            last_error = err
            if err.code == 429:
                retry_after = err.headers.get("Retry-After")
                delay = int(retry_after) if retry_after and retry_after.isdigit() else 60
            elif 500 <= err.code < 600:
                delay = min(60, 2 ** attempt)
            else:
                detail = err.read().decode("utf-8", errors="replace")
                raise RuntimeError(f"VNDB API HTTP {err.code}: {detail}") from err
        except (URLError, TimeoutError) as err:
            last_error = err
            delay = min(60, 2 ** attempt)

        if attempt >= retries:
            raise RuntimeError(f"VNDB API request failed after {retries + 1} attempts") from last_error
        print(f"VNDB API batch retry in {delay}s after error: {last_error}", file=sys.stderr, flush=True)
        time.sleep(delay)

    raise RuntimeError("unreachable VNDB API retry state")


def fetch_vndb_image_urls(
    source_ids: list[str],
    include_thumbnails: bool,
    batch_size: int,
    api_sleep: float,
    retries: int,
) -> dict[str, str]:
    unique_ids = sorted(set(source_ids), key=lambda value: int(value[1:]))
    image_urls: dict[str, str] = {}
    batches = batched(unique_ids, batch_size)

    for index, batch in enumerate(batches, start=1):
        print(f"Fetching VNDB image metadata batch {index}/{len(batches)} ({len(batch)} ids)", flush=True)
        image_urls.update(request_vndb_batch(batch, include_thumbnails, retries))
        if api_sleep > 0 and index < len(batches):
            time.sleep(api_sleep)

    return image_urls


def build_cover_rows(
    games: list[GameRow],
    image_urls: dict[str, str],
    include_thumbnails: bool,
) -> list[CoverRow]:
    result: list[CoverRow] = []
    seen: set[tuple[str, str]] = set()

    for game in games:
        cover_url = image_urls.get(game.source_id, "")
        rel_path = parse_vndb_image_path(cover_url, include_thumbnails)
        if rel_path is None:
            continue

        key = (game.game_id, rel_path)
        if key in seen:
            continue
        seen.add(key)
        result.append(
            CoverRow(
                game_id=game.game_id,
                name=game.name,
                source_id=game.source_id,
                previous_cover_url=game.previous_cover_url,
                cover_url=cover_url,
                rel_path=rel_path,
            )
        )

    return result


def write_files_from(rows: list[CoverRow], path: Path) -> None:
    rel_paths = sorted({row.rel_path for row in rows})
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(rel_paths) + "\n", encoding="utf-8")


def write_mapping(rows: list[CoverRow], path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as fp:
        writer = csv.writer(fp, delimiter="\t")
        writer.writerow(["game_id", "name", "source_id", "rel_path", "cover_url", "previous_cover_url"])
        for row in rows:
            writer.writerow(
                [row.game_id, row.name, row.source_id, row.rel_path, row.cover_url, row.previous_cover_url]
            )


def run_rsync(files_from: Path, image_cache_dir: Path, dry_run: bool) -> None:
    image_cache_dir.mkdir(parents=True, exist_ok=True)
    cmd = [
        "rsync",
        "-rtpv",
        "--partial",
        "--info=progress2",
        f"--files-from={files_from}",
    ]
    if dry_run:
        cmd.append("--dry-run")
    cmd.extend([VNDB_RSYNC_ROOT, str(image_cache_dir) + "/"])
    print("Running:", " ".join(cmd), flush=True)
    completed = subprocess.run(cmd, check=False)
    if completed.returncode == 0:
        return
    if completed.returncode == 23:
        action = "No files or database rows were modified by this dry-run." if dry_run else (
            "The script will continue; missing downloaded files will be skipped during copy/update."
        )
        print(
            "Warning: rsync finished with code 23, which means some files/attrs "
            "had errors. Review the earlier rsync lines for the exact missing or failed "
            f"paths. {action}",
            file=sys.stderr,
        )
        return
    raise subprocess.CalledProcessError(completed.returncode, cmd)


def backup_database(db_path: Path) -> Path:
    backup_path = db_path.with_name(db_path.name + ".before-vndb-cover-sync")
    if backup_path.exists():
        index = 1
        while True:
            candidate = db_path.with_name(db_path.name + f".before-vndb-cover-sync.{index}")
            if not candidate.exists():
                backup_path = candidate
                break
            index += 1
    shutil.copy2(db_path, backup_path)
    wal_path = db_path.with_name(db_path.name + ".wal")
    if wal_path.exists():
        shutil.copy2(wal_path, backup_path.with_name(backup_path.name + ".wal"))
    return backup_path


def remove_existing_cover_variants(covers_dir: Path, game_id: str) -> None:
    for ext in MANAGED_COVER_EXTENSIONS:
        candidate = covers_dir / f"{game_id}{ext}"
        if candidate.exists():
            candidate.unlink()


def copy_downloaded_covers(rows: list[CoverRow], image_cache_dir: Path, covers_dir: Path) -> dict[str, str]:
    covers_dir.mkdir(parents=True, exist_ok=True)
    updates: dict[str, str] = {}
    missing = 0

    for row in rows:
        src = image_cache_dir / row.rel_path
        if not src.is_file():
            missing += 1
            continue

        ext = src.suffix.lower()
        if ext not in MANAGED_COVER_EXTENSIONS:
            ext = ".jpg"

        remove_existing_cover_variants(covers_dir, row.game_id)
        dest = covers_dir / f"{row.game_id}{ext}"
        shutil.copy2(src, dest)
        updates[row.game_id] = f"/local/covers/{dest.name}"

    if missing:
        print(f"Missing downloaded files: {missing}", file=sys.stderr)
    return updates


def update_database(db_path: Path, updates: dict[str, str]) -> None:
    if not updates:
        return

    duckdb = import_duckdb()
    conn = duckdb.connect(str(db_path), read_only=False)
    try:
        conn.execute("BEGIN TRANSACTION")
        try:
            for game_id, local_url in updates.items():
                conn.execute(
                    "UPDATE games SET cover_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
                    [local_url, game_id],
                )
            conn.execute("COMMIT")
        except Exception:
            conn.execute("ROLLBACK")
            raise
    finally:
        conn.close()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Refresh covers for every VNDB-backed LunaBox game and rewrite covers to local paths."
    )
    parser.add_argument("--db", required=True, type=Path, help="Path to lunabox.db.")
    parser.add_argument(
        "--covers-dir",
        type=Path,
        help="LunaBox covers directory. Defaults to <db parent>/covers.",
    )
    parser.add_argument(
        "--work-dir",
        type=Path,
        help="Working directory for file lists and rsync cache. Defaults to <db parent>/vndb-cover-sync.",
    )
    parser.add_argument(
        "--no-rsync",
        action="store_true",
        help="Skip rsync and only process files already present in the work cache.",
    )
    parser.add_argument(
        "--no-db-update",
        action="store_true",
        help="Copy cover files but do not update games.cover_url.",
    )
    parser.add_argument(
        "--no-db-backup",
        action="store_true",
        help="Do not create a .before-vndb-cover-sync database backup before updating.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Generate lists and run rsync dry-run; do not copy covers or update the database.",
    )
    parser.add_argument(
        "--include-thumbnails",
        action="store_true",
        help="Use image.thumbnail only if image.url is missing, and accept cv.t paths.",
    )
    parser.add_argument(
        "--api-batch-size",
        type=int,
        default=100,
        help="VNDB IDs per API request. VNDB recommends not exceeding 100.",
    )
    parser.add_argument(
        "--api-sleep",
        type=float,
        default=0.2,
        help="Seconds to sleep between VNDB API batches.",
    )
    parser.add_argument(
        "--api-retries",
        type=int,
        default=3,
        help="Retries for VNDB API batch requests.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    db_path = args.db.expanduser().resolve()
    if not db_path.is_file():
        print(f"Database not found: {db_path}", file=sys.stderr)
        return 2

    covers_dir = (args.covers_dir or db_path.parent / "covers").expanduser().resolve()
    work_dir = (args.work_dir or db_path.parent / "vndb-cover-sync").expanduser().resolve()
    image_cache_dir = work_dir / "vndb-img"
    files_from = work_dir / "vndb-cv-files.txt"
    mapping_file = work_dir / "vndb-cover-map.tsv"

    if args.api_batch_size < 1 or args.api_batch_size > 100:
        print("--api-batch-size must be between 1 and 100.", file=sys.stderr)
        return 2

    games = fetch_vndb_games(db_path)
    image_urls = fetch_vndb_image_urls(
        [game.source_id for game in games],
        args.include_thumbnails,
        args.api_batch_size,
        args.api_sleep,
        args.api_retries,
    )
    rows = build_cover_rows(games, image_urls, args.include_thumbnails)
    write_files_from(rows, files_from)
    write_mapping(rows, mapping_file)

    unique_files = len({row.rel_path for row in rows})
    print(f"VNDB source games: {len(games)}")
    print(f"VNDB image URLs fetched: {len(image_urls)}")
    print(f"Matched games with downloadable covers: {len(rows)}")
    print(f"Unique VNDB image files: {unique_files}")
    print(f"files-from: {files_from}")
    print(f"mapping: {mapping_file}")

    if not rows:
        return 0

    if not args.no_rsync:
        run_rsync(files_from, image_cache_dir, args.dry_run)

    if args.dry_run:
        print("Dry-run complete. Database and covers directory were not modified.")
        return 0

    updates = copy_downloaded_covers(rows, image_cache_dir, covers_dir)
    print(f"Copied covers: {len(updates)}")

    if args.no_db_update:
        print("Skipped database update because --no-db-update was set.")
        return 0

    if not args.no_db_backup:
        backup_path = backup_database(db_path)
        print(f"Database backup: {backup_path}")

    update_database(db_path, updates)
    print(f"Updated database rows: {len(updates)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
