import type { RefObject } from "react";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { findScrollParent, getScrollTop, scrollToTop } from "../utils/scroll";

type UsePageScrollControlsOptions = {
  anchorRef: RefObject<HTMLElement>;
  enabled: boolean;
  toolbarRef: RefObject<HTMLElement>;
};

const SCROLL_TOP_BUTTON_THRESHOLD = 120;

export function usePageScrollControls({
  anchorRef,
  enabled,
  toolbarRef,
}: UsePageScrollControlsOptions) {
  const [showScrollTop, setShowScrollTop] = useState(false);
  const scrollElementRef = useRef<HTMLElement | null>(null);

  useLayoutEffect(() => {
    if (!enabled) {
      return;
    }

    const anchor = anchorRef.current ?? toolbarRef.current;
    const scrollElement = findScrollParent(anchor);
    scrollElementRef.current = scrollElement;

    return () => {
      scrollElementRef.current = null;
    };
  }, [anchorRef, enabled, toolbarRef]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const scrollElement = scrollElementRef.current;
    if (!scrollElement) {
      return;
    }

    let animationFrame = 0;
    const updateVisibility = () => {
      if (animationFrame) {
        return;
      }

      animationFrame = window.requestAnimationFrame(() => {
        animationFrame = 0;
        setShowScrollTop(
          getScrollTop(scrollElement) > SCROLL_TOP_BUTTON_THRESHOLD,
        );
      });
    };

    const restoreTimer = window.setTimeout(updateVisibility, 0);
    updateVisibility();
    scrollElement.addEventListener("scroll", updateVisibility, {
      passive: true,
    });

    return () => {
      window.clearTimeout(restoreTimer);
      if (animationFrame) {
        window.cancelAnimationFrame(animationFrame);
      }
      scrollElement.removeEventListener("scroll", updateVisibility);
    };
  }, [enabled]);

  const handleScrollToTop = useCallback(() => {
    scrollToTop(scrollElementRef.current);
  }, []);

  return {
    scrollToTop: handleScrollToTop,
    showScrollTop,
  };
}
