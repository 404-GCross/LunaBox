import type { CSSProperties } from "react";

import { Window } from "@wailsio/runtime";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { GetFailure } from "../../../bindings/lunabox/internal/service/startupservice";
import appIcon from "../../assets/branding/appicon.png";
import { onWailsEvent } from "../../bindings/runtime";

interface StartupFailure {
  message: string;
}

function StartupWindow() {
  const { t } = useTranslation();
  const [failure, setFailure] = useState<StartupFailure>({ message: "" });
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let active = true;
    const unsubscribe = onWailsEvent<StartupFailure>(
      "startup:failed",
      nextFailure => setFailure(nextFailure),
    );

    void GetFailure().then((latestFailure) => {
      if (active) {
        setFailure(latestFailure);
      }
    });

    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  const errorMessage = failure.message || t("startup.unknownError");

  const copyError = async () => {
    await navigator.clipboard.writeText(errorMessage);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  return (
    <main
      className="startup-backdrop relative h-screen w-screen select-none overflow-hidden text-brand-900 dark:text-white"
      style={{ "--wails-draggable": "drag" } as CSSProperties}
    >
      <div
        className="pointer-events-none absolute inset-0 overflow-hidden"
        aria-hidden="true"
      >
        <div className="startup-orbit absolute h-78 w-78 rounded-full border border-primary-300/25 dark:border-primary-400/18" />
        <div className="startup-orbit startup-orbit-second absolute h-54 w-54 rounded-full border border-primary-300/35 dark:border-primary-400/22" />
        <span className="absolute left-15 top-13 h-1.5 w-1.5 rounded-full bg-primary-400/70" />
        <span className="absolute left-46 top-8 h-1 w-1 rounded-full bg-accent-400/70" />
        <span className="absolute right-18 top-15 h-1 w-1 rounded-full bg-primary-300/75" />
        <span className="absolute bottom-14 right-38 h-1.5 w-1.5 rounded-full bg-accent-300/70" />
      </div>

      <section className="relative flex h-full flex-col px-11 pb-9 pt-10">
        <header className="flex items-center gap-3">
          <div className="startup-icon relative h-12 w-12 rounded-2xl p-1.5">
            <img
              className="h-full w-full object-contain"
              src={appIcon}
              alt=""
            />
          </div>
          <div>
            <p className="text-[11px] font-semibold tracking-[0.28em] text-primary-600 uppercase dark:text-primary-300">
              LunaBox
            </p>
            <p className="mt-0.5 text-xs text-brand-500 dark:text-brand-400">
              {t("startup.errorSubtitle")}
            </p>
          </div>
        </header>

        <div className="mt-auto max-w-124">
          <div className="mb-5 flex items-center gap-3">
            <div className="grid h-8 w-8 shrink-0 place-items-center rounded-full border border-error-200 bg-error-50 text-error-600 dark:border-error-500/30 dark:bg-error-500/10 dark:text-error-300">
              <span
                className="i-mdi-alert-circle-outline text-lg"
                aria-hidden="true"
              />
            </div>
            <div>
              <h1 className="text-xl font-semibold tracking-tight">
                {t("startup.title")}
              </h1>
              <p className="mt-1 text-sm leading-5 text-brand-500 dark:text-brand-400">
                {t("startup.detail")}
              </p>
            </div>
          </div>

          <div
            className="rounded-xl border border-error-200/80 bg-error-50/70 p-3 dark:border-error-500/25 dark:bg-error-500/8"
            style={{ "--wails-draggable": "no-drag" } as CSSProperties}
          >
            <pre className="scrollbar-hide max-h-28 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-5 text-error-700 select-text dark:text-error-200">
              {errorMessage}
            </pre>
            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-lg bg-error-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-error-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-error-500"
                onClick={() => void copyError()}
              >
                <span
                  className={copied ? "i-mdi-check" : "i-mdi-content-copy"}
                  aria-hidden="true"
                />
                {copied ? t("startup.copied") : t("startup.copyError")}
              </button>
              <button
                type="button"
                className="rounded-lg border border-brand-200 bg-white/60 px-3 py-1.5 text-xs font-medium text-brand-700 transition-colors hover:bg-white dark:border-brand-700 dark:bg-brand-800/60 dark:text-brand-200 dark:hover:bg-brand-750"
                onClick={() => void Window.Close()}
              >
                {t("startup.exit")}
              </button>
            </div>
          </div>
        </div>

        <footer className="mt-5 text-[10px] tracking-wide text-brand-400 dark:text-brand-500">
          {t("startup.errorHint")}
        </footer>
      </section>
    </main>
  );
}

export default StartupWindow;
