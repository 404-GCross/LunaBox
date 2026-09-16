import type { HTMLAttributes } from "react";
import { FocusTrap, FocusTrapFeatures, Portal } from "@headlessui/react";
import { mountModalScope } from "@lunabox/desktop-shell-core";
import { useLayoutEffect, useRef } from "react";
import { useDesktopShell } from "./DesktopShellContext.js";
import { LayerPortal } from "./LayerPortal.js";

export interface DesktopModalProps extends HTMLAttributes<HTMLDivElement> {
  onClose: () => void;
  host?: string;
  backdrop?: React.ReactNode;
}

/** A content-scoped dialog. Window chrome outside the host remains interactive. */
export function DesktopModal({
  children,
  backdrop,
  host = "content",
  onClose,
  ...props
}: DesktopModalProps) {
  const { hosts } = useDesktopShell();
  const target = hosts.get(host);
  const panel = useRef<HTMLDivElement>(null);
  const portalTarget = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  useLayoutEffect(() => {
    closeRef.current = onClose;
  }, [onClose]);
  useLayoutEffect(() => {
    if (!target || !panel.current)
      return;
    return mountModalScope({
      host: target,
      panel: panel.current,
      onDismiss: () => closeRef.current(),
    });
  }, [target]);

  return (
    <LayerPortal host={host} layer="modal" style={{ overflow: "clip" }}>
      {backdrop}
      <FocusTrap
        {...props}
        ref={panel}
        role="dialog"
        tabIndex={-1}
        initialFocusFallback={panel}
        features={
          FocusTrapFeatures.InitialFocus
          | FocusTrapFeatures.TabLock
          | FocusTrapFeatures.RestoreFocus
        }
      >
        <Portal.Group target={portalTarget}>{children}</Portal.Group>
        <div
          ref={portalTarget}
          style={{ display: "contents", pointerEvents: "auto" }}
        />
      </FocusTrap>
    </LayerPortal>
  );
}
