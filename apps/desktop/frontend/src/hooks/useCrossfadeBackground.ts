import { useEffect, useLayoutEffect, useRef, useState } from "react";

type CrossfadeBackgroundOptions = {
  durationMs?: number;
};

export function useCrossfadeBackground(
  nextBackgroundUrl: string | null | undefined,
  { durationMs = 1200 }: CrossfadeBackgroundOptions = {},
) {
  const [previousBackgroundUrl, setPreviousBackgroundUrl] = useState<
    string | null
  >(null);
  const [isBackgroundCrossfading, setIsBackgroundCrossfading] = useState(false);
  const backgroundUrlRef = useRef<string | null>(null);
  const frameRef = useRef<number | null>(null);
  const timerRef = useRef<number | null>(null);
  const normalizedBackgroundUrl = nextBackgroundUrl || null;

  useLayoutEffect(() => {
    const currentBackgroundUrl = backgroundUrlRef.current;

    if (frameRef.current !== null) {
      window.cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    if (
      currentBackgroundUrl
      && normalizedBackgroundUrl
      && currentBackgroundUrl !== normalizedBackgroundUrl
    ) {
      setPreviousBackgroundUrl(currentBackgroundUrl);
      setIsBackgroundCrossfading(true);

      frameRef.current = window.requestAnimationFrame(() => {
        setIsBackgroundCrossfading(false);
        frameRef.current = null;
      });

      timerRef.current = window.setTimeout(() => {
        setPreviousBackgroundUrl(null);
        timerRef.current = null;
      }, durationMs);
    }
    else if (!normalizedBackgroundUrl) {
      setPreviousBackgroundUrl(null);
      setIsBackgroundCrossfading(false);
    }

    backgroundUrlRef.current = normalizedBackgroundUrl;
  }, [durationMs, normalizedBackgroundUrl]);

  useEffect(() => {
    return () => {
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
      }
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
      }
    };
  }, []);

  return {
    isBackgroundCrossfading,
    previousBackgroundUrl,
  };
}
