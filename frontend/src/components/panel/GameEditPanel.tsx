import type { ClipboardEvent } from "react";
import type { models } from "../../../wailsjs/go/models";
import type { BetterDataTableColumn } from "../ui/better/BetterDataTable";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  DownloadCoverImage,
  OpenLocalPath,
  SaveCoverImageDataURL,
} from "../../../wailsjs/go/service/GameService";
import { formatDateInputValue, formatDateToYYYYMMDD } from "../../utils/time";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterDataTable } from "../ui/better/BetterDataTable";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameEditFormProps {
  game: models.Game;
  onGameChange: (game: models.Game) => void;
  onDelete: () => void;
  onSelectExecutable: () => void;
  onSelectSaveDirectory: () => void;
  onSelectSaveFile: () => void;
  onSelectCoverImage: () => void;
  onCoverImageChanged?: () => void;
  onUpdateFromRemote?: () => void;
}

interface ReleaseDateRow {
  week: string;
  dates: Date[];
}

const weekdayLabels = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];

function parseDateInputValue(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match)
    return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);

  if (
    date.getFullYear() !== year
    || date.getMonth() !== month - 1
    || date.getDate() !== day
  ) {
    return null;
  }

  return date;
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function formatMonthLabel(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${year}/${month}`;
}

function getCalendarRows(monthDate: Date): ReleaseDateRow[] {
  const firstDay = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
  const start = new Date(firstDay);
  start.setDate(firstDay.getDate() - firstDay.getDay());

  return Array.from({ length: 6 }, (_, weekIndex) => ({
    week: `week-${weekIndex}`,
    dates: Array.from({ length: 7 }, (__, dayIndex) => {
      const date = new Date(start);
      date.setDate(start.getDate() + weekIndex * 7 + dayIndex);
      return date;
    }),
  }));
}

function ReleaseDatePicker({
  value,
  label,
  onChange,
}: {
  value: string;
  label: string;
  onChange: (value: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const selectedDate = parseDateInputValue(value);
  const [monthDate, setMonthDate] = useState(() =>
    startOfMonth(selectedDate ?? new Date()),
  );
  const [isOpen, setIsOpen] = useState(false);
  const rows = getCalendarRows(monthDate);
  const displayValue = value ? value.replaceAll("-", "/") : "";
  const columns: BetterDataTableColumn<ReleaseDateRow>[] = weekdayLabels.map(
    (weekday, dayIndex) => ({
      key: weekday,
      header: weekday,
      className: "w-[14.285%]",
      headerClassName: "text-center",
      cellClassName: "p-1 text-center",
      render: (row) => {
        const date = row.dates[dayIndex];
        const dateValue = formatDateToYYYYMMDD(date);
        const isOutside = date.getMonth() !== monthDate.getMonth();
        const isSelected = dateValue === value;

        return (
          <button
            type="button"
            onClick={() => {
              onChange(dateValue);
              setIsOpen(false);
            }}
            className={[
              "inline-flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors",
              "focus:outline-none focus:ring-2 focus:ring-neutral-500/30",
              isOutside
                ? "text-brand-300 hover:text-brand-600 dark:text-brand-600 dark:hover:text-brand-300"
                : "text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-700",
              isSelected
                ? "bg-neutral-800 text-white hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-white"
                : "",
            ].join(" ")}
          >
            {date.getDate()}
          </button>
        );
      },
    }),
  );

  useEffect(() => {
    if (!isOpen)
      return;

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && !containerRef.current?.contains(target)) {
        setIsOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape")
        setIsOpen(false);
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-expanded={isOpen}
        onClick={() => setIsOpen(open => !open)}
        className={[
          "glass-input flex min-h-10 w-full min-w-0 items-center justify-between gap-3",
          "rounded-md border border-brand-300 bg-white px-3 py-2 text-left",
          "text-brand-900 outline-none transition-colors",
          "focus:ring-2 focus:ring-neutral-500",
          "dark:border-brand-600 dark:bg-brand-700 dark:text-white",
        ].join(" ")}
      >
        <span
          className={[
            "min-w-0 flex-1 truncate",
            displayValue ? "" : "text-brand-400 dark:text-brand-500",
          ].join(" ")}
        >
          {displayValue || label}
        </span>
        <span
          className="i-mdi-calendar-month-outline shrink-0 text-lg text-brand-500 dark:text-brand-300"
          aria-hidden="true"
        />
      </button>

      {isOpen && (
        <div className="absolute left-0 top-full z-[9999] mt-2 w-[22rem] max-w-[calc(100vw-2rem)] rounded-xl border border-brand-200 bg-white p-3 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90">
          <div className="space-y-3">
            <div className="grid h-9 grid-cols-[4rem_1fr_4rem] items-center">
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  aria-label="Previous year"
                  onClick={() => setMonthDate(date => addMonths(date, -12))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-double-left text-lg" />
                </button>
                <button
                  type="button"
                  aria-label="Previous month"
                  onClick={() => setMonthDate(date => addMonths(date, -1))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-left text-lg" />
                </button>
              </div>
              <div className="text-center text-sm font-semibold text-brand-900 dark:text-white">
                {formatMonthLabel(monthDate)}
              </div>
              <div className="flex items-center justify-end gap-1">
                <button
                  type="button"
                  aria-label="Next month"
                  onClick={() => setMonthDate(date => addMonths(date, 1))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-right text-lg" />
                </button>
                <button
                  type="button"
                  aria-label="Next year"
                  onClick={() => setMonthDate(date => addMonths(date, 12))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-double-right text-lg" />
                </button>
              </div>
            </div>

            <BetterDataTable
              rows={rows}
              columns={columns}
              rowKey={row => row.week}
              maxHeightClassName="max-h-none"
            />
          </div>
        </div>
      )}
    </div>
  );
}

function readBlobAsDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => {
      if (typeof reader.result === "string") {
        resolve(reader.result);
        return;
      }
      reject(new Error("clipboard-image-read-failed"));
    });
    reader.addEventListener("error", () => {
      reject(reader.error ?? new Error("clipboard-image-read-failed"));
    });
    reader.readAsDataURL(blob);
  });
}

function getClipboardImageBlob(items: DataTransferItemList): Blob | null {
  for (const item of Array.from(items)) {
    if (item.kind !== "file" || !item.type.startsWith("image/"))
      continue;

    const file = item.getAsFile();
    if (file)
      return file;
  }
  return null;
}

function isRemoteCoverURL(coverURL: string): boolean {
  const trimmedURL = coverURL.trim();
  const normalizedURL = trimmedURL.toLowerCase();
  return (
    (normalizedURL.startsWith("http://")
      || normalizedURL.startsWith("https://"))
    && !normalizedURL.includes("wails.localhost")
  );
}

export function GameEditPanel({
  game,
  onGameChange,
  onDelete,
  onSelectExecutable,
  onSelectSaveDirectory,
  onSelectSaveFile,
  onSelectCoverImage,
  onCoverImageChanged,
  onUpdateFromRemote,
}: GameEditFormProps) {
  const { t } = useTranslation();
  const [isDownloadingCover, setIsDownloadingCover] = useState(false);
  const releaseDateInputValue = formatDateInputValue(game.release_date);
  const hasUnsupportedReleaseDate
    = Boolean(game.release_date) && releaseDateInputValue === "";
  const canDownloadCover
    = isRemoteCoverURL(game.cover_url) && !isDownloadingCover;

  const importCoverDataURL = async (dataURL: string) => {
    const coverUrl = await SaveCoverImageDataURL(game.id, dataURL);
    if (coverUrl) {
      onGameChange({
        ...game,
        cover_url: coverUrl,
      } as models.Game);
      onCoverImageChanged?.();
    }
    toast.success(t("gameEdit.importFromClipboardSuccess"));
  };

  const handleCoverPaste = async (event: ClipboardEvent<HTMLInputElement>) => {
    const imageBlob = getClipboardImageBlob(event.clipboardData.items);
    if (!imageBlob)
      return;

    event.preventDefault();
    try {
      await importCoverDataURL(await readBlobAsDataURL(imageBlob));
    }
    catch (error) {
      console.error("Failed to import pasted cover image:", error);
      toast.error(t("gameEdit.importFromClipboardFailed"));
    }
  };

  const handleDownloadCover = async () => {
    if (!isRemoteCoverURL(game.cover_url))
      return;

    setIsDownloadingCover(true);
    try {
      const coverUrl = await DownloadCoverImage(game.id, game.cover_url);
      if (coverUrl) {
        onGameChange({
          ...game,
          cover_url: coverUrl,
        } as models.Game);
        onCoverImageChanged?.();
      }
      toast.success(t("gameEdit.downloadCoverSuccess"));
    }
    catch (error) {
      console.error("Failed to download cover image:", error);
      toast.error(t("gameEdit.downloadCoverFailed"));
    }
    finally {
      setIsDownloadingCover(false);
    }
  };

  return (
    <div className="glass-panel w-full min-w-0 bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.name")}
          </label>
          <input
            type="text"
            value={game.name}
            onChange={e =>
              onGameChange({ ...game, name: e.target.value } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.cover")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.cover_url}
              onChange={e =>
                onGameChange({
                  ...game,
                  cover_url: e.target.value,
                } as models.Game)}
              onPaste={handleCoverPaste}
              placeholder={t("gameEdit.coverPlaceholder")}
              className="glass-input min-w-0 flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <BetterButton
              onClick={onSelectCoverImage}
              icon="i-mdi-image"
              aria-label={t("gameEdit.selectImage")}
              className="shrink-0"
            />
            <BetterButton
              onClick={handleDownloadCover}
              disabled={!canDownloadCover}
              isLoading={isDownloadingCover}
              icon="i-mdi-download"
              aria-label={t("gameEdit.downloadCover")}
              className="shrink-0"
            />
          </div>
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.coverHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.developer")}
          </label>
          <input
            type="text"
            value={game.company}
            onChange={e =>
              onGameChange({
                ...game,
                company: e.target.value,
              } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.rating")}
            </label>
            <div className="flex items-center gap-2">
              <input
                type="number"
                min={0}
                max={10}
                step={0.1}
                inputMode="decimal"
                value={game.rating > 0 ? game.rating : ""}
                onChange={(e) => {
                  const rawValue = e.target.value;
                  const nextRating
                    = rawValue === ""
                      ? 0
                      : Math.min(10, Math.max(0, Number(rawValue)));
                  onGameChange({
                    ...game,
                    rating: Number.isFinite(nextRating) ? nextRating : 0,
                  } as models.Game);
                }}
                placeholder={t("gameEdit.ratingPlaceholder")}
                className="glass-input min-w-0 flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <span className="shrink-0 text-sm text-brand-500 dark:text-brand-400">
                / 10
              </span>
            </div>
            <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.ratingHint")}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.releaseDate")}
            </label>
            <ReleaseDatePicker
              value={releaseDateInputValue}
              label={t("gameEdit.releaseDate")}
              onChange={value =>
                onGameChange({
                  ...game,
                  release_date: value,
                } as models.Game)}
            />
            {hasUnsupportedReleaseDate && (
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("gameEdit.releaseDateRawHint", {
                  value: game.release_date,
                })}
              </p>
            )}
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.path")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.path}
              onChange={e =>
                onGameChange({ ...game, path: e.target.value } as models.Game)}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <BetterButton
                onClick={onSelectExecutable}
                icon="i-mdi-file"
                aria-label={t("gameEdit.selectFile")}
              />
              <BetterButton
                onClick={async () => {
                  try {
                    await OpenLocalPath(game.path);
                  }
                  catch {
                    toast.error(t("gameEdit.openPathFailed"));
                  }
                }}
                disabled={!game.path}
                icon="i-mdi-folder-open"
                aria-label={t("gameEdit.openInExplorer")}
              />
            </div>
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.savePath")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.save_path || ""}
              onChange={e =>
                onGameChange({
                  ...game,
                  save_path: e.target.value,
                } as models.Game)}
              placeholder={t("gameEdit.savePathPlaceholder")}
              className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <div className="flex items-center gap-1">
              <BetterButton
                onClick={onSelectSaveDirectory}
                icon="i-mdi-folder"
                aria-label={t("gameEdit.selectFolder")}
              />
              <BetterButton
                onClick={onSelectSaveFile}
                icon="i-mdi-file"
                aria-label={t("gameEdit.selectFile")}
              />
              <BetterButton
                onClick={async () => {
                  if (!game.save_path)
                    return;
                  try {
                    await OpenLocalPath(game.save_path);
                  }
                  catch {
                    toast.error(t("gameEdit.openPathFailed"));
                  }
                }}
                disabled={!game.save_path}
                icon="i-mdi-folder-open"
                aria-label={t("gameEdit.openInExplorer")}
              />
            </div>
          </div>
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.savePathHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.summary")}
          </label>
          <textarea
            value={game.summary}
            onChange={e =>
              onGameChange({ ...game, summary: e.target.value } as models.Game)}
            rows={6}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none resize-none"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.sourceType")}
            </label>
            <BetterSelect
              value={game.source_type || ""}
              onChange={value =>
                onGameChange({ ...game, source_type: value } as models.Game)}
              options={[
                { value: "", label: t("gameEdit.sourceNone") },
                { value: "local", label: t("gameEdit.sourceLocal") },
                { value: "bangumi", label: "Bangumi" },
                { value: "vndb", label: "VNDB" },
                { value: "ymgal", label: t("gameEdit.sourceYmgal") },
                { value: "steam", label: "Steam" },
                { value: "dlsite", label: t("gameEdit.sourceDlsite") },
                { value: "touchgal", label: t("gameEdit.sourceTouchGal") },
                {
                  value: "erogamescape",
                  label: t("gameEdit.sourceErogameScape"),
                },
              ]}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.sourceId")}
            </label>
            <input
              type="text"
              value={game.source_id || ""}
              onChange={e =>
                onGameChange({
                  ...game,
                  source_id: e.target.value,
                } as models.Game)}
              placeholder={t("gameEdit.sourceIdPlaceholder")}
              className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
          </div>
        </div>

        <div className="data-glass:bg-white/2 data-glass:dark:bg-black/2 flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-4 dark:border-brand-700 dark:bg-brand-700/50">
          <div className="flex-1 space-y-2">
            <label
              htmlFor="game-metadata-lock"
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
            >
              {t("gameEdit.metadataLock")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.metadataLockHint")}
            </p>
          </div>
          <BetterSwitch
            id="game-metadata-lock"
            checked={Boolean(game.metadata_locked)}
            onCheckedChange={checked =>
              onGameChange({
                ...game,
                metadata_locked: checked,
              } as models.Game)}
          />
        </div>

        <div className="data-glass:bg-white/2 data-glass:dark:bg-black/2 flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-4 dark:border-brand-700 dark:bg-brand-700/50">
          <div className="flex-1 space-y-2">
            <label
              htmlFor="game-is-nsfw"
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
            >
              {t("gameEdit.isNsfw")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.isNsfwHint")}
            </p>
          </div>
          <BetterSwitch
            id="game-is-nsfw"
            checked={Boolean(game.is_nsfw)}
            onCheckedChange={checked =>
              onGameChange({
                ...game,
                is_nsfw: checked,
              } as models.Game)}
          />
        </div>

        <div className="flex justify-between pt-4">
          <div className="flex gap-4 justify-end w-full">
            {onUpdateFromRemote && (
              <BetterButton
                variant="primary"
                onClick={onUpdateFromRemote}
                disabled={Boolean(game.metadata_locked)}
                icon="i-mdi-cloud-sync"
              >
                {t("gameEdit.updateFromRemote")}
              </BetterButton>
            )}
            <BetterButton
              variant="danger"
              onClick={onDelete}
              icon="i-mdi-trash-can-outline"
            >
              {t("common.delete")}
            </BetterButton>
          </div>
        </div>
      </div>
    </div>
  );
}
