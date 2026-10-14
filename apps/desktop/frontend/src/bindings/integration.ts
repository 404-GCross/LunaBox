import type { service } from "./models";

import * as GeneratedIntegrationService from "../../bindings/lunabox/internal/service/integrationservice";

export type SteamCompatibilityTool = {
  name: string;
  display_name: string;
  path: string;
  built_in: boolean;
};

export type SteamCompatibilityInfo = {
  supported: boolean;
  steam_installed: boolean;
  steam_root: string;
  app_id: string;
  proton_prefix: string;
  current_tool: string;
  default_tool: string;
  tools: SteamCompatibilityTool[];
};

export type LocalProtonTool = {
  id: string;
  name: string;
  display_name: string;
  path: string;
  proton_path: string;
  source: string;
  built_in: boolean;
};

export type GameCompatibilityToolsInfo = {
  supported: boolean;
  runner_kind: string;
  prefix_path: string;
  drive_c_path: string;
  app_id: string;
  winetricks_path: string;
  winetricks_source: string;
  winetricks_available: boolean;
  winetricks_error: string;
  protontricks_path: string;
  protontricks_source: string;
  protontricks_available: boolean;
  protontricks_error: string;
  actions: string[];
  message: string;
};

type IntegrationServiceCompat = typeof GeneratedIntegrationService & {
  GetLocalProtonTools?: () => Promise<LocalProtonTool[]>;
  GetGameCompatibilityTools?: (
    gameID: string,
  ) => Promise<GameCompatibilityToolsInfo>;
  GetGameSteamCompatibility?: (
    gameID: string,
  ) => Promise<SteamCompatibilityInfo>;
  OpenGameCompatibilityTool?: (
    gameID: string,
    action: string,
  ) => Promise<string>;
  SetGameSteamCompatibilityTool?: (
    gameID: string,
    toolName: string,
  ) => Promise<SteamCompatibilityInfo>;
  SetGameSteamLaunchOptions?: (
    gameID: string,
    launchOptions: string,
  ) => Promise<service.SteamLaunchStatus>;
  OpenGameSteamProtonPrefix?: (gameID: string) => Promise<string>;
  RestartSteamClient?: () => Promise<void>;
};

const integrationService
  = GeneratedIntegrationService as IntegrationServiceCompat;

function missingBinding<T>(method: string): Promise<T> {
  return Promise.reject(
    new Error(`${method} binding is not generated yet`),
  );
}

export function GetLocalProtonTools(): Promise<LocalProtonTool[]> {
  if (integrationService.GetLocalProtonTools) {
    return integrationService.GetLocalProtonTools();
  }
  return missingBinding<LocalProtonTool[]>("GetLocalProtonTools");
}

export function GetGameCompatibilityTools(
  gameID: string,
): Promise<GameCompatibilityToolsInfo> {
  if (integrationService.GetGameCompatibilityTools) {
    return integrationService.GetGameCompatibilityTools(gameID);
  }
  return missingBinding<GameCompatibilityToolsInfo>(
    "GetGameCompatibilityTools",
  );
}

export function GetGameSteamCompatibility(
  gameID: string,
): Promise<SteamCompatibilityInfo> {
  if (integrationService.GetGameSteamCompatibility) {
    return integrationService.GetGameSteamCompatibility(gameID);
  }
  return missingBinding<SteamCompatibilityInfo>("GetGameSteamCompatibility");
}

export function SetGameSteamCompatibilityTool(
  gameID: string,
  toolName: string,
): Promise<SteamCompatibilityInfo> {
  if (integrationService.SetGameSteamCompatibilityTool) {
    return integrationService.SetGameSteamCompatibilityTool(gameID, toolName);
  }
  return missingBinding<SteamCompatibilityInfo>("SetGameSteamCompatibilityTool");
}

export function SetGameSteamLaunchOptions(
  gameID: string,
  launchOptions: string,
): Promise<service.SteamLaunchStatus> {
  if (integrationService.SetGameSteamLaunchOptions) {
    return integrationService.SetGameSteamLaunchOptions(gameID, launchOptions);
  }
  return missingBinding<service.SteamLaunchStatus>("SetGameSteamLaunchOptions");
}

export function OpenGameSteamProtonPrefix(gameID: string): Promise<string> {
  if (integrationService.OpenGameSteamProtonPrefix) {
    return integrationService.OpenGameSteamProtonPrefix(gameID);
  }
  return missingBinding<string>("OpenGameSteamProtonPrefix");
}

export function OpenGameCompatibilityTool(
  gameID: string,
  action: string,
): Promise<string> {
  if (integrationService.OpenGameCompatibilityTool) {
    return integrationService.OpenGameCompatibilityTool(gameID, action);
  }
  return missingBinding<string>("OpenGameCompatibilityTool");
}

export function RestartSteamClient(): Promise<void> {
  if (integrationService.RestartSteamClient) {
    return integrationService.RestartSteamClient();
  }
  return missingBinding<void>("RestartSteamClient");
}
