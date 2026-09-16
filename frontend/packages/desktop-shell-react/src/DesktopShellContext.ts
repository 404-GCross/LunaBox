import type { CSSProperties } from "react";

import { createContext, useContext } from "react";

import type {
  DesktopInsets,
  DesktopPlatform,
  DesktopWindowAdapter,
} from "./types.js";

import { DESKTOP_LAYER_ORDER } from "./layers.js";

export type DesktopShellContextValue = {
  adapter: DesktopWindowAdapter;
  close: () => Promise<void>;
  hosts: ReadonlyMap<string, HTMLElement>;
  insets: DesktopInsets;
  isMaximized: boolean;
  minimize: () => Promise<void>;
  platform: DesktopPlatform | null;
  refreshWindowState: (force?: boolean) => Promise<void>;
  registerHost: (name: string, element: HTMLElement | null) => void;
  toggleMaximize: () => Promise<void>;
};

export const DesktopShellContext
  = createContext<DesktopShellContextValue | null>(null);

export function useDesktopShell() {
  const context = useContext(DesktopShellContext);
  if (!context) {
    throw new Error("useDesktopShell must be used inside DesktopShellProvider");
  }
  return context;
}

export function useDesktopInsets() {
  return useDesktopShell().insets;
}

export function getDesktopLayerStyle(
  layer: keyof typeof DESKTOP_LAYER_ORDER,
): CSSProperties {
  return { zIndex: DESKTOP_LAYER_ORDER[layer] };
}

export function useDesktopWindow() {
  const {
    close,
    isMaximized,
    minimize,
    platform,
    refreshWindowState,
    toggleMaximize,
  } = useDesktopShell();

  return {
    close,
    isMaximized,
    minimize,
    platform,
    refreshWindowState,
    toggleMaximize,
  };
}
