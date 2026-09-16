import type { ReactNode } from "react";
import type { DesktopShellContextValue } from "./DesktopShellContext.js";
import type {
  DesktopInsets,
  DesktopPlatform,
  DesktopWindowAdapter,
} from "./types.js";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DesktopShellContext } from "./DesktopShellContext.js";
import {
  DESKTOP_INSET_VARIABLES,
  DESKTOP_LAYER_ORDER,
  DESKTOP_LAYER_VARIABLES,
} from "./layers.js";

const DEFAULT_INSETS: DesktopInsets = {
  bottom: 0,
  left: 0,
  right: 0,
  top: 0,
};

export interface DesktopShellProviderProps {
  adapter: DesktopWindowAdapter;
  children: ReactNode;
  insets?: Partial<DesktopInsets>;
}

const STATE_POLL_INTERVAL = 500;
const STATE_SETTLE_DELAY = 800;

export function DesktopShellProvider({
  adapter,
  children,
  insets: insetOverrides,
}: DesktopShellProviderProps) {
  const insetBottom = insetOverrides?.bottom ?? DEFAULT_INSETS.bottom;
  const insetLeft = insetOverrides?.left ?? DEFAULT_INSETS.left;
  const insetRight = insetOverrides?.right ?? DEFAULT_INSETS.right;
  const insetTop = insetOverrides?.top ?? DEFAULT_INSETS.top;
  const insets = useMemo(
    () => ({
      bottom: insetBottom,
      left: insetLeft,
      right: insetRight,
      top: insetTop,
    }),
    [insetBottom, insetLeft, insetRight, insetTop],
  );
  const [hosts, setHosts] = useState<ReadonlyMap<string, HTMLElement>>(
    () => new Map(),
  );
  const [platform, setPlatform] = useState<DesktopPlatform | null>(null);
  const [isMaximized, setIsMaximized] = useState(false);
  const isMaximizedRef = useRef(false);
  const pendingStateRef = useRef<{ until: number; value: boolean } | null>(
    null,
  );

  const updateMaximized = useCallback((value: boolean) => {
    isMaximizedRef.current = value;
    setIsMaximized(value);
  }, []);

  const refreshWindowState = useCallback(
    async (force = false) => {
      const state = await adapter.getWindowState();
      const pending = pendingStateRef.current;

      if (
        !force
        && pending
        && Date.now() < pending.until
        && state.isMaximized !== pending.value
      ) {
        return;
      }

      pendingStateRef.current = null;
      updateMaximized(state.isMaximized);
    },
    [adapter, updateMaximized],
  );

  const toggleMaximize = useCallback(async () => {
    const nextValue = !isMaximizedRef.current;
    pendingStateRef.current = {
      until: Date.now() + STATE_SETTLE_DELAY,
      value: nextValue,
    };
    updateMaximized(nextValue);

    try {
      await adapter.toggleMaximize();
    }
    catch (error) {
      pendingStateRef.current = null;
      await refreshWindowState(true).catch(() => undefined);
      throw error;
    }

    window.setTimeout(() => {
      void refreshWindowState(true).catch(() => undefined);
    }, STATE_SETTLE_DELAY);
  }, [adapter, refreshWindowState, updateMaximized]);

  const registerHost = useCallback(
    (name: string, element: HTMLElement | null) => {
      setHosts((current) => {
        const next = new Map(current);
        if (element) {
          next.set(name, element);
        }
        else {
          next.delete(name);
        }
        return next;
      });
    },
    [],
  );

  useEffect(() => {
    let active = true;
    adapter
      .getPlatform()
      .then((value) => {
        if (active) {
          setPlatform(value);
        }
      })
      .catch(() => {
        if (active) {
          setPlatform("unknown");
        }
      });
    void refreshWindowState().catch(() => undefined);

    return () => {
      active = false;
    };
  }, [adapter, refreshWindowState]);

  useEffect(() => {
    if (platform === "macos") {
      return;
    }

    const sync = () => {
      void refreshWindowState().catch(() => undefined);
    };
    window.addEventListener("resize", sync);
    const interval = window.setInterval(sync, STATE_POLL_INTERVAL);

    return () => {
      window.removeEventListener("resize", sync);
      window.clearInterval(interval);
    };
  }, [platform, refreshWindowState]);

  useEffect(() => {
    const root = document.documentElement;
    const properties: Array<[string, string]> = [
      [DESKTOP_INSET_VARIABLES.top, `${insets.top}px`],
      [DESKTOP_INSET_VARIABLES.right, `${insets.right}px`],
      [DESKTOP_INSET_VARIABLES.bottom, `${insets.bottom}px`],
      [DESKTOP_INSET_VARIABLES.left, `${insets.left}px`],
      ...Object.entries(DESKTOP_LAYER_VARIABLES).map(
        ([layer, property]) =>
          [
            property,
            String(
              DESKTOP_LAYER_ORDER[layer as keyof typeof DESKTOP_LAYER_ORDER],
            ),
          ] as [string, string],
      ),
    ];
    const previous = properties.map(
      ([property]) =>
        [property, root.style.getPropertyValue(property)] as const,
    );

    for (const [property, value] of properties) {
      root.style.setProperty(property, value);
    }

    return () => {
      for (const [property, value] of previous) {
        if (value) {
          root.style.setProperty(property, value);
        }
        else {
          root.style.removeProperty(property);
        }
      }
    };
  }, [insets]);

  const value = useMemo<DesktopShellContextValue>(
    () => ({
      adapter,
      close: adapter.close,
      hosts,
      insets,
      isMaximized,
      minimize: adapter.minimize,
      platform,
      refreshWindowState,
      registerHost,
      toggleMaximize,
    }),
    [
      adapter,
      hosts,
      insets,
      isMaximized,
      platform,
      refreshWindowState,
      registerHost,
      toggleMaximize,
    ],
  );

  return (
    <DesktopShellContext.Provider value={value}>
      {children}
    </DesktopShellContext.Provider>
  );
}
