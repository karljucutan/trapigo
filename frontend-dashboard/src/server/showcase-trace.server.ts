import type { RequestResult } from "../lib/request-flow";

const TRACE_TTL_MS = 30 * 60_000;
const MAX_TRACE_QUERIES = 10;
const TRACE_QUERY_INTERVAL_MS = 500;

type ActiveTrace = {
  id: string;
  expiresAt: number;
  queries: number;
  lastQueryAt: number | null;
  result: RequestResult | null;
};

let activeTrace: ActiveTrace | null = null;

export function registerShowcaseTrace(traceId: string, now = Date.now()): void {
  activeTrace = {
    id: traceId,
    expiresAt: now + TRACE_TTL_MS,
    queries: 0,
    lastQueryAt: null,
    result: null,
  };
}

export function isActiveShowcaseTrace(
  traceId: string,
  now = Date.now(),
): boolean {
  if (activeTrace && activeTrace.expiresAt <= now) activeTrace = null;
  return activeTrace?.id === traceId;
}

export function saveShowcaseResult(
  result: RequestResult,
  now = Date.now(),
): boolean {
  if (!isActiveShowcaseTrace(result.traceId, now) || !activeTrace) return false;
  activeTrace.result = result;
  return true;
}

export function getShowcaseResult(
  traceId: string,
  now = Date.now(),
): RequestResult | null {
  return isActiveShowcaseTrace(traceId, now)
    ? (activeTrace?.result ?? null)
    : null;
}

export function consumeTraceQuery(
  traceId: string,
  now = Date.now(),
): number | null {
  if (!isActiveShowcaseTrace(traceId, now) || !activeTrace) return null;
  if (
    activeTrace.queries >= MAX_TRACE_QUERIES ||
    (activeTrace.lastQueryAt !== null &&
      now - activeTrace.lastQueryAt < TRACE_QUERY_INTERVAL_MS)
  ) {
    return null;
  }
  activeTrace.queries += 1;
  activeTrace.lastQueryAt = now;
  return activeTrace.queries;
}
