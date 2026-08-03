import type { appconf, vo } from "../../../src/bindings/models";

import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import {
  disconnectHikarinagiAuthorization,
  fetchHikarinagiAuthStatus,
  fetchHikarinagiProfile,
  mergeHikarinagiAuthStatus,
  startHikarinagiAuthorization,
} from "../../utils/hikarinagiAuth";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface HikarinagiAccountSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
  onConfigRefresh: () => Promise<void>;
}

export function HikarinagiAccountSettings({
  formData,
  onChange,
  onConfigRefresh,
}: HikarinagiAccountSettingsProps) {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<vo.HikarinagiAuthStatus | null>(
    null,
  );
  const [profile, setProfile] = useState<vo.HikarinagiProfile | null>(null);
  const [isStatusLoading, setIsStatusLoading] = useState(false);
  const [isProfileLoading, setIsProfileLoading] = useState(false);
  const [isAuthorizing, setIsAuthorizing] = useState(false);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const [showDisconnectConfirm, setShowDisconnectConfirm] = useState(false);

  const auth = mergeHikarinagiAuthStatus(formData, snapshot);
  const displayName
    = profile?.nickname?.trim() || profile?.username?.trim() || auth.identity;
  const username = profile?.username?.trim() || "";
  const avatarURL = profile?.avatar_url?.trim() || auth.avatarUrl?.trim() || "";
  const isAuthorized = auth.state === "authorized";
  const shouldShowProfile = isAuthorized && Boolean(displayName);
  const avatarFallback = displayName.trim().charAt(0).toUpperCase() || "H";

  const refreshProfile = useCallback(
    async (status: vo.HikarinagiAuthStatus | null) => {
      const canLoadProfile
        = Boolean(status?.authorized) && !status?.needs_reauthorization;
      if (!canLoadProfile) {
        setProfile(null);
        return;
      }

      setIsProfileLoading(true);
      try {
        setProfile(await fetchHikarinagiProfile());
      }
      catch (error) {
        console.error("Failed to fetch Hikarinagi profile:", error);
        setProfile(null);
      }
      finally {
        setIsProfileLoading(false);
      }
    },
    [],
  );

  const refreshStatus = useCallback(async () => {
    setIsStatusLoading(true);
    try {
      const nextSnapshot = await fetchHikarinagiAuthStatus();
      setSnapshot(nextSnapshot);
      await refreshProfile(nextSnapshot);
    }
    catch (error) {
      console.error("Failed to fetch Hikarinagi auth status:", error);
      setSnapshot(null);
      setProfile(null);
    }
    finally {
      setIsStatusLoading(false);
    }
  }, [refreshProfile]);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  const handleAuthorize = async () => {
    setIsAuthorizing(true);
    try {
      await startHikarinagiAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.hikarinagiAuthSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.hikarinagiAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
      await refreshStatus();
    }
    finally {
      setIsAuthorizing(false);
    }
  };

  const handleDisconnect = async () => {
    setIsDisconnecting(true);
    try {
      await disconnectHikarinagiAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.hikarinagiDisconnectSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.hikarinagiAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsDisconnecting(false);
    }
  };

  return (
    <>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.hikarinagiSectionLabel")}
        </label>
        <div className="glass-panel flex flex-col gap-4 rounded-2xl border border-brand-200/80 bg-white/55 p-4 dark:border-brand-700/80 dark:bg-brand-900/25">
          <div className="flex items-center justify-between gap-4">
            <div className="min-w-0 flex flex-1 items-center gap-3">
              {shouldShowProfile ? (
                avatarURL ? (
                  <img
                    src={avatarURL}
                    alt=""
                    width={48}
                    height={48}
                    className="h-12 w-12 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                  />
                ) : (
                  <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80 dark:text-brand-200">
                    {avatarFallback}
                  </div>
                )
              ) : (
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-brand-200/80 bg-white/70 dark:border-brand-700/80 dark:bg-brand-800/70">
                  <img
                    src="/hikarinagi.png"
                    alt=""
                    width={28}
                    height={28}
                    className="h-7 w-7 object-contain opacity-90"
                  />
                </div>
              )}

              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                    {shouldShowProfile ? displayName : "Hikarinagi"}
                  </div>
                  {isStatusLoading || isProfileLoading ? (
                    <span
                      aria-hidden="true"
                      className="i-mdi-loading animate-spin text-brand-400"
                    />
                  ) : null}
                  {auth.state === "needs_reauth" ? (
                    <span className="rounded-full bg-warning-100 px-2 py-0.5 text-[11px] font-semibold text-warning-700 dark:bg-warning-900/30 dark:text-warning-300">
                      {t("settings.basic.hikarinagiAuthNeedsReauth")}
                    </span>
                  ) : null}
                </div>

                {shouldShowProfile ? (
                  <>
                    {username && username !== displayName ? (
                      <p className="truncate text-xs text-brand-500 dark:text-brand-400">
                        @
                        {username}
                      </p>
                    ) : null}
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("settings.basic.hikarinagiAuthAuthorized")}
                    </p>
                  </>
                ) : (
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {auth.state === "needs_reauth"
                      ? t("settings.basic.hikarinagiAuthReconnectHint")
                      : t("settings.basic.hikarinagiAuthHint")}
                  </p>
                )}

                {auth.lastError ? (
                  <p className="text-xs text-warning-700 dark:text-warning-300">
                    {t("settings.basic.hikarinagiAuthLastErrorLabel")}
                    {": "}
                    {auth.lastError}
                  </p>
                ) : null}
              </div>
            </div>

            {isAuthorized ? (
              <BetterButton
                variant="secondary"
                icon="i-mdi-link-off"
                isLoading={isDisconnecting}
                onClick={() => setShowDisconnectConfirm(true)}
              >
                {t("settings.basic.hikarinagiDisconnect")}
              </BetterButton>
            ) : (
              <BetterButton
                variant="primary"
                icon="i-mdi-account-key-outline"
                isLoading={isAuthorizing}
                onClick={handleAuthorize}
              >
                {auth.state === "needs_reauth"
                  ? t("settings.basic.hikarinagiReauthorize")
                  : t("settings.basic.hikarinagiAuthorize")}
              </BetterButton>
            )}
          </div>

          {isAuthorized ? (
            <>
              <div className="h-px w-full bg-brand-200/50 dark:bg-brand-700/50" />
              <div className="flex items-center justify-between gap-4">
                <div className="flex-1 space-y-1">
                  <div className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    {t("settings.basic.hikarinagiStatusPushLabel")}
                  </div>
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.basic.hikarinagiStatusPushHint")}
                  </p>
                </div>
                <BetterSwitch
                  id="hikarinagi_status_push_enabled"
                  checked={formData.hikarinagi_status_push_enabled ?? true}
                  onCheckedChange={checked =>
                    onChange({
                      ...formData,
                      hikarinagi_status_push_enabled: checked,
                    } as appconf.AppConfig)}
                />
              </div>
            </>
          ) : null}
        </div>
      </div>

      <ConfirmModal
        isOpen={showDisconnectConfirm}
        title={t("settings.basic.hikarinagiDisconnectConfirmTitle")}
        message={t("settings.basic.hikarinagiDisconnectConfirmMsg")}
        confirmText={t("settings.basic.hikarinagiDisconnect")}
        type="danger"
        onClose={() => setShowDisconnectConfirm(false)}
        onConfirm={() => {
          void handleDisconnect();
        }}
      />
    </>
  );
}
