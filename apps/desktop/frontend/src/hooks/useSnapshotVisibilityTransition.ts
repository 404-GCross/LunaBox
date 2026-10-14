import { useEffect, useLayoutEffect, useRef, useState } from "react";

type SnapshotVisibilityTransitionOptions = {
  fadeInDelayMs: number;
  fadeOutMs: number;
};

export function useSnapshotVisibilityTransition<TSnapshot>(
  currentSnapshot: TSnapshot | null,
  currentSnapshotKey: string | null,
  { fadeInDelayMs, fadeOutMs }: SnapshotVisibilityTransitionOptions,
) {
  const [displayedSnapshot, setDisplayedSnapshot] = useState<TSnapshot | null>(
    null,
  );
  const [isVisible, setIsVisible] = useState(false);
  const displayedSnapshotKeyRef = useRef<string | null>(null);
  const pendingSnapshotRef = useRef<TSnapshot | null>(null);
  const frameRef = useRef<number | null>(null);
  const timerRef = useRef<number | null>(null);

  useLayoutEffect(() => {
    pendingSnapshotRef.current = currentSnapshot;
  }, [currentSnapshot]);

  useLayoutEffect(() => {
    const nextSnapshot = pendingSnapshotRef.current;
    const hasNewSnapshot = Boolean(currentSnapshotKey);
    const hasVisibleSnapshot = Boolean(displayedSnapshotKeyRef.current);
    const hasChangedSnapshot
      = hasVisibleSnapshot
        && Boolean(currentSnapshotKey)
        && displayedSnapshotKeyRef.current !== currentSnapshotKey;

    if (frameRef.current !== null) {
      window.cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    if (!hasNewSnapshot) {
      setIsVisible(false);
      setDisplayedSnapshot(null);
      displayedSnapshotKeyRef.current = null;
      return;
    }

    if (!hasVisibleSnapshot || !hasChangedSnapshot) {
      setDisplayedSnapshot(nextSnapshot);
      displayedSnapshotKeyRef.current = currentSnapshotKey;
      setIsVisible(false);

      frameRef.current = window.requestAnimationFrame(() => {
        setIsVisible(true);
        frameRef.current = null;
      });
      return;
    }

    setIsVisible(false);

    timerRef.current = window.setTimeout(() => {
      setDisplayedSnapshot(nextSnapshot);
      displayedSnapshotKeyRef.current = currentSnapshotKey;

      timerRef.current = window.setTimeout(() => {
        frameRef.current = window.requestAnimationFrame(() => {
          setIsVisible(true);
          frameRef.current = null;
        });
        timerRef.current = null;
      }, fadeInDelayMs);
    }, fadeOutMs);
  }, [currentSnapshotKey, fadeInDelayMs, fadeOutMs]);

  useLayoutEffect(() => {
    if (
      currentSnapshot
      && displayedSnapshotKeyRef.current === currentSnapshotKey
    ) {
      setDisplayedSnapshot(currentSnapshot);
    }
  }, [currentSnapshot, currentSnapshotKey]);

  useEffect(() => {
    return () => {
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
      }
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, []);

  return {
    displayedSnapshot,
    isVisible,
  };
}
