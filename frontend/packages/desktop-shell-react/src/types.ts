import type { CSSProperties } from "react";

export type DesktopPlatform = "linux" | "macos" | "unknown" | "windows";

export type DesktopInsets = {
  bottom: number;
  left: number;
  right: number;
  top: number;
};

export type DesktopWindowState = {
  isMaximized: boolean;
};

export type DesktopWindowAdapter = {
  close: () => Promise<void>;
  dragRegionAttributes?: Record<string, boolean | string>;
  dragRegionStyle?: CSSProperties;
  getPlatform: () => Promise<DesktopPlatform>;
  getWindowState: () => Promise<DesktopWindowState>;
  minimize: () => Promise<void>;
  noDragRegionAttributes?: Record<string, boolean | string>;
  noDragRegionStyle?: CSSProperties;
  startDragging?: () => Promise<void>;
  toggleMaximize: () => Promise<void>;
};

export type DesktopLayer
  = | "content"
    | "floating"
    | "dropdown"
    | "tooltip"
    | "toast"
    | "modal"
    | "critical";
