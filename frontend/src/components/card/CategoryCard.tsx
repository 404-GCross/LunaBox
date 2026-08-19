import type { vo } from "../../../src/bindings/models";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { GameCoverImage } from "../ui/GameCoverImage";

const COVER_STACK_SLOTS = [
  {
    id: "back",
    isFront: false,
    baseClass: "-rotate-10 -translate-x-4 translate-y-1 scale-[0.82]",
    hoverClass:
      "group-hover:-rotate-7 group-hover:-translate-x-[13px] group-hover:translate-y-1 group-focus-within:-rotate-7 group-focus-within:-translate-x-[13px] group-focus-within:translate-y-1",
    zIndex: 0,
  },
  {
    id: "middle",
    isFront: false,
    baseClass: "rotate-1 scale-90",
    hoverClass:
      "group-hover:rotate-1 group-hover:-translate-x-[3px] group-hover:-translate-y-0.5 group-focus-within:rotate-1 group-focus-within:-translate-x-[3px] group-focus-within:-translate-y-0.5",
    zIndex: 10,
  },
  {
    id: "front",
    isFront: true,
    baseClass: "rotate-10 translate-x-4 translate-y-1 scale-100",
    hoverClass:
      "group-hover:rotate-7 group-hover:translate-x-[13px] group-hover:translate-y-1 group-focus-within:rotate-7 group-focus-within:translate-x-[13px] group-focus-within:translate-y-1",
    zIndex: 20,
  },
];

interface CategoryCardProps {
  category: vo.CategoryVO;
  selectionMode?: boolean;
  selected?: boolean;
  selectionDisabled?: boolean;
  onSelectChange?: (selected: boolean) => void;
}

export function CategoryCard({
  category,
  selectionMode = false,
  selected = false,
  selectionDisabled = false,
  onSelectChange,
}: CategoryCardProps) {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const handleViewDetails = () => {
    navigate({ to: `/categories/${category.id}` });
  };

  const handleToggleSelect = (e?: React.MouseEvent) => {
    if (e) {
      e.preventDefault();
      e.stopPropagation();
    }
    if (selectionDisabled)
      return;
    onSelectChange?.(!selected);
  };

  const handleCardClick = () => {
    if (selectionMode) {
      handleToggleSelect();
      return;
    }
    handleViewDetails();
  };

  const previewGamesWithCovers = (category.preview_games || [])
    .filter(game => game.cover_url || game.cover_source_url)
    .slice(0, 3);
  const displayName = category.is_system
    ? t("categories.favorites")
    : category.name;
  const coverGames
    = previewGamesWithCovers.length === 0
      ? []
      : previewGamesWithCovers.length === 1
        ? [
            previewGamesWithCovers[0],
            previewGamesWithCovers[0],
            previewGamesWithCovers[0],
          ]
        : previewGamesWithCovers.length === 2
          ? [
              previewGamesWithCovers[0],
              previewGamesWithCovers[0],
              previewGamesWithCovers[1],
            ]
          : previewGamesWithCovers;
  const coverStack = coverGames.map((game, index) => ({
    game,
    slot: COVER_STACK_SLOTS[index],
  }));

  return (
    <article
      data-drag-selection-id={
        selectionMode && !selectionDisabled ? category.id : undefined
      }
      className={`group relative w-full min-w-[min(11rem,100%)] max-w-48 justify-self-center rounded-2xl hover:z-30 focus-within:z-30 ${selectionMode && !selectionDisabled ? "[touch-action:none]" : ""}`}
    >
      <div
        className={`relative aspect-[4/5] w-full rounded-xl bg-brand-150/65 dark:bg-brand-800/55 data-glass:bg-brand-100/35 data-glass:dark:bg-black/12 ${selectionMode && selected ? "ring-2 ring-primary-500 ring-offset-2 ring-offset-white dark:ring-primary-400 dark:ring-offset-brand-900" : ""}`}
      >
        <button
          type="button"
          onClick={handleCardClick}
          aria-label={displayName}
          aria-pressed={selectionMode ? selected : undefined}
          aria-disabled={selectionMode && selectionDisabled}
          className={`absolute inset-0 overflow-visible rounded-xl text-left outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 ${selectionMode && selectionDisabled ? "cursor-not-allowed opacity-70" : "cursor-pointer"}`}
        >
          {coverStack.length > 0
            ? coverStack.map(({ game, slot }) => (
                <span
                  key={`${game.id || category.id}-${slot.id}`}
                  className="absolute left-[47%] top-1/2 aspect-[2/3] w-[68%] -translate-x-1/2 -translate-y-1/2"
                  style={{ zIndex: slot.zIndex }}
                >
                  <span
                    className={`block h-full w-full transition-transform duration-500 ease-out motion-reduce:transition-none ${slot.hoverClass}`}
                  >
                    <span
                      className={`block h-full w-full overflow-hidden rounded-lg border-[3px] border-white bg-brand-200 shadow-md shadow-black/15 dark:border-brand-900 dark:bg-brand-700 dark:shadow-black/30 ${slot.baseClass}`}
                    >
                      <GameCoverImage
                        src={game.cover_url || game.cover_source_url}
                        fallbackSrc={game.cover_source_url}
                        alt=""
                        aria-hidden="true"
                        isNSFW={game.is_nsfw}
                        className="h-full w-full"
                        imageClassName="h-full w-full object-cover object-center"
                        decoding="async"
                        loading="lazy"
                      />
                    </span>
                  </span>
                </span>
              ))
            : COVER_STACK_SLOTS.map(slot => (
                <span
                  key={`empty-cover-${slot.id}`}
                  className="absolute left-[47%] top-1/2 aspect-[2/3] w-[68%] -translate-x-1/2 -translate-y-1/2"
                  style={{ zIndex: slot.zIndex }}
                >
                  <span
                    className={`block h-full w-full transition-transform duration-500 ease-out motion-reduce:transition-none ${slot.hoverClass}`}
                  >
                    <span
                      className={`flex h-full w-full items-center justify-center overflow-hidden rounded-lg border-[3px] border-white bg-brand-150 shadow-md shadow-black/10 dark:border-brand-900 dark:bg-brand-750 dark:shadow-black/30 ${slot.baseClass}`}
                    >
                      {slot.isFront && (
                        <span
                          className={`${category.is_system ? "i-mdi-heart-outline text-error-400" : "i-mdi-folder-open-outline text-brand-400 dark:text-brand-500"} text-5xl`}
                        />
                      )}
                    </span>
                  </span>
                </span>
              ))}

          <span className="pointer-events-none absolute bottom-2 right-2 z-30 inline-flex items-center gap-1 rounded-full bg-white/85 px-2 py-0.5 text-xs font-medium tabular-nums text-brand-900 shadow-sm backdrop-blur-sm dark:bg-black/55 dark:text-brand-100">
            <span className="i-mdi-layers-triple-outline size-3" />
            {category.game_count || 0}
          </span>
        </button>

        {selectionMode && (
          <button
            type="button"
            onClick={handleToggleSelect}
            aria-label={displayName}
            disabled={selectionDisabled}
            className="absolute right-3 top-3 z-20 flex h-7 w-7 items-center justify-center rounded-full border shadow-md backdrop-blur-md transition-transform hover:scale-105 disabled:cursor-not-allowed"
          >
            {selectionDisabled ? (
              <div className="i-mdi-lock text-brand-400 dark:text-brand-500 text-base" />
            ) : (
              <div
                className={`absolute inset-0 flex items-center justify-center rounded-full border ${
                  selected
                    ? "border-primary-500 bg-primary-500 text-white dark:border-primary-400 dark:bg-primary-500"
                    : "border-brand-300 bg-white/90 text-transparent dark:border-brand-600 dark:bg-brand-800/90"
                }`}
              >
                <div className="i-mdi-check text-base" />
              </div>
            )}
          </button>
        )}
      </div>

      <button
        type="button"
        onClick={handleCardClick}
        aria-label={displayName}
        className={`mt-2.5 block w-full min-w-0 text-center outline-none ${selectionMode && selectionDisabled ? "cursor-not-allowed opacity-70" : ""}`}
      >
        <h3 className="flex min-w-0 items-center justify-center gap-1.5 text-sm font-medium text-brand-900 dark:text-brand-100">
          {(category.emoji || "").trim() && (
            <span className="shrink-0 leading-none" aria-hidden="true">
              {category.emoji}
            </span>
          )}
          <span className="min-w-0 truncate">{displayName}</span>
        </h3>
      </button>
    </article>
  );
}
