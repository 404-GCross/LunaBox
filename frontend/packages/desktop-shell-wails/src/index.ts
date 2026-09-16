import type {
  DesktopPlatform,
  DesktopWindowAdapter,
} from "@lunabox/desktop-shell-core";

import { System, Window } from "@wailsio/runtime";

function normalizePlatform(platform: string): DesktopPlatform {
  if (platform === "darwin") {
    return "macos";
  }
  if (platform === "windows" || platform === "linux") {
    return platform;
  }
  return "unknown";
}

const dragRegionStyle = {
  "--wails-draggable": "drag",
};

const noDragRegionStyle = {
  "--wails-draggable": "no-drag",
};

export function createWailsDesktopAdapter(): DesktopWindowAdapter {
  return {
    close: () => Window.Close(),
    dragRegionStyle,
    getPlatform: async () => {
      const environment = await System.Environment();
      return normalizePlatform(environment.OS);
    },
    getWindowState: async () => ({
      isMaximized: await Window.IsMaximised(),
    }),
    minimize: () => Window.Minimise(),
    noDragRegionStyle,
    toggleMaximize: () => Window.ToggleMaximise(),
  };
}
