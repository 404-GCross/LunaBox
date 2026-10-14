import { useCallback, useEffect, useRef, useState } from "react";

type HorizontalRailScrollControlsOptions = {
  contentVersion?: unknown;
  enabled: boolean;
};

export function useHorizontalRailScrollControls({
  contentVersion,
  enabled,
}: HorizontalRailScrollControlsOptions) {
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const [canScrollPrev, setCanScrollPrev] = useState(false);
  const [canScrollNext, setCanScrollNext] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);

  const updateScrollState = useCallback(() => {
    const scroller = scrollerRef.current;
    if (!scroller) {
      setCanScrollPrev(false);
      setCanScrollNext(false);
      setHasOverflow(false);
      return;
    }

    const maxScrollLeft = scroller.scrollWidth - scroller.clientWidth;
    const nextHasOverflow = maxScrollLeft > 2;
    setHasOverflow(nextHasOverflow);
    setCanScrollPrev(scroller.scrollLeft > 2);
    setCanScrollNext(scroller.scrollLeft < maxScrollLeft - 2);
  }, []);

  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!enabled || !scroller) {
      setCanScrollPrev(false);
      setCanScrollNext(false);
      setHasOverflow(false);
      return;
    }

    updateScrollState();
    scroller.addEventListener("scroll", updateScrollState, {
      passive: true,
    });
    window.addEventListener("resize", updateScrollState);

    const resizeObserver = new ResizeObserver(updateScrollState);
    resizeObserver.observe(scroller);

    const frame = window.requestAnimationFrame(updateScrollState);

    return () => {
      window.cancelAnimationFrame(frame);
      scroller.removeEventListener("scroll", updateScrollState);
      window.removeEventListener("resize", updateScrollState);
      resizeObserver.disconnect();
    };
  }, [contentVersion, enabled, updateScrollState]);

  const scrollRail = useCallback((direction: -1 | 1) => {
    const scroller = scrollerRef.current;
    if (!scroller) {
      return;
    }

    scroller.scrollBy({
      behavior: "smooth",
      left: direction * Math.max(scroller.clientWidth - 112, 280),
    });
  }, []);

  return {
    canScrollNext,
    canScrollPrev,
    hasOverflow,
    scrollerRef,
    scrollRail,
  };
}
