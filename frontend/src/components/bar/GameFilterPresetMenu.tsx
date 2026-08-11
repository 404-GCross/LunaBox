import type { models, vo } from "../../../src/bindings/models";
import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CreateGameFilterPreset,
  DeleteGameFilterPreset,
  ListGameFilterPresets,
} from "../../../bindings/lunabox/internal/service/gamefilterpresetservice";
import { enums } from "../../../src/bindings/models";
import { statusOptions } from "../../consts/options";
import { getTagDisplayName } from "../../utils/tagTranslation";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";

interface PresetFilters {
  excludeStatus: boolean;
  excludeTags: boolean;
  status: enums.GameStatus;
  tags: string[];
}

interface GameFilterPresetMenuProps {
  enableTagTranslation?: boolean;
  excludeStatus: boolean;
  excludeTags: boolean;
  status: enums.GameStatus | "";
  tags: string[];
  onApplyPreset: (preset: models.GameFilterPreset) => void;
}

export function GameFilterPresetMenu({
  enableTagTranslation = true,
  excludeStatus,
  excludeTags,
  status,
  tags,
  onApplyPreset,
}: GameFilterPresetMenuProps) {
  const { t } = useTranslation();
  const [presets, setPresets] = useState<models.GameFilterPreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [isCreateFormOpen, setIsCreateFormOpen] = useState(false);
  const [draftName, setDraftName] = useState("");
  const [draftFilters, setDraftFilters] = useState<PresetFilters>({
    excludeStatus: false,
    excludeTags: false,
    status: enums.GameStatus.$zero,
    tags: [],
  });
  const [presetToDelete, setPresetToDelete]
    = useState<models.GameFilterPreset | null>(null);

  useEffect(() => {
    let active = true;
    const loadPresets = async () => {
      try {
        const result = await ListGameFilterPresets();
        if (active) {
          setPresets(Array.isArray(result) ? result : []);
        }
      }
      catch (error) {
        if (active) {
          console.error("Failed to load game filter presets:", error);
          toast.error(t("filterPresets.loadFailed"));
        }
      }
      finally {
        if (active) {
          setLoading(false);
        }
      }
    };
    void loadPresets();
    return () => {
      active = false;
    };
  }, [t]);

  const currentFilters: PresetFilters = {
    excludeStatus: Boolean(status) && excludeStatus,
    excludeTags: tags.length > 0 && excludeTags,
    status: status || enums.GameStatus.$zero,
    tags: [...tags],
  };
  const hasCurrentFilters = tags.length > 0 || Boolean(status);

  const describeFilters = (filters: PresetFilters) => {
    const descriptions: string[] = [];
    if (filters.tags.length > 0) {
      const displayTags = filters.tags
        .map(tag => getTagDisplayName(tag, enableTagTranslation))
        .join(", ");
      descriptions.push(
        t(
          filters.excludeTags
            ? "filterPresets.excludeTagsSummary"
            : "filterPresets.includeTagsSummary",
          { tags: displayTags },
        ),
      );
    }
    if (filters.status) {
      const statusOption = statusOptions.find(
        option => option.value === filters.status,
      );
      const statusLabel = statusOption ? t(statusOption.label) : filters.status;
      descriptions.push(
        t(
          filters.excludeStatus
            ? "filterPresets.excludeStatusSummary"
            : "filterPresets.includeStatusSummary",
          { status: statusLabel },
        ),
      );
    }
    return descriptions.join(" · ");
  };

  const openCreateForm = () => {
    if (!hasCurrentFilters) {
      return;
    }
    setDraftName("");
    setDraftFilters(currentFilters);
    setIsCreateFormOpen(true);
  };

  const closeForm = () => {
    setIsCreateFormOpen(false);
    setDraftName("");
  };

  const savePreset = async () => {
    const name = draftName.trim();
    if (!name) {
      toast.error(t("filterPresets.nameRequired"));
      return;
    }
    if (draftFilters.tags.length === 0 && !draftFilters.status) {
      toast.error(t("filterPresets.filterRequired"));
      return;
    }

    const request: vo.SaveGameFilterPresetRequest = {
      name,
      tags: draftFilters.tags,
      exclude_tags: draftFilters.tags.length > 0 && draftFilters.excludeTags,
      status: draftFilters.status,
      exclude_status:
        Boolean(draftFilters.status) && draftFilters.excludeStatus,
    };

    setSaving(true);
    try {
      const created = await CreateGameFilterPreset(request);
      setPresets(previous => [...previous, created]);
      toast.success(t("filterPresets.createSuccess"));
      closeForm();
    }
    catch (error) {
      console.error("Failed to save game filter preset:", error);
      toast.error(t("filterPresets.createFailed"));
    }
    finally {
      setSaving(false);
    }
  };

  const deletePreset = async (preset: models.GameFilterPreset) => {
    try {
      await DeleteGameFilterPreset(preset.id);
      setPresets(previous =>
        previous.filter(item => item.id !== preset.id),
      );
      toast.success(t("filterPresets.deleteSuccess"));
    }
    catch (error) {
      console.error("Failed to delete game filter preset:", error);
      toast.error(t("filterPresets.deleteFailed"));
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="text-xs font-medium text-brand-400 dark:text-brand-500">
          {t("filterPresets.title")}
        </div>
        <button
          type="button"
          disabled={!hasCurrentFilters || saving}
          onClick={openCreateForm}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-1.5 text-[11px] font-medium text-brand-400 transition-colors hover:bg-brand-50 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-500 dark:hover:bg-brand-700/60 dark:hover:text-brand-300"
        >
          <span className="i-mdi-bookmark-plus-outline text-sm" />
          {t("filterPresets.saveCurrent")}
        </button>
      </div>

      {isCreateFormOpen && (
        <div className="space-y-2 rounded-lg border border-brand-200 bg-brand-50/70 p-2.5 dark:border-brand-700 dark:bg-brand-900/40">
          <label
            htmlFor="game-filter-preset-name"
            className="block text-[11px] font-medium text-brand-500 dark:text-brand-400"
          >
            {t("filterPresets.name")}
          </label>
          <input
            id="game-filter-preset-name"
            value={draftName}
            autoFocus
            onChange={event => setDraftName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                void savePreset();
              }
              if (event.key === "Escape") {
                closeForm();
              }
            }}
            placeholder={t("filterPresets.namePlaceholder")}
            className="glass-input w-full rounded-lg border border-brand-200 bg-white px-2.5 py-2 text-xs text-brand-900 outline-none placeholder:text-brand-400 focus:border-neutral-500 dark:border-brand-700 dark:bg-brand-900/70 dark:text-white"
          />
          <div className="rounded-md bg-white/70 px-2 py-1.5 text-[11px] leading-relaxed text-brand-500 dark:bg-brand-800/60 dark:text-brand-400">
            {describeFilters(draftFilters)}
          </div>
          <div className="flex justify-end gap-1.5">
            <BetterButton
              size="sm"
              variant="ghost"
              disabled={saving}
              onClick={closeForm}
            >
              {t("common.cancel")}
            </BetterButton>
            <BetterButton
              size="sm"
              variant="primary"
              icon="i-mdi-plus"
              isLoading={saving}
              disabled={saving}
              onClick={() => void savePreset()}
            >
              {t("filterPresets.create")}
            </BetterButton>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center gap-2 py-4 text-xs text-brand-400 dark:text-brand-500">
          <span className="i-mdi-loading animate-spin text-base" />
          {t("common.loading")}
        </div>
      ) : presets.length === 0 ? (
        <div className="rounded-lg border border-dashed border-brand-200 px-3 py-4 text-center text-xs text-brand-400 dark:border-brand-700 dark:text-brand-500">
          {t("filterPresets.empty")}
        </div>
      ) : (
        <div className="space-y-1.5">
          {presets.map(preset => (
            <div
              key={preset.id}
              className="group flex items-center gap-1 rounded-lg border border-brand-200 bg-white/70 p-1 transition-colors hover:border-brand-300 dark:border-brand-700 dark:bg-brand-900/40 dark:hover:border-brand-600"
            >
              <button
                type="button"
                onClick={() => {
                  onApplyPreset(preset);
                  toast.success(
                    t("filterPresets.applied", { name: preset.name }),
                  );
                }}
                className="min-w-0 flex-1 rounded-md px-2 py-1.5 text-left"
              >
                <span className="block truncate text-xs font-medium text-brand-700 dark:text-brand-200">
                  {preset.name}
                </span>
                <span className="mt-0.5 block line-clamp-2 text-[11px] leading-relaxed text-brand-400 dark:text-brand-500">
                  {describeFilters({
                    excludeStatus: preset.exclude_status,
                    excludeTags: preset.exclude_tags,
                    status: preset.status,
                    tags: preset.tags || [],
                  })}
                </span>
              </button>
              <BetterButton
                size="sm"
                variant="ghost"
                icon="i-mdi-delete-outline"
                aria-label={t("filterPresets.delete", { name: preset.name })}
                onClick={() => setPresetToDelete(preset)}
                className="hover:!bg-error-50 hover:!text-error-600 dark:hover:!bg-error-900/30 dark:hover:!text-error-400"
              />
            </div>
          ))}
        </div>
      )}

      <ConfirmModal
        isOpen={Boolean(presetToDelete)}
        title={t("filterPresets.deleteTitle")}
        message={t("filterPresets.deleteMessage", {
          name: presetToDelete?.name || "",
        })}
        confirmText={t("common.delete")}
        type="danger"
        onClose={() => setPresetToDelete(null)}
        onConfirm={() => {
          if (presetToDelete) {
            void deletePreset(presetToDelete);
          }
        }}
      />
    </div>
  );
}
