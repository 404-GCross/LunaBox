import {
  useDesktopWindow,
  WindowDragRegion,
  WindowNoDragRegion,
} from "@lunabox/desktop-shell-react";
import { useRef } from "react";
import topbarTitleDarkUrl from "../../assets/branding/topbar-title-dark.png";
import topbarTitleUrl from "../../assets/branding/topbar-title.png";

export const TOPBAR_HEIGHT = 28;
const WINDOW_STATE_DRAG_SYNC_DELAYS_MS = [80, 300] as const;

export function TopBar() {
  const {
    close,
    isMaximized,
    minimize,
    platform,
    refreshWindowState,
    toggleMaximize,
  } = useDesktopWindow();
  const isMaximizedRef = useRef(isMaximized);
  isMaximizedRef.current = isMaximized;

  const isMac = platform === "macos";
  const showWindowControls = platform !== null && !isMac;

  function scheduleForcedWindowStateSync() {
    for (const delay of WINDOW_STATE_DRAG_SYNC_DELAYS_MS) {
      window.setTimeout(() => {
        void refreshWindowState(true);
      }, delay);
    }
  }

  const handleTopBarMouseDown = (event: React.MouseEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }

    if (event.detail === 1 && isMaximizedRef.current) {
      window.addEventListener("mouseup", scheduleForcedWindowStateSync, {
        once: true,
      });
      return;
    }

    if (event.detail === 2) {
      event.preventDefault();
      void toggleMaximize();
    }
  };

  return (
    <WindowDragRegion
      onMouseDown={handleTopBarMouseDown}
      className="relative z-50 flex h-[28px] select-none items-center justify-center border-b border-brand-200/50 bg-brand-50 dark:border-brand-700/50 dark:bg-brand-800"
    >
      <img
        src={topbarTitleDarkUrl}
        className="h-[20px] absolute dark:hidden left-1/2 -translate-x-1/2 pointer-events-none"
        draggable="false"
        onDragStart={event => event.preventDefault()}
      />
      <img
        src={topbarTitleUrl}
        className="h-[20px] absolute hidden dark:block left-1/2 -translate-x-1/2 pointer-events-none"
        draggable="false"
        onDragStart={event => event.preventDefault()}
      />

      {showWindowControls && (
        <WindowNoDragRegion
          onDoubleClick={event => event.stopPropagation()}
          onMouseDown={event => event.stopPropagation()}
          className="ml-auto flex items-center"
        >
          <button
            type="button"
            aria-label="Minimize window"
            onClick={() => void minimize()}
            className="flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
          >
            <svg
              className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
              viewBox="0 0 12 12"
              fill="none"
              aria-hidden="true"
            >
              <path d="M0 6h12" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>

          <button
            type="button"
            aria-label={isMaximized ? "Restore window" : "Maximize window"}
            onClick={() => void toggleMaximize()}
            className="flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
          >
            {isMaximized ? (
              <svg
                className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
                viewBox="0 0 12 12"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M3 3h6v6H3V3z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                />
                <path d="M5 1h6v6" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            ) : (
              <svg
                className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
                viewBox="0 0 12 12"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M1 1h10v10H1V1z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                />
              </svg>
            )}
          </button>

          <button
            type="button"
            aria-label="Close window"
            onClick={() => void close()}
            className="group flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-red-500 active:scale-98"
          >
            <svg
              className="h-[10px] w-[10px] text-brand-600 transition-colors group-hover:text-white dark:text-brand-400"
              viewBox="0 0 12 12"
              fill="none"
              aria-hidden="true"
            >
              <path
                d="M1 1l10 10M11 1L1 11"
                stroke="currentColor"
                strokeWidth="1.5"
              />
            </svg>
          </button>
        </WindowNoDragRegion>
      )}
    </WindowDragRegion>
  );
}
