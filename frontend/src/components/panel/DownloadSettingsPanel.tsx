import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../wailsjs/go/service/ConfigService";
import {
  checkSystemNotificationAvailability,
  sendSystemNotification,
} from "../../utils/systemNotification";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterButton } from "../ui/better/BetterButton";

interface GameLibrarySettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

type NotificationAvailability = "unknown" | "available" | "unavailable";

const notificationAvailabilityIcons: Record<NotificationAvailability, string>
  = {
    unknown: "i-mdi-help-circle-outline",
    available:
      "i-mdi-check-circle-outline text-success-600 dark:text-success-400",
    unavailable:
      "i-mdi-alert-circle-outline text-error-600 dark:text-error-400",
  };

export function DownloadSettingsPanel({
  formData,
  onChange,
}: GameLibrarySettingsPanelProps) {
  const { t } = useTranslation();
  const [isTestingNotification, setIsTestingNotification] = useState(false);
  const [isCheckingNotification, setIsCheckingNotification] = useState(false);
  const [notificationAvailability, setNotificationAvailability]
    = useState<NotificationAvailability>("unknown");

  const handleSelectGameLibraryPath = async () => {
    try {
      const path = await SelectDirectory(
        t("settings.download.selectGameLibraryTitle", "选择游戏库目录"),
      );
      if (path) {
        onChange({ ...formData, game_library_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select game library path:", error);
      toast.error(t("settings.download.toast.selectFailed", "选择目录失败"));
    }
  };

  const handleClearGameLibraryPath = () => {
    onChange({ ...formData, game_library_path: "" } as appconf.AppConfig);
  };

  const handleTestSystemNotification = async () => {
    setIsTestingNotification(true);
    try {
      const sent = await sendSystemNotification({
        id: `lunabox-notification-test-${Date.now()}`,
        title: "LunaBox",
        body: t(
          "settings.download.notificationTest.body",
          "系统通知工作正常。下载任务完成或失败时，LunaBox 会在这里提醒你。",
        ),
        data: { type: "notification-test" },
      });

      if (sent) {
        toast.success(
          t(
            "settings.download.notificationTest.toastSuccess",
            "测试通知已发送",
          ),
        );
      }
      else {
        toast.error(
          t(
            "settings.download.notificationTest.toastFailed",
            "系统通知不可用或发送失败",
          ),
        );
      }
    }
    finally {
      setIsTestingNotification(false);
    }
  };

  const handleCheckSystemNotification = async () => {
    setIsCheckingNotification(true);
    try {
      const available = await checkSystemNotificationAvailability();
      setNotificationAvailability(available ? "available" : "unavailable");

      if (available) {
        toast.success(
          t("settings.download.notificationTest.available", "当前系统支持通知"),
        );
      }
      else {
        toast.error(
          t(
            "settings.download.notificationTest.unavailable",
            "当前系统不支持通知或检测失败",
          ),
        );
      }
    }
    finally {
      setIsCheckingNotification(false);
    }
  };

  const notificationAvailabilityLabel = {
    unknown: t(
      "settings.download.notificationTest.checkAvailability",
      "检测系统通知是否可用",
    ),
    available: t(
      "settings.download.notificationTest.available",
      "当前系统支持通知",
    ),
    unavailable: t(
      "settings.download.notificationTest.unavailable",
      "当前系统不支持通知或检测失败",
    ),
  }[notificationAvailability];

  return (
    <div className="space-y-4">
      {/* 游戏库目录 */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.download.gameLibraryPath", "游戏库目录")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            "settings.download.gameLibraryPathHint",
            "下载的游戏将解压到此目录。留空则使用 ~/Games",
          )}
        </p>
        <BetterActionInput
          value={formData.game_library_path || ""}
          onChange={e =>
            onChange({
              ...formData,
              game_library_path: e.target.value,
            } as appconf.AppConfig)}
          placeholder={t(
            "settings.download.gameLibraryPathPlaceholder",
            "例如 D:\\Games 或 /home/user/games",
          )}
          className="text-sm"
          containerClassName="shadow-sm"
          actions={[
            {
              ariaLabel: t(
                "settings.download.selectGameLibraryTitle",
                "选择游戏库目录",
              ),
              icon: "i-mdi-folder-open-outline",
              onClick: handleSelectGameLibraryPath,
            },
            ...(formData.game_library_path
              ? [
                  {
                    ariaLabel: t(
                      "settings.download.clearGameLibraryPath",
                      "清除游戏库目录",
                    ),
                    icon: "i-mdi-close",
                    onClick: handleClearGameLibraryPath,
                  },
                ]
              : []),
          ]}
        />

        {/* 当前生效路径提示 */}
        <p className="flex items-center gap-1 text-xs text-brand-400 dark:text-brand-500">
          <span className="i-mdi-information-outline" />
          {formData.game_library_path
            ? t("settings.download.effectivePath", "游戏库路径：{{path}}", {
                path: formData.game_library_path,
              })
            : t("settings.download.defaultPath", "游戏库路径：~/Games（默认）")}
        </p>
      </div>

      <div className="flex items-center justify-between gap-4 pt-2">
        <div className="flex-1 space-y-2">
          <span className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.download.notificationTest.label", "系统通知")}
          </span>
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t(
              "settings.download.notificationTest.hint",
              "发送一条测试通知，确认下载完成或失败时可以收到系统提醒。",
            )}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <BetterButton
            variant="secondary"
            size="sm"
            icon={notificationAvailabilityIcons[notificationAvailability]}
            isLoading={isCheckingNotification}
            disabled={isTestingNotification}
            aria-label={notificationAvailabilityLabel}
            onClick={handleCheckSystemNotification}
          />
          <BetterButton
            variant="secondary"
            size="sm"
            icon="i-mdi-bell-ring-outline"
            isLoading={isTestingNotification}
            disabled={isCheckingNotification}
            onClick={handleTestSystemNotification}
          >
            {isTestingNotification
              ? t("settings.download.notificationTest.sending", "发送中...")
              : t("settings.download.notificationTest.button", "发送测试通知")}
          </BetterButton>
        </div>
      </div>
    </div>
  );
}
