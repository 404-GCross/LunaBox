import type { appconf, vo } from "../../src/bindings/models";

import {
  Disconnect,
  GetAuthStatus,
  GetProfile,
  StartAuth,
  SyncAllGameStatuses,
} from "../../bindings/lunabox/internal/service/kungalservice";

export const KUNGAL_STATUS_SYNC_PROGRESS_EVENT = "kungal:status-sync-progress";

export type KungalAuthViewState
  = | "unauthorized"
    | "authorized"
    | "needs_reauth";

export type KungalAuthStatus = {
  state: KungalAuthViewState;
  identity: string;
  avatarUrl?: string;
  expiresAt?: string;
  lastError?: string;
};

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function getIdentity(username?: string, userID?: string): string {
  return readString(username) || readString(userID) || "鲲 Galgame";
}

function deriveState(
  authorized: boolean,
  needsReauthorization: boolean,
): KungalAuthViewState {
  if (needsReauthorization) {
    return "needs_reauth";
  }
  if (authorized) {
    return "authorized";
  }
  return "unauthorized";
}

function getStatusFromConfig(config: appconf.AppConfig): KungalAuthStatus {
  const accessToken = readString(config.kungal_access_token);
  const refreshToken = readString(config.kungal_refresh_token);
  const username = readString(config.kungal_authorized_username);
  const userID = readString(config.kungal_authorized_user_id);
  const avatarUrl = readString(config.kungal_authorized_avatar_url);
  const expiresAt = readString(config.kungal_token_expires_at);
  const lastError = readString(config.kungal_auth_error);

  return {
    state: deriveState(
      Boolean(accessToken || refreshToken),
      Boolean(lastError),
    ),
    identity: getIdentity(username, userID),
    avatarUrl: avatarUrl || undefined,
    expiresAt: expiresAt || undefined,
    lastError: lastError || undefined,
  };
}

function getStatusFromSnapshot(
  snapshot: vo.KungalAuthStatus,
): KungalAuthStatus {
  return {
    state: deriveState(
      Boolean(snapshot.authorized),
      Boolean(snapshot.needs_reauthorization),
    ),
    identity: getIdentity(snapshot.username, snapshot.user_id),
    avatarUrl: readString(snapshot.avatar_url) || undefined,
    expiresAt: readString(snapshot.access_token_expires_at) || undefined,
    lastError: readString(snapshot.last_error) || undefined,
  };
}

export function mergeKungalAuthStatus(
  config: appconf.AppConfig,
  snapshot?: vo.KungalAuthStatus | null,
): KungalAuthStatus {
  if (snapshot) {
    return getStatusFromSnapshot(snapshot);
  }
  return getStatusFromConfig(config);
}

export function fetchKungalAuthStatus(): Promise<vo.KungalAuthStatus> {
  return GetAuthStatus();
}

export function fetchKungalProfile(): Promise<vo.KungalProfile> {
  return GetProfile();
}

export function startKungalAuthorization(): Promise<vo.KungalAuthStatus> {
  return StartAuth();
}

export function disconnectKungalAuthorization(): Promise<vo.KungalAuthStatus> {
  return Disconnect();
}

export function syncAllKungalGameStatuses(): Promise<vo.RemoteStatusSyncProgress> {
  return SyncAllGameStatuses();
}
