import { createRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums, vo } from "../../wailsjs/go/models";
import { AISummarize } from "../../wailsjs/go/service/AiService";
import { GetGlobalPeriodStats } from "../../wailsjs/go/service/StatsService";
import { AiSummaryCard } from "../components/card/AiSummaryCard";
import { DurationLineChart } from "../components/chart/DurationLineChart";
import { HourWeekDistribution } from "../components/chart/HourWeekDistribution";
import { PlayHeatmap } from "../components/chart/PlayHeatmap";
import { TagDistributionChart } from "../components/chart/TagDistributionChart";
import { TemplateExportModal } from "../components/modal/TemplateExportModal";
import { StatsSkeleton } from "../components/skeleton/StatsSkeleton";
import { ProxyImage } from "../components/ui/ProxyImage";
import { SlideButton } from "../components/ui/SlideButton";
import { useAppStore } from "../store";
import {
  formatDateToYYYYMMDD,
  formatDuration,
  formatDurationCompact,
} from "../utils/time";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stats",
  component: StatsPage,
});

function StatsPage() {
  const { t } = useTranslation();
  const ref = useRef<HTMLDivElement>(null);
  const [dimension, setDimension] = useState<enums.Period>(enums.Period.WEEK);
  const [stats, setStats] = useState<vo.PeriodStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [webSearchUsed, setWebSearchUsed] = useState(false);
  const [showTemplateModal, setShowTemplateModal] = useState(false);

  // Custom date range
  const [customDateRange, setCustomDateRange] = useState(false);
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  // Delay skeleton display to avoid flash
  useEffect(() => {
    let timer: number;
    if (loading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [loading]);

  // Get cached AI summary from store
  const aiSummaryCache = useAppStore(state => state.aiSummaryCache);
  const setAISummary = useAppStore(state => state.setAISummary);
  const aiSummary = aiSummaryCache[dimension] || "";

  const handleAISummarize = useCallback(async () => {
    setAiLoading(true);
    setWebSearchUsed(false);
    setAISummary(dimension, "");
    try {
      const result = await AISummarize({ dimension, spoiler_level: "" });
      setAISummary(dimension, result.summary);
      setWebSearchUsed(result.web_search_used ?? false);
    }
    catch (err) {
      console.error("AI summarize failed:", err);
      setAISummary(dimension, "");
      toast.error(t("stats.ai.summarizeFailed"));
    }
    finally {
      setAiLoading(false);
    }
  }, [dimension, setAISummary, t]);

  const loadStats = async (dim: enums.Period, start?: string, end?: string) => {
    setLoading(true);
    try {
      const req = new vo.PeriodStatsRequest({
        dimension: dim,
        start_date: start || "",
        end_date: end || "",
      });
      const data = await GetGlobalPeriodStats(req);
      setStats(data);
    }
    catch (error) {
      console.error("Failed to load stats:", error);
      toast.error(t("stats.toast.loadStatsFailed"));
    }
    finally {
      setLoading(false);
    }
  };

  const handleApplyDateRange = () => {
    if (!startDate || !endDate) {
      toast.error(t("stats.toast.selectDateRange"));
      return;
    }
    if (new Date(startDate) >= new Date(endDate)) {
      toast.error(t("stats.toast.startBeforeEnd"));
      return;
    }
    loadStats(enums.Period.WEEK, startDate, endDate);
  };

  const handleResetDateRange = () => {
    setCustomDateRange(false);
    setStartDate("");
    setEndDate("");
    loadStats(dimension);
  };

  useEffect(() => {
    if (!customDateRange) {
      loadStats(dimension);
    }
  }, [dimension]);

  // When switching to custom date range, initialize dates to today
  useEffect(() => {
    if (customDateRange && !startDate && !endDate) {
      const today = formatDateToYYYYMMDD(new Date());
      setStartDate(today);
      setEndDate(today);
    }
  }, [customDateRange]);

  if (loading && !stats) {
    if (!showSkeleton) {
      return null;
    }
    return <StatsSkeleton />;
  }

  if (!stats) {
    return null;
  }

  // Chart 1: Total Play Duration Trend
  const timelineLabels = stats.timeline.map(p => p.label);
  const totalTrendDurations = stats.timeline.map(p => p.duration);
  const hasTotalTrendPlayData = totalTrendDurations.some(
    duration => duration > 0,
  );

  const totalTrendData = {
    labels: timelineLabels,
    datasets: [
      {
        label: t("stats.totalDurationDataset"),
        data: totalTrendDurations,
        borderColor: "rgb(75, 192, 192)",
        backgroundColor: "rgba(75, 192, 192, 0.5)",
        tension: 0.3,
      },
    ],
  };

  // Chart 2: Game Play Duration Trend (Multi-line)
  const gameTrendDurations = stats.leaderboard_series.flatMap(series =>
    series.points.map(p => p.duration),
  );
  const hasGameTrendPlayData = gameTrendDurations.some(
    duration => duration > 0,
  );

  const gameTrendData = {
    labels: timelineLabels,
    datasets: stats.leaderboard_series.map((series, index) => {
      const colors = [
        "rgb(255, 99, 132)",
        "rgb(54, 162, 235)",
        "rgb(255, 206, 86)",
        "rgb(75, 192, 192)",
        "rgb(153, 102, 255)",
        "rgb(255, 159, 64)",
        "rgb(99, 255, 132)",
        "rgb(199, 99, 255)",
        "rgb(64, 224, 208)",
        "rgb(255, 105, 180)",
      ];
      const color = colors[index % colors.length];
      return {
        label: series.game_name,
        data: series.points.map(p => p.duration),
        borderColor: color,
        backgroundColor: color.replace("rgb", "rgba").replace(")", ", 0.5)"),
        tension: 0.3,
      };
    }),
  };

  const summaryItems = [
    {
      value: stats.total_play_count,
      label: t("stats.summary.totalPlayCount"),
    },
    {
      value: formatDurationCompact(stats.total_play_duration, t),
      label: t("stats.summary.totalPlayDuration"),
      valueClassName: "whitespace-nowrap",
    },
    {
      value: stats.total_games_count,
      label: t("stats.summary.gamesPlayed"),
    },
    {
      value: stats.completed_games_count,
      label: t("stats.summary.completedGames"),
    },
    {
      value: formatDurationCompact(stats.avg_daily_duration, t),
      label: t("stats.summary.avgDailyDuration"),
      valueClassName: "whitespace-nowrap",
    },
    {
      value: formatDurationCompact(stats.avg_session_duration, t),
      label: t("stats.summary.avgSessionDuration"),
      valueClassName: "whitespace-nowrap",
    },
    {
      value: stats.max_streak,
      label: t("stats.summary.maxStreak"),
      suffix: t("stats.summary.dayUnit"),
    },
    {
      value: stats.new_games_count,
      label: t("stats.summary.newGames"),
    },
  ];

  const hasLeaderboard = stats.play_time_leaderboard.length > 0;
  const hasTagDistribution = (stats.tag_distribution?.length ?? 0) > 0;
  const showHeatmap
    = dimension === enums.Period.YEAR
      && !customDateRange
      && (stats.heatmap?.length ?? 0) > 0;

  return (
    <div
      id="stats-container"
      ref={ref}
      className={`space-y-6 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${loading ? "opacity-50 pointer-events-none" : "opacity-100"}`}
    >
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">
          {t("stats.title")}
        </h1>
      </div>
      <div className="flex justify-between items-center no-export">
        <div className="flex items-center space-x-4">
          <SlideButton
            options={[
              { label: t("stats.period.week"), value: enums.Period.WEEK },
              { label: t("stats.period.month"), value: enums.Period.MONTH },
              { label: t("stats.period.year"), value: enums.Period.YEAR },
            ]}
            value={customDateRange ? ("" as enums.Period) : dimension}
            onChange={(value) => {
              setDimension(value);
              if (customDateRange) {
                setCustomDateRange(false);
                setStartDate("");
                setEndDate("");
              }
            }}
            disabled={loading}
          />

          {/* Custom Date Range Toggle */}
          <button
            type="button"
            onClick={() => setCustomDateRange(!customDateRange)}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1 ${
              customDateRange
                ? "bg-neutral-100 dark:bg-neutral-900 text-neutral-600 dark:text-neutral-400"
                : "text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200"
            }`}
          >
            <span className="i-mdi-calendar-range text-lg" />
            {t("stats.customRange")}
          </button>
        </div>
        <div className="flex space-x-2 items-center">
          <button
            type="button"
            onClick={() => setShowTemplateModal(true)}
            className="flex justify-end i-mdi-image-filter-hdr text-2xl text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200 transition-colors"
            title={t("stats.exportTitle")}
          />
          <button
            type="button"
            onClick={handleAISummarize}
            className="flex justify-end i-mdi-robot-happy text-2xl text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200 transition-colors"
            title={t("stats.aiSummarizeTitle")}
          />
        </div>
      </div>

      {/* Custom Date Range Picker */}
      {customDateRange && (
        <div className="glass-panel flex items-center gap-4 p-4 bg-white dark:bg-brand-800 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 no-export">
          <div className="flex items-center gap-2">
            <label className="text-sm text-brand-600 dark:text-brand-400">
              {t("stats.startDate")}
            </label>
            <input
              type="date"
              value={startDate}
              onChange={e => setStartDate(e.target.value)}
              className="px-3 py-1.5 rounded-md border border-brand-300 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white text-sm"
            />
          </div>
          <div className="flex items-center gap-2">
            <label className="text-sm text-brand-600 dark:text-brand-400">
              {t("stats.endDate")}
            </label>
            <input
              type="date"
              value={endDate}
              onChange={e => setEndDate(e.target.value)}
              className="px-3 py-1.5 rounded-md border border-brand-300 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white text-sm"
            />
          </div>
          <button
            type="button"
            onClick={handleApplyDateRange}
            className="px-4 py-1.5 bg-neutral-600 hover:bg-neutral-700 text-white rounded-md text-sm font-medium transition-colors"
          >
            {t("stats.applyBtn")}
          </button>
          <button
            type="button"
            onClick={handleResetDateRange}
            className="px-4 py-1.5 bg-brand-200 dark:bg-brand-700 hover:bg-brand-300 dark:hover:bg-brand-600 text-brand-700 dark:text-brand-300 rounded-md text-sm font-medium transition-colors"
          >
            {t("stats.resetBtn")}
          </button>
        </div>
      )}

      {/* AI Summary Card */}
      {(aiLoading || aiSummary) && (
        <AiSummaryCard
          aiSummary={aiSummary}
          aiLoading={aiLoading}
          webSearchUsed={webSearchUsed}
        />
      )}

      {/* Summary Cards - compact 4/8 cols grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-3">
        {summaryItems.map(item => (
          <div
            key={item.label}
            className="glass-card bg-white dark:bg-brand-800 px-4 py-3 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700"
          >
            <h3 className="text-xs font-medium text-brand-500 dark:text-brand-400 mb-1 truncate">
              {item.label}
            </h3>
            <p
              className={`text-xl font-bold text-brand-900 dark:text-white ${item.valueClassName ?? ""}`}
            >
              {item.value}
              {item.suffix && (
                <span className="text-xs font-normal text-brand-500 dark:text-brand-400 ml-1">
                  {item.suffix}
                </span>
              )}
            </p>
          </div>
        ))}
      </div>

      {/* Year Heatmap (year dimension only, custom range excluded) */}
      {showHeatmap && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">
            {t("stats.heatmap.title")}
          </h3>
          <PlayHeatmap cells={stats.heatmap} />
        </div>
      )}

      {/* Row: Leaderboard + Tag Distribution (lg:2-col) */}
      <div className="grid grid-cols-1 xl:grid-cols-12 gap-6">
        {/* Leaderboard - xl:col-span-7 */}
        <div className="xl:col-span-7 glass-card bg-white dark:bg-brand-800 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 overflow-hidden flex flex-col">
          <div className="px-5 py-3 border-b border-brand-200 dark:border-brand-700 flex items-center justify-between">
            <h3 className="text-base font-semibold text-brand-900 dark:text-white">
              {t("stats.leaderboard.fullTitle")}
            </h3>
            {hasLeaderboard && (
              <span className="text-xs text-brand-500 dark:text-brand-400">
                {t("stats.leaderboard.countHint", {
                  count: stats.play_time_leaderboard.length,
                })}
              </span>
            )}
          </div>

          {!hasLeaderboard ? (
            <div className="px-6 py-12 text-center text-brand-500 dark:text-brand-400">
              {t("stats.leaderboard.noData")}
            </div>
          ) : (
            <div className="p-4 space-y-3 flex-1">
              {/* #1 hero - compact horizontal */}
              <div className="relative flex items-center gap-3 p-3 rounded-lg bg-gradient-to-r from-yellow-50/60 via-orange-50 to-transparent dark:from-yellow-900/10 dark:via-orange-900/20 dark:to-transparent border border-yellow-200/60 dark:border-yellow-800/40 overflow-hidden">
                <div className="w-7 h-7 bg-yellow-100 dark:bg-yellow-900/40 text-yellow-600 dark:text-yellow-400 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0">
                  #1
                </div>
                <ProxyImage
                  src={stats.play_time_leaderboard[0].cover_url}
                  alt={stats.play_time_leaderboard[0].game_name}
                  className="w-10 h-14 object-cover rounded shadow-md flex-shrink-0 bg-brand-200 dark:bg-brand-700"
                />
                <div className="flex-1 min-w-0">
                  <h4 className="text-sm font-bold text-brand-900 dark:text-white line-clamp-1">
                    {stats.play_time_leaderboard[0].game_name}
                  </h4>
                  <p className="text-base font-mono font-semibold text-neutral-700 dark:text-neutral-300 mt-0.5">
                    {formatDuration(
                      stats.play_time_leaderboard[0].total_duration,
                      t,
                    )}
                  </p>
                </div>
              </div>

              {/* #2 - #10: two-column grid, denser rows */}
              {stats.play_time_leaderboard.length > 1 && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                  {stats.play_time_leaderboard.slice(1).map((game, index) => (
                    <div
                      key={game.game_id}
                      className="flex items-center gap-2.5 p-2 rounded-lg bg-brand-50 dark:bg-brand-700/40 hover:bg-brand-100 dark:hover:bg-brand-700/60 transition-colors data-glass:bg-white/5 data-glass:dark:bg-black/5"
                    >
                      <span className="w-6 text-xs text-brand-500 dark:text-brand-400 font-medium tabular-nums text-center flex-shrink-0">
                        #
                        {index + 2}
                      </span>
                      <ProxyImage
                        src={game.cover_url}
                        alt={game.game_name}
                        className="w-7 h-10 object-cover rounded shadow-sm flex-shrink-0 bg-brand-200 dark:bg-brand-700"
                      />
                      <div className="flex-1 min-w-0">
                        <p className="text-xs font-medium text-brand-900 dark:text-white line-clamp-1">
                          {game.game_name}
                        </p>
                        <p className="text-[11px] font-mono text-brand-600 dark:text-brand-300 mt-0.5">
                          {formatDuration(game.total_duration, t)}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Tag Distribution - xl:col-span-5 */}
        {hasTagDistribution && (
          <div className="xl:col-span-5 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 flex flex-col">
            <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
              {t("stats.tagDistribution.title")}
            </h3>
            <div className="flex-1 min-h-0">
              <TagDistributionChart tags={stats.tag_distribution} />
            </div>
          </div>
        )}
      </div>

      {/* Row: Time-of-day distribution + Total trend (lg:2-col) */}
      <div className="grid grid-cols-1 xl:grid-cols-12 gap-6">
        <div className="xl:col-span-5 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
            {t("stats.timeOfDay.title")}
          </h3>
          <HourWeekDistribution
            hourly={stats.hourly_distribution}
            weekday={stats.weekday_distribution}
          />
        </div>
        <div className="xl:col-span-7 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 flex flex-col">
          <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
            {t("stats.charts.totalTrend")}
          </h3>
          <div className="flex-1 min-h-[18rem]">
            <DurationLineChart
              data={totalTrendData}
              hasPlayData={hasTotalTrendPlayData}
              yAxisTitle={t("stats.chartYAxis")}
              className="h-full"
            />
          </div>
        </div>
      </div>

      {/* Game Trend - full width */}
      <div className="glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
        <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
          {t("stats.charts.gameTrend")}
        </h3>
        <DurationLineChart
          data={gameTrendData}
          hasPlayData={hasGameTrendPlayData}
          yAxisTitle={t("stats.chartYAxis")}
          className="h-80"
        />
      </div>

      {/* Template Export Modal */}
      <TemplateExportModal
        isOpen={showTemplateModal}
        onClose={() => setShowTemplateModal(false)}
        stats={stats}
        aiSummary={aiSummary}
      />
    </div>
  );
}
