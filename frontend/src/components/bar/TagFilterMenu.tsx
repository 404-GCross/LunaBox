import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { getTagDisplayName } from "../../utils/tagTranslation";

interface TagFilterMenuProps {
  selectedTags: string[];
  tagInput: string;
  tagSuggestions: string[];
  enableTagTranslation?: boolean;
  inverted?: boolean;
  onTagInputChange: (value: string) => void;
  onSelectTag: (tagName: string) => void;
  onRemoveTag: (tagName: string) => void;
  onClearTagFilter: () => void;
  onInvertedChange?: (value: boolean) => void;
}

export function TagFilterMenu({
  selectedTags,
  tagInput,
  tagSuggestions,
  enableTagTranslation = true,
  inverted = false,
  onTagInputChange,
  onSelectTag,
  onRemoveTag,
  onClearTagFilter,
  onInvertedChange,
}: TagFilterMenuProps) {
  const { t } = useTranslation();
  const [isTagInputFocused, setIsTagInputFocused] = useState(false);
  const tagInputRef = useRef<HTMLInputElement>(null);
  const canInvert = selectedTags.length > 0;
  const isInverted = canInvert && inverted;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="text-xs font-medium text-brand-400 dark:text-brand-500">
          {t("filterBar.tagFilter")}
        </div>
        {onInvertedChange && (
          <button
            type="button"
            disabled={!canInvert}
            aria-label={t("filterBar.invertTagFilter")}
            onClick={() => onInvertedChange(!isInverted)}
            className={`inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-1.5 text-[11px] font-medium transition-colors
              ${
          isInverted
            ? "text-neutral-600 hover:text-neutral-700 dark:text-brand-200 dark:hover:text-white"
            : "text-brand-400 hover:bg-brand-50 hover:text-brand-600 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-500 dark:hover:bg-brand-700/60 dark:hover:text-brand-300"
          }`}
          >
            <div className="i-mdi-swap-horizontal text-sm" />
            {t("filterBar.invert")}
          </button>
        )}
      </div>
      <div className="relative">
        <div
          className={`relative flex w-full min-w-0 flex-wrap items-center gap-1.5 rounded-lg border border-brand-200 bg-white px-2 py-1.5 dark:border-brand-700 dark:bg-brand-900/50 cursor-text min-h-[34px] ${selectedTags.length > 0 || tagInput ? "pr-8 pb-5" : ""}`}
          onClick={() => {
            setIsTagInputFocused(true);
            setTimeout(() => tagInputRef.current?.focus(), 0);
          }}
        >
          <div className="i-mdi-tag-outline text-base text-brand-500 dark:text-brand-400 shrink-0" />
          {selectedTags.map(tag => (
            <span
              key={tag}
              className="inline-flex max-w-full items-center gap-1 break-all rounded bg-brand-100 px-1.5 py-0.5 text-xs text-brand-700 dark:bg-brand-800 dark:text-brand-200"
            >
              {getTagDisplayName(tag, enableTagTranslation)}
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  if (selectedTags.length <= 1) {
                    onInvertedChange?.(false);
                  }
                  onRemoveTag(tag);
                }}
                className="hover:text-brand-900 dark:hover:text-white"
              >
                <div className="i-mdi-close text-[10px]" />
              </button>
            </span>
          ))}
          {(!selectedTags.length || isTagInputFocused || tagInput) && (
            <div className="flex min-w-[96px] flex-[1_1_96px] max-w-full">
              <input
                ref={tagInputRef}
                type="text"
                value={tagInput}
                onChange={event => onTagInputChange(event.target.value)}
                onFocus={() => setIsTagInputFocused(true)}
                onBlur={() => {
                  setTimeout(() => setIsTagInputFocused(false), 200);
                }}
                onKeyDown={(event) => {
                  if (
                    event.key === "Backspace"
                    && !tagInput
                    && selectedTags.length > 0
                  ) {
                    if (selectedTags.length === 1) {
                      onInvertedChange?.(false);
                    }
                    onRemoveTag(selectedTags[selectedTags.length - 1]);
                  }
                }}
                placeholder={
                  selectedTags.length ? "" : t("filterBar.tagsPlaceholder")
                }
                className="min-w-0 w-full bg-transparent text-xs text-brand-900 outline-none placeholder:text-brand-400 dark:text-white"
              />
            </div>
          )}
          {(selectedTags.length > 0 || tagInput) && (
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                onInvertedChange?.(false);
                onClearTagFilter();
              }}
              className="absolute bottom-1 right-1 rounded-full bg-white/80 p-0.5 text-brand-400 transition-colors hover:text-brand-600 dark:bg-brand-900/70 dark:hover:text-brand-200"
            >
              <div className="i-mdi-close-circle text-sm" />
            </button>
          )}
        </div>
        {tagInput && tagSuggestions.length > 0 && (
          <div className="absolute left-0 right-0 top-full z-30 mt-1 max-h-36 overflow-y-auto rounded-lg border border-brand-200 bg-white/95 p-1 shadow-xl backdrop-blur dark:border-brand-700 dark:bg-brand-900/95">
            {tagSuggestions.map(tagName => (
              <button
                key={tagName}
                type="button"
                onClick={() => onSelectTag(tagName)}
                className="w-full rounded-md px-2.5 py-1.5 text-left text-xs text-brand-700 transition-colors hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-700"
              >
                {getTagDisplayName(tagName, enableTagTranslation)}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
