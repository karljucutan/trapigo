import {
  httpStatus,
  type RequestResult,
  retryAfterMs,
  retryAttempts,
  traceId,
} from "./request-flow";

const REQUEST_TIMEOUT_MS = 10_000;
const MAX_RESPONSE_BYTES = 64 * 1024;
const MAX_RETRY_AFTER_MS = 300_000;

function randomHex(byteLength: number): string {
  const bytes = new Uint8Array(byteLength);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join(
    "",
  );
}

function parseRetryAfter(value: string | null, now = Date.now()) {
  if (!value) return null;
  const trimmed = value.trim();
  let milliseconds: number;
  if (/^\d+$/.test(trimmed)) {
    milliseconds = Number(trimmed) * 1_000;
  } else {
    const timestamp = Date.parse(trimmed);
    if (!Number.isFinite(timestamp)) return null;
    milliseconds = Math.max(0, timestamp - now);
  }
  if (!Number.isFinite(milliseconds)) milliseconds = MAX_RETRY_AFTER_MS;
  return retryAfterMs(Math.min(Math.max(0, milliseconds), MAX_RETRY_AFTER_MS));
}

async function consumeBoundedBody(response: Response) {
  if (!response.body) return "complete" as const;
  const reader = response.body.getReader();
  let consumed = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) return "complete" as const;
      consumed += value.byteLength;
      if (consumed > MAX_RESPONSE_BYTES) {
        await reader.cancel("Orders response exceeded the configured limit");
        return "too-large" as const;
      }
    }
  } finally {
    reader.releaseLock();
  }
}

function gatewayOrigin(): string {
  const configured =
    import.meta.env.VITE_TRAPIGO_GATEWAY_ORIGIN ??
    (import.meta.env.DEV
      ? "http://localhost:8081"
      : globalThis.location.origin);
  const origin = new URL(configured);
  if (
    !["http:", "https:"].includes(origin.protocol) ||
    origin.username ||
    origin.password ||
    origin.pathname !== "/" ||
    origin.search ||
    origin.hash
  ) {
    throw new Error("VITE_TRAPIGO_GATEWAY_ORIGIN must be an HTTP(S) origin");
  }
  if (import.meta.env.PROD && origin.protocol !== "https:") {
    throw new Error("VITE_TRAPIGO_GATEWAY_ORIGIN must use HTTPS in production");
  }
  return origin.origin;
}

export async function requestOrdersDirectly(): Promise<{
  result: RequestResult;
  durationMs: number;
}> {
  let id: ReturnType<typeof traceId>;
  do {
    id = traceId(randomHex(16));
  } while (/^0{32}$/.test(id));
  let parentId: string;
  do {
    parentId = randomHex(8);
  } while (/^0{16}$/.test(parentId));

  const url = new URL("/api/v1/go-orders", gatewayOrigin());
  const startedAt = performance.now();
  let result: RequestResult;
  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        traceparent: `00-${id}-${parentId}-01`,
      },
      credentials: "include",
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
    });
    const retry = {
      attempts: retryAttempts(0),
      retryAfterMs: parseRetryAfter(response.headers.get("retry-after")),
    };
    try {
      const body = await consumeBoundedBody(response);
      result =
        body === "complete"
          ? {
              kind: "http-response",
              traceId: id,
              statusCode: httpStatus(response.status),
              retry,
            }
          : {
              kind: "response-body-failure",
              traceId: id,
              statusCode: httpStatus(response.status),
              failure: "too-large",
              retry,
            };
    } catch (error) {
      result = {
        kind: "response-body-failure",
        traceId: id,
        statusCode: httpStatus(response.status),
        failure:
          error instanceof Error &&
          (error.name === "TimeoutError" || error.name === "AbortError")
            ? "timed-out"
            : "read-failed",
        retry,
      };
    }
  } catch (error) {
    const failure =
      error instanceof Error &&
      (error.name === "TimeoutError" || error.name === "AbortError")
        ? "timed-out"
        : "connection-failed";
    console.error(
      "Direct orders request failed before an HTTP response",
      error,
    );
    result = {
      kind: "connection-failure",
      traceId: id,
      failure,
      retry: { attempts: retryAttempts(0), retryAfterMs: null },
    };
  }
  const durationMs = Number((performance.now() - startedAt).toFixed(1));
  return { result, durationMs };
}
