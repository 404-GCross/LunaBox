import type { service } from "../../../src/bindings/models";
import type { BetterDataTableColumn } from "../ui/better/BetterDataTable";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterDataTable } from "../ui/better/BetterDataTable";
import { ImportModalContainer } from "../ui/import/ImportModalContainer";

interface GameIDEnrichmentPreviewModalProps {
  isOpen: boolean;
  isLoading: boolean;
  isApplying: boolean;
  preview: service.GameIDEnrichmentPreview | null;
  onClose: () => void;
  onConfirm: () => void;
}

function sourceLabel(sourceType: string) {
  switch (sourceType) {
    case "bangumi":
      return "Bangumi";
    case "vndb":
      return "VNDB";
    case "steam":
      return "Steam";
    default:
      return sourceType;
  }
}

function SourceBadge({
  source,
  added = false,
}: {
  source: service.GameIDEnrichmentSource;
  added?: boolean;
}) {
  return (
    <span
      className={[
        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs",
        added
          ? "bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400"
          : "bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400",
      ].join(" ")}
    >
      <span>{sourceLabel(source.source_type)}</span>
      <span className="font-mono">{source.source_id}</span>
    </span>
  );
}

export function GameIDEnrichmentPreviewModal({
  isOpen,
  isLoading,
  isApplying,
  preview,
  onClose,
  onConfirm,
}: GameIDEnrichmentPreviewModalProps) {
  const { t } = useTranslation();
  const columns = useMemo<
    BetterDataTableColumn<service.GameIDEnrichmentPreviewItem>[]
  >(
    () => [
      {
        key: "game",
        header: t("settings.metadata.idPreview.columns.game"),
        className: "w-[30%]",
        render: item => (
          <div className="min-w-0">
            <div className="truncate font-medium text-brand-900 dark:text-white">
              {item.game_name}
            </div>
            <div className="mt-0.5 truncate text-xs text-brand-400 dark:text-brand-500">
              {sourceLabel(item.default_source)}
              {" "}
              {item.default_source_id}
            </div>
          </div>
        ),
      },
      {
        key: "existing",
        header: t("settings.metadata.idPreview.columns.existing"),
        className: "w-[34%]",
        render: item => (
          <div className="flex flex-wrap gap-1.5">
            {item.existing_sources.map(source => (
              <SourceBadge
                key={`${item.game_id}-existing-${source.source_type}`}
                source={source}
              />
            ))}
          </div>
        ),
      },
      {
        key: "result",
        header: t("settings.metadata.idPreview.columns.result"),
        className: "w-[36%]",
        render: item =>
          item.can_enrich ? (
            <div className="flex flex-wrap gap-1.5">
              {item.added_sources.map(source => (
                <SourceBadge
                  key={`${item.game_id}-added-${source.source_type}`}
                  source={source}
                  added
                />
              ))}
            </div>
          ) : (
            <div className="flex items-start gap-1.5 text-xs text-brand-500 dark:text-brand-400">
              <span
                className="i-mdi-information-outline mt-0.5 shrink-0 text-brand-400"
                aria-hidden="true"
              />
              <span>
                {t(`settings.metadata.idPreview.reasons.${item.reason}`)}
              </span>
            </div>
          ),
      },
    ],
    [t],
  );

  if (!isOpen) {
    return null;
  }

  const closeModal = () => {
    if (!isApplying) {
      onClose();
    }
  };

  return (
    <ImportModalContainer
      title={t("settings.metadata.idPreview.title")}
      iconClassName="i-mdi-database-search-outline text-3xl text-neutral-500"
      onClose={closeModal}
    >
      {isLoading ? (
        <div className="flex min-h-80 flex-col items-center justify-center gap-3 text-brand-500 dark:text-brand-400">
          <span className="i-mdi-loading animate-spin text-4xl" />
          <span className="text-sm">
            {t("settings.metadata.idPreview.loading")}
          </span>
        </div>
      ) : preview ? (
        <div className="space-y-4">
          <div className="flex gap-4">
            <div className="flex-1 rounded-lg bg-success-50 p-4 text-center dark:bg-success-900/20">
              <div className="text-3xl font-bold text-success-600 dark:text-success-400">
                {preview.enrichable_games}
              </div>
              <div className="text-sm text-success-700 dark:text-success-300">
                {t("settings.metadata.idPreview.enrichableGames")}
              </div>
            </div>
            <div className="flex-1 rounded-lg bg-gray-50 p-4 text-center dark:bg-gray-900/20">
              <div className="text-3xl font-bold text-gray-600 dark:text-gray-400">
                {preview.unchanged_games}
              </div>
              <div className="text-sm text-gray-700 dark:text-gray-300">
                {t("settings.metadata.idPreview.unchangedGames")}
              </div>
            </div>
          </div>

          <BetterDataTable
            rows={preview.items}
            columns={columns}
            rowKey={item => item.game_id}
            empty={t("settings.metadata.idPreview.empty")}
            maxHeightClassName="max-h-[48vh]"
          />

          <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
            <div className="text-sm text-brand-500 dark:text-brand-400">
              {t("settings.metadata.idPreview.scanned", {
                count: preview.scanned_games,
              })}
            </div>
            <div className="flex items-center justify-end gap-3">
              <BetterButton
                variant="primary"
                icon="i-mdi-database-plus-outline"
                isLoading={isApplying}
                disabled={preview.enrichable_games === 0}
                onClick={onConfirm}
              >
                {t("settings.metadata.idPreview.confirm", {
                  count: preview.enrichable_games,
                })}
              </BetterButton>
            </div>
          </div>
        </div>
      ) : null}
    </ImportModalContainer>
  );
}
