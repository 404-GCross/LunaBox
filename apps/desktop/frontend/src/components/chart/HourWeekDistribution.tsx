import type { ChartData, ChartOptions, TooltipItem } from "chart.js";
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  LinearScale,
  Tooltip,
} from "chart.js";
import { useMemo } from "react";
import { Bar } from "react-chartjs-2";
import { useTranslation } from "react-i18next";
import { useChartTheme } from "../../hooks/useChartTheme";
import { formatDuration, formatDurationChart } from "../../utils/time";

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip);

interface HourPoint {
  hour: number;
  duration: number;
}

interface WeekdayPoint {
  weekday: number;
  duration: number;
}

interface HourWeekDistributionProps {
  hourly: HourPoint[];
  weekday: WeekdayPoint[];
  className?: string;
}

export function HourWeekDistribution({
  hourly,
  weekday,
  className = "",
}: HourWeekDistributionProps) {
  const { t } = useTranslation();
  const { isDark, textColor, gridColor } = useChartTheme();

  const { peakHour, hourMax, totalHour } = useMemo(() => {
    let max = 0;
    let peak = -1;
    let total = 0;
    for (const h of hourly) {
      total += h.duration;
      if (h.duration > max) {
        max = h.duration;
        peak = h.hour;
      }
    }
    return { peakHour: peak, hourMax: max, totalHour: total };
  }, [hourly]);

  const { peakWeekday, weekdayMax } = useMemo(() => {
    let max = 0;
    let peak = -1;
    for (const w of weekday) {
      if (w.duration > max) {
        max = w.duration;
        peak = w.weekday;
      }
    }
    return { peakWeekday: peak, weekdayMax: max };
  }, [weekday]);

  const weekdayLabels = useMemo(
    () => [
      t("stats.heatmap.weekdays.sun"),
      t("stats.heatmap.weekdays.mon"),
      t("stats.heatmap.weekdays.tue"),
      t("stats.heatmap.weekdays.wed"),
      t("stats.heatmap.weekdays.thu"),
      t("stats.heatmap.weekdays.fri"),
      t("stats.heatmap.weekdays.sat"),
    ],
    [t],
  );

  const barFill = isDark ? "#34d399" : "#10b981";
  const barFillPeak = isDark ? "#fbbf24" : "#f59e0b";

  const hourlyData: ChartData<"bar"> = useMemo(
    () => ({
      labels: Array.from({ length: 24 }, (_, i) => String(i)),
      datasets: [
        {
          data: hourly.map(h => h.duration),
          backgroundColor: hourly.map(h =>
            h.hour === peakHour && h.duration > 0 ? barFillPeak : barFill,
          ),
          borderRadius: 2,
          maxBarThickness: 18,
        },
      ],
    }),
    [hourly, peakHour, barFill, barFillPeak],
  );

  const hourlyOptions = useMemo<ChartOptions<"bar">>(
    () => ({
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            title: (items: TooltipItem<"bar">[]) =>
              `${String(items[0]?.label ?? "").padStart(2, "0")}:00`,
            label: (ctx: TooltipItem<"bar">) => {
              const v = Number(ctx.parsed.y || 0);
              return v > 0 ? formatDuration(v, t) : t("stats.heatmap.noPlay");
            },
          },
        },
      },
      scales: {
        x: {
          grid: { display: false },
          ticks: {
            color: textColor,
            autoSkip: false,
            callback: (_v, idx) =>
              [0, 6, 12, 18, 23].includes(idx) ? String(idx) : "",
          },
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor },
          ticks: {
            color: textColor,
            maxTicksLimit: 4,
            callback: v => formatDurationChart(Number(v), t),
          },
        },
      },
    }),
    [t, textColor, gridColor],
  );

  const weekdayData: ChartData<"bar"> = useMemo(
    () => ({
      labels: weekday.map(w => weekdayLabels[w.weekday]),
      datasets: [
        {
          data: weekday.map(w => w.duration),
          backgroundColor: weekday.map(w =>
            w.weekday === peakWeekday && w.duration > 0 ? barFillPeak : barFill,
          ),
          borderRadius: 2,
          barThickness: 14,
        },
      ],
    }),
    [weekday, weekdayLabels, peakWeekday, barFill, barFillPeak],
  );

  const weekdayOptions = useMemo<ChartOptions<"bar">>(
    () => ({
      indexAxis: "y" as const,
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx: TooltipItem<"bar">) => {
              const v = Number(ctx.parsed.x || 0);
              return v > 0 ? formatDuration(v, t) : t("stats.heatmap.noPlay");
            },
          },
        },
      },
      scales: {
        x: {
          beginAtZero: true,
          grid: { color: gridColor },
          ticks: {
            color: textColor,
            maxTicksLimit: 4,
            callback: v => formatDurationChart(Number(v), t),
          },
        },
        y: {
          grid: { display: false },
          ticks: { color: textColor },
        },
      },
    }),
    [t, textColor, gridColor],
  );

  if (!totalHour) {
    return (
      <div
        className={`flex items-center justify-center text-sm text-brand-500 dark:text-brand-400 ${className}`}
      >
        {t("stats.heatmap.empty")}
      </div>
    );
  }

  return (
    <div className={`flex flex-col gap-4 ${className}`}>
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-brand-500 dark:text-brand-400">
            {t("stats.timeOfDay.hourLabel")}
          </span>
          {peakHour >= 0 && (
            <span className="text-xs text-brand-600 dark:text-brand-300">
              {t("stats.timeOfDay.peakHour", {
                hour: `${String(peakHour).padStart(2, "0")}:00`,
                duration: formatDuration(hourMax, t),
              })}
            </span>
          )}
        </div>
        <div className="h-28">
          <Bar data={hourlyData} options={hourlyOptions} />
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-brand-500 dark:text-brand-400">
            {t("stats.timeOfDay.weekdayLabel")}
          </span>
          {peakWeekday >= 0 && (
            <span className="text-xs text-brand-600 dark:text-brand-300">
              {t("stats.timeOfDay.peakWeekday", {
                day: weekdayLabels[peakWeekday],
                duration: formatDuration(weekdayMax, t),
              })}
            </span>
          )}
        </div>
        <div className="h-44">
          <Bar data={weekdayData} options={weekdayOptions} />
        </div>
      </div>
    </div>
  );
}

export default HourWeekDistribution;
