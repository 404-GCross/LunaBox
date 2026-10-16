import type { ChartData, ChartOptions } from "chart.js";
import type { CSSProperties } from "react";
import { useEffect, useRef, useState } from "react";
import { Line } from "react-chartjs-2";

interface HorizontalScrollChartProps {
  data: ChartData<"line">;
  options?: ChartOptions<"line">;
  className?: string; // Wrapper class for height, e.g. "h-80" or "h-96"
  minPointWidth?: number; // Minimum width per data point before horizontal scrolling starts
}

// Browsers refuse to render canvases beyond these limits (Chromium: 32767px per side, 268M px area),
// leaving the chart blank. Keep some headroom below the hard limits.
const MAX_CANVAS_SIDE = 32000;
const MAX_CANVAS_AREA = 256 * 1024 * 1024;

function getMaxChartWidth(containerHeight: number) {
  const dpr = Math.max(1, window.devicePixelRatio || 1);
  const bySide = MAX_CANVAS_SIDE / dpr;
  const byArea
    = containerHeight > 0
      ? MAX_CANVAS_AREA / (containerHeight * dpr * dpr)
      : bySide;
  return Math.floor(Math.min(bySide, byArea));
}

export function HorizontalScrollChart({
  data,
  options,
  className = "h-80",
  minPointWidth = 20,
}: HorizontalScrollChartProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const containerWidth = containerSize.width;
  const dataPointCount
    = data.labels?.length
      || Math.max(0, ...data.datasets.map(dataset => dataset.data.length));
  const minChartWidth
    = dataPointCount > 1
      ? Math.min(
          dataPointCount * minPointWidth,
          getMaxChartWidth(containerSize.height),
        )
      : 0;
  const chartWidth
    = containerWidth > 0 ? Math.max(containerWidth, minChartWidth) : undefined;
  const isScrollable = containerWidth > 0 && minChartWidth > containerWidth;

  useEffect(() => {
    const wrapper = wrapperRef.current;
    if (!wrapper)
      return;

    const updateContainerSize = () => {
      const { width, height } = wrapper.getBoundingClientRect();
      setContainerSize(prev =>
        prev.width === width && prev.height === height
          ? prev
          : { width, height },
      );
    };

    updateContainerSize();
    const resizeObserver = new ResizeObserver(updateContainerSize);
    resizeObserver.observe(wrapper);

    return () => resizeObserver.disconnect();
  }, []);

  // Ensure tooltips behave correctly (snap to nearest x-axis point)
  const defaultInteraction = {
    mode: "index" as const,
    intersect: false,
  };

  const mergedOptions: ChartOptions<"line"> = {
    ...options,
    maintainAspectRatio: false,
    responsive: true,
    interaction: options?.interaction || defaultInteraction,
  };
  const xScale = options?.scales?.x as { type?: string } | undefined;
  const chartKey = `${xScale?.type ?? "category"}-${
    data.labels ? "labels" : "points"
  }`;

  const containerStyle: CSSProperties = {
    height: "100%",
    minWidth: "100%",
    width: chartWidth ? `${chartWidth}px` : "100%",
  };

  return (
    <div ref={wrapperRef} className={`w-full ${className}`}>
      <div
        className={`h-full w-full overflow-y-hidden scrollbar-thin scrollbar-thumb-brand-200 dark:scrollbar-thumb-brand-700 scrollbar-track-transparent ${
          isScrollable ? "overflow-x-auto" : "overflow-x-hidden"
        }`}
      >
        <div className="relative" style={containerStyle}>
          <Line key={chartKey} options={mergedOptions} data={data} />
        </div>
      </div>
    </div>
  );
}
