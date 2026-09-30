import { useEffect, useState } from "react";

// useNow re-renders every `ms` with the current timestamp — countdown timers.
export function useNow(ms: number): number {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    if (ms <= 0) return; // no ticking needed
    const t = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(t);
  }, [ms]);
  return now;
}
