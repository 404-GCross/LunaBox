import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../wailsjs/go/service/ConfigService";
import { BetterActionInput } from "../ui/better/BetterActionInput";

interface GameLibrarySettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function DownloadSettingsPanel({
  formData,
  onChange,
}: GameLibrarySettingsPanelProps) {
  const { t } = useTranslation();

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
    </div>
  );
}
