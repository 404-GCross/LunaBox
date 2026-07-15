import type { enums } from "../../wailsjs/go/models";

import { enums as modelEnums } from "../../wailsjs/go/models";

export const ALL_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.BANGUMI,
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.YMGAL,
  modelEnums.SourceType.DLSITE,
  modelEnums.SourceType.TOUCHGAL,
  modelEnums.SourceType.EROGAMESCAPE,
  modelEnums.SourceType.STEAM,
];

export const DEFAULT_ENABLED_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.BANGUMI,
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.YMGAL,
  modelEnums.SourceType.STEAM,
];

const VALID_METADATA_SOURCE_SET = new Set<string>(ALL_METADATA_SOURCES);

export function normalizeEnabledMetadataSources(
  sources: readonly string[] | undefined,
): enums.SourceType[] {
  if (!sources || sources.length === 0) {
    return [...DEFAULT_ENABLED_METADATA_SOURCES];
  }

  const normalized: enums.SourceType[] = [];
  const seen = new Set<string>();
  for (const source of sources) {
    const value = source.toLowerCase().trim();
    if (!VALID_METADATA_SOURCE_SET.has(value) || seen.has(value)) {
      continue;
    }
    seen.add(value);
    normalized.push(value as enums.SourceType);
  }

  return normalized.length > 0
    ? normalized
    : [...DEFAULT_ENABLED_METADATA_SOURCES];
}
