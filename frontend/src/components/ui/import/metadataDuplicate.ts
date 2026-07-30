import type { enums } from "../../../../wailsjs/go/models";
import type { ImportCandidate } from "./types";

import { vo } from "../../../../wailsjs/go/models";
import { CheckImportMetadataDuplicates } from "../../../../wailsjs/go/service/ImportService";

function getMetadataDuplicateKey(
  source: enums.SourceType | null | undefined,
  sourceId: string | undefined,
): string {
  if (!source || !sourceId) {
    return "";
  }
  return `${source}\0${sourceId.trim().toLowerCase()}`;
}

export async function applyMetadataDuplicateHints(
  candidates: ImportCandidate[],
): Promise<ImportCandidate[]> {
  const requestsByKey = new Map<string, vo.ImportMetadataDuplicateRequest>();

  for (const candidate of candidates) {
    const source = candidate.matchSource || candidate.matchedGame?.source_type;
    const sourceId = candidate.matchedGame?.source_id;
    if (!source || !sourceId) {
      continue;
    }
    const key = getMetadataDuplicateKey(source, sourceId);
    if (requestsByKey.has(key)) {
      continue;
    }
    requestsByKey.set(
      key,
      new vo.ImportMetadataDuplicateRequest({
        source,
        source_id: sourceId,
      }),
    );
  }

  if (requestsByKey.size === 0) {
    return candidates.map(candidate => ({
      ...candidate,
      metadataDuplicateExistingId: undefined,
      metadataDuplicateExistingName: undefined,
    }));
  }

  const results = await CheckImportMetadataDuplicates([
    ...requestsByKey.values(),
  ]);
  const resultsByKey = new Map(
    (results || []).map(result => [
      getMetadataDuplicateKey(result.source, result.source_id),
      result,
    ]),
  );

  return candidates.map((candidate) => {
    const source = candidate.matchSource || candidate.matchedGame?.source_type;
    const sourceId = candidate.matchedGame?.source_id;
    const duplicate = resultsByKey.get(
      getMetadataDuplicateKey(source, sourceId),
    );

    return {
      ...candidate,
      metadataDuplicateExistingId: duplicate?.exists
        ? duplicate.existing_id
        : undefined,
      metadataDuplicateExistingName: duplicate?.exists
        ? duplicate.existing_name
        : undefined,
    };
  });
}
