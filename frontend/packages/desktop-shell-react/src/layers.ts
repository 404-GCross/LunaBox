import type { DesktopLayer } from "./types";

export const DESKTOP_LAYER_ORDER: Record<DesktopLayer, number> = {
  content: 0,
  modal: 400,
  floating: 500,
  dropdown: 600,
  tooltip: 700,
  toast: 800,
  critical: 900,
};

export const DESKTOP_INSET_VARIABLES = {
  bottom: "--desktop-inset-bottom",
  left: "--desktop-inset-left",
  right: "--desktop-inset-right",
  top: "--desktop-inset-top",
} as const;

export const DESKTOP_LAYER_VARIABLES: Record<DesktopLayer, string> = {
  content: "--desktop-layer-content",
  floating: "--desktop-layer-floating",
  dropdown: "--desktop-layer-dropdown",
  tooltip: "--desktop-layer-tooltip",
  toast: "--desktop-layer-toast",
  modal: "--desktop-layer-modal",
  critical: "--desktop-layer-critical",
};
