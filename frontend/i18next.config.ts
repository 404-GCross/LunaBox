import { defineConfig } from "i18next-cli";

export default defineConfig({
  locales: ["en-US", "zh-CN", "zh-TW", "ja-JP"],
  extract: {
    input: "src/**/*.{js,jsx,ts,tsx}",
    output: "src/locales/{{language}}.json",
    defaultNS: false,
    removeUnusedKeys: true,
    sort: false,
    preservePatterns: [
      "gameStats.periodStatsLabel.*",
      "settings.portableSetup.toast.*",
      "metadataUpdateFields.*",
      "settings.appearance.gameCardLayout_*",
    ],
  },
});
