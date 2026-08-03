import type { appconf } from "../../../src/bindings/models";
import { useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../bindings/lunabox/internal/service/configservice";
import { appZoomOptions, languageOptions } from "../../consts/options";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";
import { BangumiAccountSettings } from "./BangumiAccountSettings";
import { HikarinagiAccountSettings } from "./HikarinagiAccountSettings";

interface BetterSelectOption {
  value: string;
  label: string;
}

type AccountProvider = "bangumi" | "hikarinagi";

const ACCOUNT_CONTENT_FADE_MS = 140;
const ACCOUNT_CARD_RESIZE_MS = 280;
const ACCOUNT_CARD_RESIZE_BUFFER_MS = 32;

interface BasicSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
  onZoomChange: (zoomFactor: number) => void;
  onConfigRefresh: () => Promise<void>;
}

export function BasicSettingsPanel({
  formData,
  onChange,
  onZoomChange,
  onConfigRefresh,
}: BasicSettingsProps) {
  const { t } = useTranslation();
  const [expandedAccount, setExpandedAccount]
    = useState<AccountProvider | null>(null);
  const [isAccountContentVisible, setIsAccountContentVisible] = useState(true);
  const accountContentTimerRef = useRef<number | null>(null);

  const COMMON_TIMEZONES: BetterSelectOption[] = [
    { value: "Asia/Shanghai", label: "China Standard Time (UTC+8)" },
    { value: "Asia/Tokyo", label: "Japan Standard Time (UTC+9)" },
    { value: "Asia/Seoul", label: "Korea Standard Time (UTC+9)" },
    { value: "Asia/Hong_Kong", label: "Hong Kong Time (UTC+8)" },
    { value: "Asia/Taipei", label: "Taipei Time (UTC+8)" },
    { value: "Asia/Singapore", label: "Singapore Time (UTC+8)" },
    { value: "Asia/Bangkok", label: "Bangkok Time (UTC+7)" },
    { value: "Asia/Dubai", label: "Dubai Time (UTC+4)" },
    { value: "Europe/London", label: "London Time (UTC+0)" },
    { value: "Europe/Paris", label: "Paris Time (UTC+1)" },
    { value: "Europe/Berlin", label: "Berlin Time (UTC+1)" },
    { value: "Europe/Moscow", label: "Moscow Time (UTC+3)" },
    { value: "America/New_York", label: "New York Time (UTC-5)" },
    { value: "America/Chicago", label: "Chicago Time (UTC-6)" },
    { value: "America/Denver", label: "Denver Time (UTC-7)" },
    { value: "America/Los_Angeles", label: "Los Angeles Time (UTC-8)" },
    { value: "America/Sao_Paulo", label: "São Paulo Time (UTC-3)" },
    { value: "Australia/Sydney", label: "Sydney Time (UTC+10)" },
    { value: "Pacific/Auckland", label: "Auckland Time (UTC+12)" },
    { value: "UTC", label: "Coordinated Universal Time (UTC)" },
  ];

  const accountGridColumns
    = expandedAccount === "bangumi"
      ? "sm:grid-cols-[minmax(0,7fr)_minmax(0,3fr)]"
      : expandedAccount === "hikarinagi"
        ? "sm:grid-cols-[minmax(0,3fr)_minmax(0,7fr)]"
        : "sm:grid-cols-2";

  useEffect(() => {
    return () => {
      if (accountContentTimerRef.current !== null) {
        window.clearTimeout(accountContentTimerRef.current);
      }
    };
  }, []);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value, type } = e.target;
    const newValue
      = type === "checkbox" ? (e.target as HTMLInputElement).checked : value;
    onChange({ ...formData, [name]: newValue } as appconf.AppConfig);
  };

  const handleSelectGameLibraryPath = async () => {
    try {
      const path = await SelectDirectory(
        t("settings.basic.selectGameLibraryTitle"),
      );
      if (path) {
        onChange({ ...formData, game_library_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select game library path:", error);
      toast.error(t("settings.basic.selectGameLibraryFailed"));
    }
  };

  const handleClearGameLibraryPath = () => {
    onChange({ ...formData, game_library_path: "" } as appconf.AppConfig);
  };

  const handleAccountExpand = (account: AccountProvider) => {
    if (account === expandedAccount || !isAccountContentVisible)
      return;

    const prefersReducedMotion
      = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
    if (prefersReducedMotion) {
      setExpandedAccount(account);
      return;
    }

    const hasGridAnimation
      = window.matchMedia?.("(min-width: 640px)").matches ?? false;
    setIsAccountContentVisible(false);
    accountContentTimerRef.current = window.setTimeout(() => {
      setExpandedAccount(account);
      accountContentTimerRef.current = window.setTimeout(
        () => {
          setIsAccountContentVisible(true);
          accountContentTimerRef.current = null;
        },
        hasGridAnimation
          ? ACCOUNT_CARD_RESIZE_MS + ACCOUNT_CARD_RESIZE_BUFFER_MS
          : 16,
      );
    }, ACCOUNT_CONTENT_FADE_MS);
  };

  const handleAccountGridTransitionEnd = (
    event: React.TransitionEvent<HTMLDivElement>,
  ) => {
    if (
      event.target !== event.currentTarget
      || event.propertyName !== "grid-template-columns"
      || isAccountContentVisible
    ) {
      return;
    }

    if (accountContentTimerRef.current !== null) {
      window.clearTimeout(accountContentTimerRef.current);
    }
    accountContentTimerRef.current = window.setTimeout(() => {
      setIsAccountContentVisible(true);
      accountContentTimerRef.current = null;
    }, 16);
  };

  return (
    <>
      <section className="space-y-2">
        <h3 className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.basic.accountAuthorizationSectionLabel")}
        </h3>
        <div
          className={`account-choice-transition grid grid-cols-1 items-stretch gap-3 motion-reduce:transition-none ${accountGridColumns}`}
          role="group"
          aria-label={t("settings.basic.accountAuthorizationSectionLabel")}
          onTransitionEnd={handleAccountGridTransitionEnd}
        >
          <BangumiAccountSettings
            formData={formData}
            isContentVisible={isAccountContentVisible}
            isExpanded={expandedAccount === "bangumi"}
            onChange={onChange}
            onConfigRefresh={onConfigRefresh}
            onExpand={() => handleAccountExpand("bangumi")}
          />

          <HikarinagiAccountSettings
            formData={formData}
            isContentVisible={isAccountContentVisible}
            isExpanded={expandedAccount === "hikarinagi"}
            onChange={onChange}
            onConfigRefresh={onConfigRefresh}
            onExpand={() => handleAccountExpand("hikarinagi")}
          />
        </div>
      </section>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          VNDB Access Token
        </label>
        <input
          type="text"
          name="vndb_access_token"
          value={formData.vndb_access_token || ""}
          onChange={handleChange}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.themeLabel")}
        </label>
        <BetterSelect
          name="theme"
          value={formData.theme}
          onChange={value =>
            onChange({ ...formData, theme: value } as appconf.AppConfig)}
          options={[
            { value: "light", label: t("settings.basic.themeLight") },
            { value: "dark", label: t("settings.basic.themeDark") },
            { value: "system", label: t("settings.basic.themeSystem") },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.languageLabel")}
        </label>
        <BetterSelect
          name="language"
          value={formData.language}
          onChange={value =>
            onChange({ ...formData, language: value } as appconf.AppConfig)}
          options={languageOptions}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.zoomLabel")}
        </label>
        <BetterSelect
          name="window_zoom_factor"
          value={String(formData.window_zoom_factor || 1)}
          onChange={value => onZoomChange(Number(value))}
          options={appZoomOptions}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.zoomHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.timezoneLabel")}
        </label>
        <BetterSelect
          name="timezone"
          value={formData.time_zone || "Asia/Shanghai"}
          onChange={value =>
            onChange({ ...formData, time_zone: value } as appconf.AppConfig)}
          options={COMMON_TIMEZONES}
          placeholder={t("settings.basic.timezonePlaceholder")}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.timezoneHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.gameLibraryPath")}
        </label>
        <BetterActionInput
          value={formData.game_library_path || ""}
          onChange={e =>
            onChange({
              ...formData,
              game_library_path: e.target.value,
            } as appconf.AppConfig)}
          placeholder={t("settings.basic.gameLibraryPathPlaceholder")}
          className="text-sm"
          containerClassName="shadow-sm"
          actions={[
            {
              ariaLabel: t("settings.basic.selectGameLibraryTitle"),
              icon: "i-mdi-folder-open-outline",
              onClick: handleSelectGameLibraryPath,
            },
            ...(formData.game_library_path
              ? [
                  {
                    ariaLabel: t("settings.basic.clearGameLibraryPath"),
                    icon: "i-mdi-close",
                    onClick: handleClearGameLibraryPath,
                  },
                ]
              : []),
          ]}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.gameLibraryPathHint")}
        </p>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.basic.closeToTray")}
          </label>
          <BetterSwitch
            id="close_to_tray"
            checked={formData.close_to_tray || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                close_to_tray: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label
            htmlFor="launch_at_login"
            className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
          >
            {t("settings.basic.launchAtLogin")}
          </label>
          <BetterSwitch
            id="launch_at_login"
            checked={formData.launch_at_login || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                launch_at_login: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>
    </>
  );
}
