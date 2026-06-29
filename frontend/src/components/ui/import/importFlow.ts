import type { TFunction } from "i18next";

import type { appconf, enums, vo } from "../../../../wailsjs/go/models";
import type { ImportCandidate } from "./types";

import {
  enums as modelEnums,
  vo as modelVO,
} from "../../../../wailsjs/go/models";

export type BatchScanPreset
  = | "scan_parent"
    | "scan_library_child"
    | "hierarchy_child";

export type PreferredSourceValue = enums.SourceType | "";

export const MAX_HIERARCHY_DEPTH = 5;
export const DEFAULT_SCAN_PRESET: BatchScanPreset = "scan_parent";
export const NO_PREFERRED_SOURCE = "";
export const PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD = 3;

export const DEFAULT_METADATA_SOURCE_ORDER = [
  modelEnums.SourceType.BANGUMI,
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.YMGAL,
  modelEnums.SourceType.DLSITE,
  modelEnums.SourceType.TOUCHGAL,
  modelEnums.SourceType.EROGAMESCAPE,
  modelEnums.SourceType.STEAM,
];

const VALID_METADATA_SOURCE_SET = new Set<string>(
  DEFAULT_METADATA_SOURCE_ORDER,
);

export function normalizeEnabledMetadataSources(sources: string[] | undefined) {
  if (!sources || sources.length === 0) {
    return DEFAULT_METADATA_SOURCE_ORDER;
  }

  const normalized: enums.SourceType[] = [];
  const seen = new Set<string>();
  for (const source of sources) {
    if (!VALID_METADATA_SOURCE_SET.has(source) || seen.has(source)) {
      continue;
    }
    seen.add(source);
    normalized.push(source as enums.SourceType);
  }
  return normalized.length > 0 ? normalized : DEFAULT_METADATA_SOURCE_ORDER;
}

export function sourceLabel(source: enums.SourceType, t: TFunction) {
  return source === modelEnums.SourceType.BANGUMI
    ? "Bangumi"
    : source === modelEnums.SourceType.VNDB
      ? "VNDB"
      : source === modelEnums.SourceType.YMGAL
        ? t("gameEdit.sourceYmgal")
        : source === modelEnums.SourceType.DLSITE
          ? t("gameEdit.sourceDlsite")
          : source === modelEnums.SourceType.TOUCHGAL
            ? t("gameEdit.sourceTouchGal")
            : source === modelEnums.SourceType.EROGAMESCAPE
              ? t("gameEdit.sourceErogameScape")
              : "Steam";
}

export function normalizeScanPreset(
  preset: string | undefined,
): BatchScanPreset {
  return preset === "scan_parent"
    || preset === "scan_library_child"
    || preset === "hierarchy_child"
    ? preset
    : DEFAULT_SCAN_PRESET;
}

export function clampHierarchyDepth(depth: number | undefined) {
  return Math.min(MAX_HIERARCHY_DEPTH, Math.max(0, depth ?? 0));
}

export function getImportScanConfig(config: appconf.AppConfig | null) {
  const scanPreset = normalizeScanPreset(config?.batch_import_scan_preset);
  const hierarchyDepth = clampHierarchyDepth(
    config?.batch_import_hierarchy_depth,
  );
  return {
    scanPreset,
    hierarchyDepth,
    hierarchyLevel: hierarchyDepth + 1,
    scanOptions: new modelVO.BatchImportScanOptions({
      scan_mode: scanPreset === "hierarchy_child" ? "hierarchy" : "scan",
      scan_name_mode: scanPreset === "scan_parent" ? "parent" : "depth",
      name_depth: 0,
      hierarchy_depth: hierarchyDepth,
    }),
  };
}

export function getPreferredSource(
  config: appconf.AppConfig | null,
  enabledMetadataSources: enums.SourceType[],
): PreferredSourceValue {
  const configuredPreferredSource = config?.batch_import_preferred_source || "";
  return configuredPreferredSource
    && enabledMetadataSources.includes(
      configuredPreferredSource as enums.SourceType,
    )
    ? (configuredPreferredSource as enums.SourceType)
    : NO_PREFERRED_SOURCE;
}

export function preferredSourceOptions(
  enabledMetadataSources: enums.SourceType[],
  t: TFunction,
) {
  return [
    {
      value: NO_PREFERRED_SOURCE,
      label: t("batchImportModal.preferredSource.none"),
    },
    ...enabledMetadataSources.map(source => ({
      value: source,
      label: sourceLabel(source, t),
    })),
  ];
}

export function sourcePriorityOrder(preferredSource: PreferredSourceValue) {
  if (!preferredSource) {
    return DEFAULT_METADATA_SOURCE_ORDER;
  }
  return [
    preferredSource,
    ...DEFAULT_METADATA_SOURCE_ORDER.filter(
      source => source !== preferredSource,
    ),
  ];
}

export function pickBestMatch(
  matches: vo.GameMetadataFromWebVO[],
  preferredSource: PreferredSourceValue,
) {
  for (const source of sourcePriorityOrder(preferredSource)) {
    const match = matches.find(r => r.Source === source && r.Game);
    if (match) {
      return match;
    }
  }
  return null;
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function toImportCandidate(c: vo.BatchImportCandidate): ImportCandidate {
  return {
    folderPath: c.folder_path,
    folderName: c.folder_name,
    executables: c.executables || [],
    selectedExe: c.selected_exe,
    searchName: c.search_name,
    isSelected: true,
    importStatus: c.import_status || "new",
    skipReason: c.skip_reason || "",
    existingName: c.existing_name || "",
    matchedGame: null,
    matchedTags: [],
    matchSource: null,
    matchStatus: "pending",
    matchError: "",
    metadataDuplicateExistingId: undefined,
    metadataDuplicateExistingName: undefined,
  };
}

export function scanResultToCandidates(scanned: vo.BatchImportScanResult) {
  return {
    candidates: (scanned?.candidates || []).map(toImportCandidate),
    skippedCandidates: (scanned?.skipped_candidates || []).map(c => ({
      ...toImportCandidate(c),
      isSelected: false,
    })),
  };
}

export function candidatesToImportRequest(candidates: ImportCandidate[]) {
  return candidates
    .filter(c => c.isSelected)
    .map((c) => {
      const candidate = new modelVO.BatchImportCandidate({
        folder_path: c.folderPath,
        folder_name: c.folderName,
        executables: c.executables,
        selected_exe: c.selectedExe,
        search_name: c.searchName,
        is_selected: c.isSelected,
        match_status: c.matchStatus,
        import_status: c.importStatus,
        skip_reason: c.skipReason,
        existing_name: c.existingName,
      });
      if (c.matchedGame) {
        candidate.matched_game = c.matchedGame;
      }
      if (c.matchedTags.length > 0) {
        candidate.matched_tags = c.matchedTags;
      }
      if (c.matchSource) {
        candidate.match_source = c.matchSource;
      }
      return candidate;
    });
}
