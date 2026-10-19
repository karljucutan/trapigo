import { createServerFn } from "@tanstack/react-start";
import { traceId as makeTraceId } from "../lib/request-flow";
import { normalizeJaegerTrace } from "../lib/trace-normalizer";
import { getCurrentUser, setPrivateNoStoreHeaders } from "./auth.server";
import { getJaegerTrace } from "./jaeger.server";
import {
  consumeTraceQuery,
  getShowcaseResult,
  isActiveShowcaseTrace,
} from "./showcase-trace.server";

const ZERO_TRACE_ID = /^0{32}$/;

export const getShowcaseTrace = createServerFn({ method: "GET" })
  .validator((value: string) => {
    const id = makeTraceId(value);
    if (ZERO_TRACE_ID.test(id)) throw new Error("Invalid trace ID");
    return id;
  })
  .handler(async ({ data }) => {
    setPrivateNoStoreHeaders();
    const requestedTraceId = String(data);
    if (!isActiveShowcaseTrace(requestedTraceId)) {
      throw new Error("Trace is unavailable");
    }

    const queryNumber = consumeTraceQuery(requestedTraceId);
    if (queryNumber === null) {
      throw new Error("Trace query limit reached");
    }
    const user = await getCurrentUser();
    if (!user) return { status: "authentication-required" as const };
    if (!isActiveShowcaseTrace(requestedTraceId)) {
      throw new Error("Trace is unavailable");
    }
    const result = getShowcaseResult(requestedTraceId);
    if (!result) return { status: "waiting-for-trace" as const };

    try {
      const payload = await getJaegerTrace(requestedTraceId);
      if (payload === null) {
        return queryNumber >= 10
          ? { status: "unavailable" as const, reason: "not-found" as const }
          : { status: "waiting-for-trace" as const };
      }
      const normalized = normalizeJaegerTrace(
        payload,
        requestedTraceId,
        result,
      );
      if (normalized.status === "not-found") {
        return queryNumber >= 10
          ? { status: "unavailable" as const, reason: "not-found" as const }
          : { status: "waiting-for-trace" as const };
      }
      return normalized;
    } catch (error) {
      if (
        error instanceof Error &&
        (error.name === "TimeoutError" || error.name === "AbortError")
      ) {
        return { status: "unavailable" as const, reason: "timed-out" as const };
      }
      console.error("Authorized Jaeger trace lookup failed");
      return {
        status: "unavailable" as const,
        reason: "query-failed" as const,
      };
    }
  });
