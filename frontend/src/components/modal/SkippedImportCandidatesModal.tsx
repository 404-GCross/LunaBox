import type { ImportCandidate } from "../ui/import/types";
import { ModalPortal } from "../ui/ModalPortal";

interface SkippedImportCandidatesModalLabels {
  title: string;
  hint: string;
  path: string;
  reason: string;
  close: string;
}

interface SkippedImportCandidatesModalProps {
  isOpen: boolean;
  candidates: ImportCandidate[];
  labels: SkippedImportCandidatesModalLabels;
  onClose: () => void;
}

export function SkippedImportCandidatesModal({
  isOpen,
  candidates,
  labels,
  onClose,
}: SkippedImportCandidatesModalProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <ModalPortal>
      <div
        className="absolute inset-0 z-60 flex items-center justify-center bg-black/50 px-4 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="skipped-import-candidates-title"
          className="flex max-h-[82vh] w-full max-w-3xl flex-col rounded-xl bg-white shadow-2xl dark:bg-brand-800"
          onClick={event => event.stopPropagation()}
        >
          <div className="flex items-center justify-between border-b border-brand-200 p-5 dark:border-brand-700">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <div className="i-mdi-folder-alert-outline text-2xl text-yellow-500 dark:text-yellow-300" />
                <h3
                  id="skipped-import-candidates-title"
                  className="truncate text-lg font-bold text-brand-900 dark:text-white"
                >
                  {labels.title}
                </h3>
              </div>
              <p className="mt-1 text-sm text-brand-500 dark:text-brand-400">
                {labels.hint}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 focus:outline-none dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
            />
          </div>

          <div className="flex-1 overflow-y-auto p-5">
            <div className="space-y-2">
              {candidates.map(candidate => (
                <div
                  key={`${candidate.folderPath}-${candidate.selectedExe || candidate.searchName}`}
                  className="rounded-lg border border-brand-200 bg-brand-50/80 p-3 dark:border-brand-700 dark:bg-brand-900/30"
                >
                  <div className="truncate text-sm font-medium text-brand-900 dark:text-white">
                    {candidate.searchName || candidate.folderName}
                  </div>
                  <div className="mt-1 truncate text-xs text-brand-500 dark:text-brand-400">
                    <span className="font-medium">
                      {labels.path}
                      :
                    </span>
                    {" "}
                    {candidate.folderPath}
                  </div>

                  {candidate.skipReason && (
                    <div className="mt-2 text-xs text-yellow-700 dark:text-yellow-300">
                      <span className="font-medium">
                        {labels.reason}
                        :
                      </span>
                      {" "}
                      {candidate.skipReason}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          <div className="flex justify-end border-t border-brand-200 p-4 dark:border-brand-700">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg bg-brand-700 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-800 dark:bg-brand-600 dark:hover:bg-brand-500"
            >
              {labels.close}
            </button>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
