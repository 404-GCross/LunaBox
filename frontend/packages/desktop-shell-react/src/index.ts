export {
  getDesktopLayerStyle,
  useDesktopInsets,
  useDesktopShell,
  useDesktopWindow,
} from "./DesktopShellContext";
export { DesktopShellProvider } from "./DesktopShellProvider";
export type { DesktopShellProviderProps } from "./DesktopShellProvider";
export { LayerPortal } from "./LayerPortal";
export type { LayerPortalProps } from "./LayerPortal";
export {
  DESKTOP_INSET_VARIABLES,
  DESKTOP_LAYER_ORDER,
  DESKTOP_LAYER_VARIABLES,
} from "./layers";
export { OverlayHost } from "./OverlayHost";
export type { OverlayHostProps } from "./OverlayHost";
export type {
  DesktopInsets,
  DesktopLayer,
  DesktopPlatform,
  DesktopWindowAdapter,
  DesktopWindowState,
} from "./types";
export { WindowDragRegion, WindowNoDragRegion } from "./WindowDragRegion";
