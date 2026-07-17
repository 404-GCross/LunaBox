import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";

import enUS from "./locales/en-US.json";
import jaJP from "./locales/ja-JP.json";
import zhCN from "./locales/zh-CN.json";
import zhTW from "./locales/zh-TW.json";

const resources = {
  "zh-CN": { translation: zhCN },
  "zh-TW": { translation: zhTW },
  "en-US": { translation: enUS },
  "ja-JP": { translation: jaJP },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: "zh-CN",
    interpolation: {
      escapeValue: false,
    },
  });

if (import.meta.hot) {
  import.meta.hot.accept(
    [
      "./locales/zh-CN.json",
      "./locales/zh-TW.json",
      "./locales/en-US.json",
      "./locales/ja-JP.json",
    ],
    (modules) => {
      const updates = [
        ["zh-CN", modules[0]?.default],
        ["zh-TW", modules[1]?.default],
        ["en-US", modules[2]?.default],
        ["ja-JP", modules[3]?.default],
      ] as const;
      let hasUpdate = false;

      for (const [language, translations] of updates) {
        if (!translations) {
          continue;
        }

        i18n.removeResourceBundle(language, "translation");
        i18n.addResourceBundle(
          language,
          "translation",
          translations,
          true,
          true,
        );
        hasUpdate = true;
      }

      if (hasUpdate) {
        void i18n.changeLanguage(i18n.resolvedLanguage ?? i18n.language);
      }
    },
  );
}

export default i18n;
