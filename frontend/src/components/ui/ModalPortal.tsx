import type { ReactNode } from "react";
import { LayerPortal } from "@lunabox/desktop-shell-react";

export const APP_MODAL_ROOT_ID = "app-modal-root";

interface ModalPortalProps {
  children: ReactNode;
}

export function ModalPortal({ children }: ModalPortalProps) {
  return (
    <LayerPortal host="content" layer="modal" pointerEvents="auto">
      <div className="absolute inset-0" data-glass="false">
        {children}
      </div>
    </LayerPortal>
  );
}
