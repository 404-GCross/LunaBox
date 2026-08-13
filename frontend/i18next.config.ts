import { defineConfig } from "i18next-cli";

export default defineConfig({
  locales: ["en-US", "zh-CN", "zh-TW", "ja-JP"],
  extract: {
    input: "src/**/*.{js,jsx,ts,tsx}",
    output: "src/locales/{{language}}.json",
    removeUnusedKeys: true,
    preservePatterns: [
      "gameStats.periodStatsLabel.*",
      "settings.portableSetup.toast.*",
      "metadataUpdateFields.*",
      "settings.appearance.gameCardLayout_*",
    ],
  },
});
