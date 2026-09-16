import type { CSSProperties, HTMLAttributes } from "react";
import { useCallback } from "react";
import { useDesktopShell } from "./DesktopShellContext";

export type OverlayHostProps = HTMLAttributes<HTMLDivElement> & {
  name: string;
};

const DEFAULT_STYLE: CSSProperties = {
  inset: 0,
  pointerEvents: "none",
  position: "absolute",
};

export function OverlayHost({ name, style, ...props }: OverlayHostProps) {
  const { registerHost } = useDesktopShell();
  const setHost = useCallback(
    (element: HTMLDivElement | null) => {
      registerHost(name, element);
    },
    [name, registerHost],
  );

  return (
    <div
      {...props}
      ref={setHost}
      data-desktop-overlay-host={name}
      style={{ ...DEFAULT_STYLE, ...style }}
    />
  );
}
