import { createServerFn } from "@tanstack/react-start";
import {
  httpStatus,
  traceId as makeTraceId,
  type RequestResult,
  retryAfterMs,
  retryAttempts,
} from "../lib/request-flow";
import { getCurrentUser, setPrivateNoStoreHeaders } from "./auth.server";
import {
  registerShowcaseTrace,
  saveShowcaseResult,
} from "./showcase-trace.server";

function parseDirectResult(value: unknown): RequestResult {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error("Invalid direct request result");
  }
  const input = value as Record<string, unknown>;
  const traceId = makeTraceId(
    typeof input.traceId === "string" ? input.traceId : "",
  );
  if (input.kind === "connection-failure") {
    if (
      input.failure !== "connection-failed" &&
      input.failure !== "timed-out"
    ) {
      throw new Error("Invalid direct connection failure");
    }
    return {
      kind: "connection-failure",
      traceId,
      failure: input.failure,
      retry: { attempts: retryAttempts(0), retryAfterMs: null },
    };
  }

  if (input.kind !== "http-response") {
    if (input.kind !== "response-body-failure") {
      throw new Error("Invalid direct HTTP result");
    }
  }
  const statusCode = httpStatus(
    typeof input.statusCode === "number" ? input.statusCode : Number.NaN,
  );
  const retryInput =
    typeof input.retry === "object" && input.retry !== null
      ? (input.retry as Record<string, unknown>)
      : null;
  const after = retryInput?.retryAfterMs;
  if (after !== null && typeof after !== "number") {
    throw new Error("Invalid direct retry information");
  }
  const retry = {
    attempts: retryAttempts(0),
    retryAfterMs: after === null ? null : retryAfterMs(after),
  };
  if (input.kind === "http-response") {
    return { kind: "http-response", traceId, statusCode, retry };
  }

  if (
    input.failure !== "too-large" &&
    input.failure !== "read-failed" &&
    input.failure !== "timed-out"
  ) {
    throw new Error("Invalid direct response body failure");
  }
  return {
    kind: "response-body-failure",
    traceId,
    statusCode,
    failure: input.failure,
    retry,
  };
}

export const recordDirectShowcaseResult = createServerFn({ method: "POST" })
  .validator(parseDirectResult)
  .handler(async ({ data }) => {
    setPrivateNoStoreHeaders();
    if (!(await getCurrentUser())) {
      throw new Error("Authentication is required to record a direct request");
    }
    registerShowcaseTrace(data.traceId);
    if (!saveShowcaseResult(data)) {
      throw new Error("Could not record the direct request result");
    }
  });
