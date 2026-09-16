import type { ReactNode } from "react";
import { Transition, TransitionChild } from "@headlessui/react";
import { DesktopModal, useDesktopInsets } from "@lunabox/desktop-shell-react";
import { useId } from "react";

export type BetterDrawerPlacement = "bottom" | "right";

interface BetterDrawerProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: ReactNode;
  headerAction?: ReactNode;
  children: ReactNode;
  placement?: BetterDrawerPlacement;
  description?: ReactNode;
  footer?: ReactNode;
  closeLabel?: string;
  className?: string;
  bodyClassName?: string;
  topOffset?: number | string;
}

const PLACEMENT_CLASSES: Record<BetterDrawerPlacement, string> = {
  bottom:
    "max-h-[85dvh] w-full rounded-t-2xl border-t data-closed:translate-y-full",
  right: "h-full w-[min(92vw,26rem)] border-l data-closed:translate-x-full",
};

const WRAPPER_CLASSES: Record<BetterDrawerPlacement, string> = {
  bottom: "items-end",
  right: "justify-end",
};

/**
 * 带焦点约束与退出动效的边缘抽屉。
 *
 * 抽屉打开后，焦点会限制在面板内部；关闭后，焦点会回到触发控件。
 */
export function BetterDrawer({
  isOpen,
  onOpenChange,
  title,
  headerAction,
  children,
  placement = "right",
  description,
  footer,
  closeLabel = "Close",
  className = "",
  bodyClassName = "",
  topOffset,
}: BetterDrawerProps) {
  const insets = useDesktopInsets();
  const titleId = useId();
  const descriptionId = useId();
  // The content host already begins below the titlebar.
  const relativeTop
    = topOffset === undefined
      ? 0
      : `max(0px, calc(${typeof topOffset === "number" ? `${topOffset}px` : topOffset} - ${insets.top}px))`;

  return (
    <Transition show={isOpen} as="div" className="contents">
      <DesktopModal
        onClose={() => onOpenChange(false)}
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        className={`absolute inset-0 flex pointer-events-none ${WRAPPER_CLASSES[placement]}`}
        style={{ top: relativeTop }}
        backdrop={(
          <TransitionChild
            as="div"
            aria-hidden="true"
            className="absolute inset-0 pointer-events-auto bg-black/35 backdrop-blur-[2px] transition-opacity duration-300 ease-out data-closed:opacity-0 data-leave:duration-200 data-leave:ease-in motion-reduce:duration-0"
            style={{ top: relativeTop }}
          />
        )}
      >
        <TransitionChild
          as="div"
          className={`pointer-events-auto flex flex-col overflow-hidden border-brand-200 bg-white/96 backdrop-blur-20 transition-[transform,opacity] duration-300 ease-[cubic-bezier(.22,1,.36,1)] data-closed:opacity-95 data-leave:duration-200 data-leave:ease-in dark:border-brand-700 dark:bg-brand-800/96 motion-reduce:duration-0 ${PLACEMENT_CLASSES[placement]} ${className}`}
        >
          {placement === "bottom" && (
            <div
              className="flex h-5 shrink-0 items-center justify-center"
              aria-hidden="true"
            >
              <div className="h-1 w-10 rounded-full bg-brand-300 dark:bg-brand-600" />
            </div>
          )}

          <div
            className={`flex shrink-0 justify-between gap-4 border-b border-brand-200 px-5 py-4 dark:border-brand-700 ${description ? "items-start" : "items-center"}`}
          >
            <div className="min-w-0">
              <div className="flex min-h-8 items-center gap-1">
                <h2
                  id={titleId}
                  className="text-base font-semibold text-brand-900 dark:text-white"
                >
                  {title}
                </h2>
                {headerAction}
              </div>
              {description && (
                <p
                  id={descriptionId}
                  className="mt-1 text-xs leading-5 text-brand-500 dark:text-brand-400"
                >
                  {description}
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              aria-label={closeLabel}
              className="-mr-1 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
            >
              <div className="i-mdi-close text-xl" aria-hidden="true" />
            </button>
          </div>

          <div
            className={`min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 ${bodyClassName}`}
          >
            {children}
          </div>

          {footer && (
            <div className="shrink-0 border-t border-brand-200 p-4 dark:border-brand-700">
              {footer}
            </div>
          )}
        </TransitionChild>
      </DesktopModal>
    </Transition>
  );
}
