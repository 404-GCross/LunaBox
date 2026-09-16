import type { CSSProperties, ReactNode } from "react";
import type { DesktopLayer } from "./types.js";
import { createPortal } from "react-dom";
import { useDesktopShell } from "./DesktopShellContext.js";
import { DESKTOP_LAYER_ORDER } from "./layers.js";

export interface LayerPortalProps {
  children: ReactNode;
  className?: string;
  host?: string;
  layer: DesktopLayer;
  pointerEvents?: CSSProperties["pointerEvents"];
  style?: CSSProperties;
}

export function LayerPortal({
  children,
  className,
  host = "window",
  layer,
  pointerEvents = "none",
  style,
}: LayerPortalProps) {
  const { hosts } = useDesktopShell();
  const target = hosts.get(host);

  if (!target) {
    return null;
  }

  return createPortal(
    <div
      className={className}
      data-desktop-layer={layer}
      style={{
        inset: 0,
        pointerEvents,
        position: "absolute",
        zIndex: DESKTOP_LAYER_ORDER[layer],
        ...style,
      }}
    >
      {children}
    </div>,
    target,
  );
}
