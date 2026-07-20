import type { enums } from "../../src/bindings/models";

import { enums as modelEnums } from "../../src/bindings/models";

export const ALL_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.Bangumi,
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.Ymgal,
  modelEnums.SourceType.DLsite,
  modelEnums.SourceType.TouchGal,
  modelEnums.SourceType.Hikarinagi,
  modelEnums.SourceType.ErogameScape,
  modelEnums.SourceType.Steam,
];

export const DEFAULT_ENABLED_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.Bangumi,
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.Ymgal,
  modelEnums.SourceType.Steam,
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
