import { useEffect, useMemo, useState } from "react";

import { parseTime } from "../utils/time";

export function useElapsedSeconds(startTime: unknown, active: boolean): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!active) {
      return;
    }

    setNow(Date.now());
    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, 1000);

    return () => window.clearInterval(timer);
  }, [active, startTime]);

  return useMemo(() => {
    if (!active || !startTime) {
      return 0;
    }

    const start = parseTime(startTime).getTime();
    if (!Number.isFinite(start)) {
      return 0;
    }

    return Math.max(0, Math.floor((now - start) / 1000));
  }, [active, now, startTime]);
}
