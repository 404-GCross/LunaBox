import type { HTMLAttributes } from "react";
import { useDesktopShell } from "./DesktopShellContext";

export function WindowDragRegion({
  style,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  const { adapter } = useDesktopShell();
  return (
    <div
      {...adapter.dragRegionAttributes}
      {...props}
      style={{ ...adapter.dragRegionStyle, ...style }}
    />
  );
}

export function WindowNoDragRegion({
  style,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  const { adapter } = useDesktopShell();
  return (
    <div
      {...adapter.noDragRegionAttributes}
      {...props}
      style={{ ...adapter.noDragRegionStyle, ...style }}
    />
  );
}
