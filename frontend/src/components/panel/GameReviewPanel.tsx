import type { enums, models, vo } from "../../../src/bindings/models";
import { useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  GetGameReview,
  SaveGameReview,
  SyncGameReview,
} from "../../../bindings/lunabox/internal/service/gamereviewservice";
import {
  enums as modelEnums,
  models as modelTypes,
} from "../../../src/bindings/models";
import { fetchBangumiAuthStatus } from "../../utils/bangumiAuth";
import { fetchHikarinagiAuthStatus } from "../../utils/hikarinagiAuth";
import { getMetadataSourceIcon } from "../../utils/metadataSources";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameReviewPanelProps {
  game: models.Game;
}

interface ProviderState {
  provider: enums.SourceType;
  name: string;
  linked: boolean;
  authorized: boolean;
}

interface ProviderCardProps extends ProviderState {
  checked: boolean;
  result?: vo.GameReviewProviderSyncResult;
  onCheckedChange: (checked: boolean) => void;
}

const REVIEW_PROVIDERS = [
  modelEnums.SourceType.Bangumi,
  modelEnums.SourceType.Hikarinagi,
] as const;

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function gameHasProvider(
  game: models.Game,
  provider: enums.SourceType,
): boolean {
  const linked = (game.metadata_sources ?? []).some(
    source =>
      source.source_type === provider && Boolean(source.source_id?.trim()),
  );
  return (
    linked || (game.source_type === provider && Boolean(game.source_id?.trim()))
  );
}

function ProviderCard({
  provider,
  name,
  linked,
  authorized,
  checked,
  result,
  onCheckedChange,
}: ProviderCardProps) {
  const { t } = useTranslation();
  const enabled = linked && authorized;
  const icon = getMetadataSourceIcon(provider, "compact");
  const stateText = !linked
    ? t("gameReview.provider.sourceMissing")
    : !authorized
        ? t("gameReview.provider.authRequired")
        : t("gameReview.provider.ready");
  const resultClass
    = result?.status === "success"
      ? "bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-300"
      : "bg-error-100 text-error-700 dark:bg-error-900/30 dark:text-error-300";

  return (
    <label
      className={`flex min-h-24 items-center gap-3 rounded-xl border p-4 transition-colors ${
        enabled
          ? "cursor-pointer border-brand-200 bg-brand-50 hover:border-brand-400 dark:border-brand-650 dark:bg-brand-750 dark:hover:border-brand-500"
          : "cursor-not-allowed border-brand-200 bg-brand-100/60 opacity-70 dark:border-brand-700 dark:bg-brand-750/40"
      }`}
    >
      <input
        type="checkbox"
        className="sr-only"
        checked={checked}
        disabled={!enabled}
        onChange={event => onCheckedChange(event.target.checked)}
      />
      <span
        aria-hidden="true"
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors ${
          checked
            ? "border-neutral-600 bg-neutral-600 text-white"
            : "border-brand-400 bg-white dark:border-brand-500 dark:bg-brand-700"
        }`}
      >
        {checked && <span className="i-mdi-check text-sm" />}
      </span>
      <span className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-white p-1.5 shadow-sm dark:bg-brand-700">
        {icon ? (
          <img
            src={icon}
            alt=""
            className="max-h-full max-w-full object-contain"
          />
        ) : (
          <span className="i-mdi-cloud-upload-outline text-xl text-brand-500" />
        )}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-center gap-2">
          <span className="font-medium text-brand-900 dark:text-white">
            {name}
          </span>
          {result && (
            <span
              className={`rounded-full px-2 py-0.5 text-xs font-medium ${resultClass}`}
            >
              {result.status === "success"
                ? t("gameReview.provider.synced")
                : t("gameReview.provider.failed")}
            </span>
          )}
        </span>
        <span className="mt-1 block text-xs leading-relaxed text-brand-500 dark:text-brand-400">
          {result?.error || stateText}
        </span>
      </span>
    </label>
  );
}

export function GameReviewPanel({ game }: GameReviewPanelProps) {
  const { t } = useTranslation();
  const [rating, setRating] = useState<number | null>(null);
  const [content, setContent] = useState("");
  const [isSpoiler, setIsSpoiler] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [auth, setAuth] = useState<Record<string, boolean>>({});
  const [selectedProviders, setSelectedProviders] = useState<
    enums.SourceType[]
  >([]);
  const [syncResults, setSyncResults] = useState<
    vo.GameReviewProviderSyncResult[]
  >([]);

  const providers = useMemo<ProviderState[]>(
    () => [
      {
        provider: modelEnums.SourceType.Bangumi,
        name: "Bangumi",
        linked: gameHasProvider(game, modelEnums.SourceType.Bangumi),
        authorized: Boolean(auth[modelEnums.SourceType.Bangumi]),
      },
      {
        provider: modelEnums.SourceType.Hikarinagi,
        name: "Hikarinagi",
        linked: gameHasProvider(game, modelEnums.SourceType.Hikarinagi),
        authorized: Boolean(auth[modelEnums.SourceType.Hikarinagi]),
      },
    ],
    [auth, game],
  );

  useEffect(() => {
    let cancelled = false;

    async function loadReview() {
      setIsLoading(true);
      const [reviewResult, bangumiResult, hikarinagiResult]
        = await Promise.allSettled([
          GetGameReview(game.id),
          fetchBangumiAuthStatus(),
          fetchHikarinagiAuthStatus(),
        ]);
      if (cancelled) {
        return;
      }

      if (reviewResult.status === "fulfilled" && reviewResult.value) {
        setRating(reviewResult.value.rating);
        setContent(reviewResult.value.content);
        setIsSpoiler(reviewResult.value.is_spoiler);
      }
      else if (reviewResult.status === "rejected") {
        toast.error(
          t("gameReview.toast.loadFailed", {
            error: errorMessage(reviewResult.reason),
          }),
        );
      }

      const nextAuth: Record<string, boolean> = {
        [modelEnums.SourceType.Bangumi]:
          bangumiResult.status === "fulfilled"
          && bangumiResult.value.authorized
          && !bangumiResult.value.needs_reauthorization,
        [modelEnums.SourceType.Hikarinagi]:
          hikarinagiResult.status === "fulfilled"
          && hikarinagiResult.value.authorized
          && !hikarinagiResult.value.needs_reauthorization,
      };
      setAuth(nextAuth);
      setSelectedProviders(
        REVIEW_PROVIDERS.filter(
          provider => nextAuth[provider] && gameHasProvider(game, provider),
        ),
      );
      setIsLoading(false);
    }

    loadReview();
    return () => {
      cancelled = true;
    };
  }, [game, t]);

  const saveReview = async (): Promise<models.GameReview> => {
    const saved = await SaveGameReview(
      modelTypes.GameReview.createFrom({
        game_id: game.id,
        rating,
        content,
        is_spoiler: isSpoiler,
      }),
    );
    if (!saved) {
      throw new Error(t("gameReview.toast.emptySaveResult"));
    }
    return saved;
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await saveReview();
      toast.success(t("gameReview.toast.saved"));
    }
    catch (error) {
      toast.error(
        t("gameReview.toast.saveFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSaving(false);
    }
  };

  const handleSaveAndSync = async () => {
    setIsSyncing(true);
    setSyncResults([]);
    try {
      await saveReview();
      const result = await SyncGameReview(game.id, selectedProviders);
      setSyncResults(result.results ?? []);
      if (result.failed > 0) {
        toast.error(
          t("gameReview.toast.syncPartial", {
            succeeded: result.succeeded,
            failed: result.failed,
          }),
        );
      }
      else {
        toast.success(
          t("gameReview.toast.synced", { count: result.succeeded }),
        );
      }
    }
    catch (error) {
      toast.error(
        t("gameReview.toast.syncFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSyncing(false);
    }
  };

  const setProviderChecked = (provider: enums.SourceType, checked: boolean) => {
    setSelectedProviders(current =>
      checked
        ? [...new Set([...current, provider])]
        : current.filter(item => item !== provider),
    );
  };

  if (isLoading) {
    return (
      <div className="glass-card min-h-[22rem] animate-pulse rounded-lg bg-white p-6 shadow-sm dark:bg-brand-800">
        <div className="h-6 w-36 rounded bg-brand-200 dark:bg-brand-700" />
        <div className="mt-6 h-20 rounded-xl bg-brand-100 dark:bg-brand-750" />
        <div className="mt-5 h-36 rounded-xl bg-brand-100 dark:bg-brand-750" />
      </div>
    );
  }

  return (
    <div className="grid gap-5 xl:grid-cols-[minmax(0,1.45fr)_minmax(20rem,0.85fr)]">
      <section className="glass-card rounded-lg bg-white p-6 shadow-sm dark:bg-brand-800">
        <div className="flex flex-col gap-1">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
            {t("gameReview.title")}
          </h3>
          <p className="text-sm leading-relaxed text-brand-500 dark:text-brand-400">
            {t("gameReview.hint")}
          </p>
        </div>

        <fieldset className="mt-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <legend className="text-sm font-medium text-brand-800 dark:text-brand-100">
              {t("gameReview.ratingLabel")}
            </legend>
            <span className="font-mono text-sm text-brand-500 dark:text-brand-400">
              {rating === null
                ? t("gameReview.ratingEmpty")
                : t("gameReview.ratingValue", { rating })}
            </span>
          </div>
          <div
            className="mt-3 grid grid-cols-5 gap-2 sm:grid-cols-10"
            role="radiogroup"
            aria-label={t("gameReview.ratingLabel")}
          >
            {Array.from({ length: 10 }, (_, index) => index + 1).map(
              value => (
                <button
                  key={value}
                  type="button"
                  role="radio"
                  aria-checked={rating === value}
                  onClick={() => setRating(value)}
                  className={`h-10 rounded-lg border text-sm font-semibold transition-all focus:outline-none focus:ring-2 focus:ring-neutral-500 focus:ring-offset-2 dark:focus:ring-offset-brand-800 ${
                    rating === value
                      ? "border-neutral-600 bg-neutral-600 text-white shadow-sm"
                      : "border-brand-250 bg-brand-50 text-brand-700 hover:border-brand-400 hover:bg-brand-100 dark:border-brand-650 dark:bg-brand-750 dark:text-brand-200 dark:hover:border-brand-500"
                  }`}
                >
                  {value}
                </button>
              ),
            )}
          </div>
          <button
            type="button"
            className="mt-2 text-xs text-brand-500 underline-offset-2 hover:text-brand-800 hover:underline dark:text-brand-400 dark:hover:text-brand-200"
            onClick={() => setRating(null)}
          >
            {t("gameReview.clearRating")}
          </button>
        </fieldset>

        <div className="mt-6">
          <label
            htmlFor="game-review-content"
            className="text-sm font-medium text-brand-800 dark:text-brand-100"
          >
            {t("gameReview.contentLabel")}
          </label>
          <textarea
            id="game-review-content"
            value={content}
            onChange={event => setContent(event.target.value)}
            rows={8}
            placeholder={t("gameReview.contentPlaceholder")}
            className="mt-2 w-full resize-y rounded-xl border border-brand-250 bg-brand-50 px-4 py-3 text-sm leading-6 text-brand-900 outline-none transition-colors placeholder:text-brand-400 focus:border-neutral-500 focus:ring-2 focus:ring-neutral-500/20 dark:border-brand-650 dark:bg-brand-750 dark:text-white dark:placeholder:text-brand-500"
          />
          <div className="mt-2 flex flex-col gap-3 rounded-lg bg-brand-50 px-3 py-2.5 dark:bg-brand-750 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <label
                htmlFor="game-review-spoiler"
                className="text-sm font-medium text-brand-800 dark:text-brand-100"
              >
                {t("gameReview.spoilerLabel")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("gameReview.spoilerHint")}
              </p>
            </div>
            <BetterSwitch
              id="game-review-spoiler"
              checked={isSpoiler}
              onCheckedChange={setIsSpoiler}
            />
          </div>
        </div>

        <div className="mt-6 flex flex-col-reverse gap-3 border-t border-brand-200 pt-5 dark:border-brand-700 sm:flex-row sm:justify-end">
          <BetterButton
            variant="secondary"
            icon="i-mdi-content-save-outline"
            isLoading={isSaving}
            disabled={isSyncing}
            onClick={handleSave}
          >
            {t("gameReview.saveLocal")}
          </BetterButton>
          <BetterButton
            variant="primary"
            icon="i-mdi-cloud-upload-outline"
            isLoading={isSyncing}
            disabled={isSaving || selectedProviders.length === 0}
            onClick={handleSaveAndSync}
          >
            {t("gameReview.saveAndSync")}
          </BetterButton>
        </div>
      </section>

      <aside className="glass-card h-fit rounded-lg bg-white p-6 shadow-sm dark:bg-brand-800">
        <div className="flex items-start gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-100 text-xl text-brand-600 dark:bg-brand-700 dark:text-brand-300">
            <span className="i-mdi-cloud-sync-outline" />
          </span>
          <div>
            <h3 className="font-semibold text-brand-900 dark:text-white">
              {t("gameReview.syncTitle")}
            </h3>
            <p className="mt-1 text-xs leading-relaxed text-brand-500 dark:text-brand-400">
              {t("gameReview.syncHint")}
            </p>
          </div>
        </div>

        <div className="mt-5 space-y-3">
          {providers.map(provider => (
            <ProviderCard
              key={provider.provider}
              {...provider}
              checked={selectedProviders.includes(provider.provider)}
              result={syncResults.find(
                item => item.provider === provider.provider,
              )}
              onCheckedChange={checked =>
                setProviderChecked(provider.provider, checked)}
            />
          ))}
        </div>

        <p className="mt-4 rounded-lg border border-brand-200 bg-brand-50 px-3 py-2.5 text-xs leading-relaxed text-brand-500 dark:border-brand-700 dark:bg-brand-750 dark:text-brand-400">
          {t("gameReview.localFirstHint")}
        </p>
      </aside>
    </div>
  );
}
