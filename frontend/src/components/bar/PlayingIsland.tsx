import { useEffect, useMemo, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { EndCurrentPlaySession } from "../../../wailsjs/go/service/StartService";
import { useElapsedSeconds } from "../../hooks/useElapsedSeconds";
import { useAppStore } from "../../store";
import { formatDurationCompact } from "../../utils/time";
import { ProxyImage } from "../ui/ProxyImage";

function isRuntimeVisible(state: string) {
  return state === "launching" || state === "playing" || state === "ending";
}

export function PlayingIsland() {
  const { t } = useTranslation();
  const gameRuntime = useAppStore(state => state.gameRuntime);
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [isEnding, setIsEnding] = useState(false);
  const visible = isRuntimeVisible(gameRuntime.state);
  const game = gameRuntime.game;
  const elapsedSeconds = useElapsedSeconds(
    gameRuntime.startTime,
    visible && Boolean(gameRuntime.startTime),
  );

  useEffect(() => {
    setIsEnding(false);
  }, [gameRuntime.sessionId, gameRuntime.gameId]);

  const statusText = useMemo(() => {
    if (gameRuntime.state === "launching") {
      return t("playingIsland.launching");
    }
    if (gameRuntime.state === "ending" || isEnding) {
      return t("playingIsland.ending");
    }
    return t("playingIsland.elapsed", {
      duration: formatDurationCompact(elapsedSeconds, t),
    });
  }, [elapsedSeconds, gameRuntime.state, isEnding, t]);

  if (!visible || !game) {
    return null;
  }

  const handleEndPlay = async () => {
    if (!gameRuntime.gameId || isEnding) {
      return;
    }

    setIsEnding(true);
    try {
      await EndCurrentPlaySession(gameRuntime.gameId);
      toast.success(t("playingIsland.toast.endSuccess"));
    }
    catch (error) {
      console.error("Failed to end current play session:", error);
      toast.error(t("playingIsland.toast.endFailed"));
      setIsEnding(false);
    }
  };

  return (
    <div
      className={[
        "pointer-events-none absolute left-1/2 top-[calc(28px+0.75rem)] z-45",
        "w-[min(24rem,calc(100vw-9rem))] -translate-x-1/2",
        "transition-[transform,opacity] duration-300 ease-out",
        isCollapsed ? "-translate-y-3" : "translate-y-0",
      ].join(" ")}
    >
      <div
        className={[
          "pointer-events-auto relative overflow-hidden rounded-full bg-black text-white",
          "shadow-[0_16px_40px_rgba(0,0,0,0.34)] ring-1 ring-white/12",
          "transition-[height,opacity,transform] duration-300 ease-out",
          isCollapsed
            ? "h-3 opacity-85"
            : "h-14 opacity-100 hover:shadow-[0_18px_44px_rgba(0,0,0,0.42)]",
        ].join(" ")}
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      >
        {isCollapsed ? (
          <button
            type="button"
            aria-label={t("playingIsland.expand")}
            onClick={() => setIsCollapsed(false)}
            className="absolute inset-0 flex items-center justify-center bg-black"
          >
            <span className="h-1 w-12 rounded-full bg-white/70" />
          </button>
        ) : (
          <div className="flex h-full min-w-0 items-center gap-3 px-3">
            <button
              type="button"
              aria-label={t("playingIsland.collapse")}
              onClick={() => setIsCollapsed(true)}
              className="absolute left-1/2 top-1 h-1.5 w-10 -translate-x-1/2 rounded-full bg-white/22 transition-colors hover:bg-white/42"
            />
            <div className="h-9 w-9 shrink-0 overflow-hidden rounded-full bg-brand-800 ring-1 ring-white/16">
              {game.cover_url ? (
                <ProxyImage
                  src={game.cover_url}
                  alt={game.name}
                  className="h-full w-full object-cover"
                  decoding="async"
                />
              ) : (
                <div className="flex h-full w-full items-center justify-center">
                  <span className="i-mdi-gamepad-variant text-lg text-white/65" />
                </div>
              )}
            </div>
            <div className="min-w-0 flex-1 overflow-hidden">
              <div className="overflow-hidden whitespace-nowrap">
                <div className="inline-block min-w-max animate-playing-island-marquee text-sm font-semibold leading-5">
                  <span>{game.name}</span>
                  <span className="px-8 text-white/28">{game.name}</span>
                </div>
              </div>
              <div className="text-xs leading-4 text-white/68">
                {statusText}
              </div>
            </div>
            <button
              type="button"
              aria-label={t("playingIsland.end")}
              disabled={gameRuntime.state === "ending" || isEnding}
              onClick={handleEndPlay}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white transition-colors hover:bg-white/12 active:scale-95 disabled:cursor-not-allowed disabled:opacity-55"
            >
              <span
                className={
                  gameRuntime.state === "ending" || isEnding
                    ? "i-mdi-loading animate-spin text-xl"
                    : "i-mdi-stop text-xl"
                }
              />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
