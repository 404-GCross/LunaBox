import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useChartTheme } from "../../hooks/useChartTheme";
import { formatDuration } from "../../utils/time";

interface HeatmapCell {
  date: string; // YYYY-MM-DD
  duration: number; // seconds
}

interface PlayHeatmapProps {
  cells: HeatmapCell[];
  className?: string;
}

interface ColumnCell {
  date: Date;
  iso: string;
  duration: number;
  weekday: number; // 0 = Sun .. 6 = Sat
  isPlaceholder?: boolean;
}

const WEEKDAY_LABEL_WIDTH = 28;
const MIN_CELL_SIZE = 8;
const MAX_CELL_SIZE = 28;

function parseISODate(iso: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m)
    return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

function bucketize(duration: number, p50: number, p90: number): number {
  if (duration <= 0)
    return 0;
  if (duration < Math.max(60, p50 * 0.4))
    return 1;
  if (duration < p50)
    return 2;
  if (duration < p90)
    return 3;
  return 4;
}

export function PlayHeatmap({ cells, className = "" }: PlayHeatmapProps) {
  const { t, i18n } = useTranslation();
  const { isDark } = useChartTheme();

  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    if (!containerRef.current)
      return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });
    observer.observe(containerRef.current);
    setContainerWidth(containerRef.current.clientWidth);
    return () => observer.disconnect();
  }, []);

  const { columns, monthLabels, totalDuration, activeDays, maxDuration }
    = useMemo(() => {
      if (!cells.length) {
        return {
          columns: [] as ColumnCell[][],
          monthLabels: [] as { col: number; label: string }[],
          totalDuration: 0,
          activeDays: 0,
          maxDuration: 0,
        };
      }

      const valid = cells
        .map((c) => {
          const d = parseISODate(c.date);
          return d ? { date: d, iso: c.date, duration: c.duration ?? 0 } : null;
        })
        .filter(
          (v): v is { date: Date; iso: string; duration: number } => v !== null,
        )
        .sort((a, b) => a.date.getTime() - b.date.getTime());

      if (!valid.length) {
        return {
          columns: [] as ColumnCell[][],
          monthLabels: [] as { col: number; label: string }[],
          totalDuration: 0,
          activeDays: 0,
          maxDuration: 0,
        };
      }

      const first = valid[0].date;
      const gridStart = new Date(first);
      gridStart.setDate(first.getDate() - first.getDay());

      const last = valid[valid.length - 1].date;
      const gridEnd = new Date(last);
      gridEnd.setDate(last.getDate() + (6 - last.getDay()));

      const dataMap = new Map<string, number>();
      for (const v of valid) dataMap.set(v.iso, v.duration);

      const allCells: ColumnCell[] = [];
      const cursor = new Date(gridStart);
      while (cursor.getTime() <= gridEnd.getTime()) {
        const y = cursor.getFullYear();
        const m = String(cursor.getMonth() + 1).padStart(2, "0");
        const d = String(cursor.getDate()).padStart(2, "0");
        const iso = `${y}-${m}-${d}`;
        const duration = dataMap.get(iso) ?? 0;
        const isPlaceholder
          = cursor.getTime() < first.getTime()
            || cursor.getTime() > last.getTime();
        allCells.push({
          date: new Date(cursor),
          iso,
          duration,
          weekday: cursor.getDay(),
          isPlaceholder,
        });
        cursor.setDate(cursor.getDate() + 1);
      }

      const cols: ColumnCell[][] = [];
      for (let i = 0; i < allCells.length; i += 7) {
        cols.push(allCells.slice(i, i + 7));
      }

      const months: { col: number; label: string }[] = [];
      let lastMonth = -1;
      const monthFormatter = new Intl.DateTimeFormat(i18n.language, {
        month: "short",
      });
      cols.forEach((col, idx) => {
        const firstReal = col.find(c => !c.isPlaceholder);
        if (!firstReal)
          return;
        const m = firstReal.date.getMonth();
        if (m !== lastMonth) {
          months.push({
            col: idx,
            label: monthFormatter.format(firstReal.date),
          });
          lastMonth = m;
        }
      });

      let total = 0;
      let active = 0;
      let max = 0;
      for (const c of allCells) {
        if (c.isPlaceholder)
          continue;
        total += c.duration;
        if (c.duration > 0)
          active += 1;
        if (c.duration > max)
          max = c.duration;
      }

      return {
        columns: cols,
        monthLabels: months,
        totalDuration: total,
        activeDays: active,
        maxDuration: max,
      };
    }, [cells, i18n.language]);

  // 根据容器宽度计算 cell 尺寸：尽量填满，但限制在 [MIN, MAX]
  // 同时让 gap 与 cell 比例自然
  const { cellSize, cellGap } = useMemo(() => {
    if (!columns.length || containerWidth <= 0) {
      return { cellSize: 12, cellGap: 3 };
    }
    const available = Math.max(0, containerWidth - WEEKDAY_LABEL_WIDTH);
    // 通过比例分配：gap 约为 cell 的 1/4
    // (cell + gap) * columns ≈ available
    const step = available / columns.length;
    let size = Math.round(step * 0.8);
    size = Math.max(MIN_CELL_SIZE, Math.min(MAX_CELL_SIZE, size));
    const gap = Math.max(2, Math.round(size / 4));
    return { cellSize: size, cellGap: gap };
  }, [columns.length, containerWidth]);

  const buckets = useMemo(() => {
    const active = cells
      .map(c => c.duration ?? 0)
      .filter(d => d > 0)
      .sort((a, b) => a - b);
    if (!active.length)
      return { p50: 0, p90: 0 };
    const p50 = active[Math.floor(active.length * 0.5)];
    const p90
      = active[Math.min(active.length - 1, Math.floor(active.length * 0.9))];
    return { p50, p90 };
  }, [cells]);

  const colorScale = isDark
    ? ["#1f2937", "#0f3a2e", "#14563f", "#1f8a5c", "#34d399"]
    : ["#f1f5f9", "#bbf7d0", "#86efac", "#34d399", "#059669"];

  const weekdayLabels = [
    t("stats.heatmap.weekdays.sun"),
    t("stats.heatmap.weekdays.mon"),
    t("stats.heatmap.weekdays.tue"),
    t("stats.heatmap.weekdays.wed"),
    t("stats.heatmap.weekdays.thu"),
    t("stats.heatmap.weekdays.fri"),
    t("stats.heatmap.weekdays.sat"),
  ];

  if (!columns.length) {
    return (
      <div
        className={`flex items-center justify-center text-sm text-brand-500 dark:text-brand-400 ${className}`}
      >
        {t("stats.heatmap.empty")}
      </div>
    );
  }

  return (
    <div className={`w-full ${className}`} ref={containerRef}>
      <div className="flex items-center justify-between mb-3 text-xs text-brand-500 dark:text-brand-400">
        <span>
          {t("stats.heatmap.summary", {
            duration: formatDuration(totalDuration, t),
            days: activeDays,
          })}
        </span>
        <span>
          {t("stats.heatmap.maxDay", {
            duration: formatDuration(maxDuration, t),
          })}
        </span>
      </div>

      <div className="w-full" style={{ paddingLeft: WEEKDAY_LABEL_WIDTH }}>
        {/* Month labels - 用百分比定位与下方 flex grid 对齐；最右侧月份改用 right-anchor 防止溢出 */}
        <div className="relative w-full overflow-hidden" style={{ height: 16 }}>
          {monthLabels.map((m, idx) => {
            const remainingCols = columns.length - m.col;
            // 最后一个月份标签若剩余列不足以容纳文字，右对齐到 grid 右边缘
            const useRightAnchor
              = idx === monthLabels.length - 1 && remainingCols <= 3;
            const style = useRightAnchor
              ? { right: 0 }
              : { left: `${(m.col / columns.length) * 100}%` };
            return (
              <span
                key={`${m.col}-${m.label}`}
                className="absolute text-[10px] text-brand-500 dark:text-brand-400 whitespace-nowrap"
                style={style}
              >
                {m.label}
              </span>
            );
          })}
        </div>

        <div className="flex w-full">
          {/* Weekday labels */}
          <div
            className="flex flex-col flex-shrink-0"
            style={{
              gap: cellGap,
              marginLeft: -WEEKDAY_LABEL_WIDTH,
              width: WEEKDAY_LABEL_WIDTH - 4,
            }}
          >
            {weekdayLabels.map((w, idx) => (
              <span
                key={w + idx}
                className="text-[10px] text-brand-500 dark:text-brand-400 text-right pr-1"
                style={{
                  height: cellSize,
                  lineHeight: `${cellSize}px`,
                  visibility: idx % 2 === 1 ? "visible" : "hidden",
                }}
              >
                {w}
              </span>
            ))}
          </div>

          {/* Heat grid: 用 flex-1 列等宽分布，cell 高度=cellSize 保持方形 */}
          <div className="flex flex-1 min-w-0" style={{ gap: cellGap }}>
            {columns.map((col, ci) => (
              <div
                key={ci}
                className="flex flex-col flex-1 min-w-0"
                style={{ gap: cellGap }}
              >
                {col.map((cell, ri) => {
                  if (cell.isPlaceholder) {
                    return <div key={ri} style={{ height: cellSize }} />;
                  }
                  const level = bucketize(
                    cell.duration,
                    buckets.p50,
                    buckets.p90,
                  );
                  return (
                    <div
                      key={ri}
                      title={`${cell.iso} · ${cell.duration > 0 ? formatDuration(cell.duration, t) : t("stats.heatmap.noPlay")}`}
                      className="rounded-[2px] transition-transform hover:scale-125"
                      style={{
                        height: cellSize,
                        backgroundColor: colorScale[level],
                      }}
                    />
                  );
                })}
              </div>
            ))}
          </div>
        </div>

        {/* Legend */}
        <div className="flex items-center justify-end gap-1 mt-3 text-[10px] text-brand-500 dark:text-brand-400">
          <span>{t("stats.heatmap.less")}</span>
          {colorScale.map((c, idx) => (
            <span
              key={idx}
              className="rounded-[2px]"
              style={{ width: cellSize, height: cellSize, backgroundColor: c }}
            />
          ))}
          <span>{t("stats.heatmap.more")}</span>
        </div>
      </div>
    </div>
  );
}

export default PlayHeatmap;
