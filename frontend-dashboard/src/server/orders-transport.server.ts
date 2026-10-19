import { randomBytes } from "node:crypto";
import { traceId } from "../lib/request-flow.ts";
import {
  authCookieHeader,
  getAuthConfig,
  rotatedAuthCookies,
} from "./auth-gateway.server.ts";

const REQUEST_TIMEOUT_MS = 10_000;
const MAX_RESPONSE_BYTES = 64 * 1024;
const MAX_RETRY_AFTER_MS = 300_000;

export type OrdersTransportResult =
  | {
      kind: "http-response";
      statusCode: number;
      retryAfterMs: number | null;
      bodyComplete: boolean;
      bodyFailure: "too-large" | "read-failed" | "timed-out" | null;
    }
  | {
      kind: "connection-failure";
      failure: "connection-failed" | "timed-out";
      retryAfterMs: null;
    };

type TransportOptions = {
  traceId: string;
  request?: typeof fetch;
  timeoutMs?: number;
  maxResponseBytes?: number;
};

export function parseRetryAfter(
  value: string | null,
  now = Date.now(),
): number | null {
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
  if (!Number.isFinite(milliseconds)) return MAX_RETRY_AFTER_MS;
  return Math.min(Math.max(0, milliseconds), MAX_RETRY_AFTER_MS);
}

async function consumeBoundedBody(
  response: Response,
  maxBytes: number,
): Promise<"complete" | "too-large"> {
  if (!response.body) return "complete";
  const reader = response.body.getReader();
  let consumed = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) return "complete";
      consumed += value.byteLength;
      if (consumed > maxBytes) {
        await reader.cancel("Orders response exceeded the configured limit");
        return "too-large";
      }
    }
  } finally {
    reader.releaseLock();
  }
}

export async function requestOrders(
  cookieHeader: string,
  options: TransportOptions,
): Promise<{
  result: OrdersTransportResult;
  cookies: string[];
}> {
  const request = options.request ?? fetch;
  const url = new URL("/api/v1/go-orders", getOrdersGatewayOrigin());
  const headers = new Headers();
  const cookies = authCookieHeader(cookieHeader);
  if (cookies) headers.set("cookie", cookies);
  let parentId: string;
  do {
    parentId = randomBytes(8).toString("hex");
  } while (/^0{16}$/.test(parentId));
  // W3C correlation only: no browser or BFF span is recorded or exported.
  headers.set("traceparent", `00-${traceId(options.traceId)}-${parentId}-01`);

  let response: Response;
  try {
    response = await request(url, {
      method: "GET",
      headers,
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(options.timeoutMs ?? REQUEST_TIMEOUT_MS),
    });
  } catch (error) {
    const failure =
      error instanceof Error &&
      (error.name === "TimeoutError" || error.name === "AbortError")
        ? "timed-out"
        : "connection-failed";
    console.error("Orders request failed before an HTTP response", failure);
    return {
      result: { kind: "connection-failure", failure, retryAfterMs: null },
      cookies: [],
    };
  }

  const rotatedCookies = rotatedAuthCookies(response.headers);
  let bodyResult: "complete" | "too-large";
  try {
    bodyResult = await consumeBoundedBody(
      response,
      options.maxResponseBytes ?? MAX_RESPONSE_BYTES,
    );
  } catch (error) {
    console.error(
      "Orders response body could not be consumed",
      error instanceof Error ? error.name : "unknown",
    );
    return {
      result: {
        kind: "http-response",
        statusCode: response.status,
        retryAfterMs: parseRetryAfter(response.headers.get("retry-after")),
        bodyComplete: false,
        bodyFailure:
          error instanceof Error &&
          (error.name === "TimeoutError" || error.name === "AbortError")
            ? "timed-out"
            : "read-failed",
      },
      cookies: rotatedCookies,
    };
  }

  return {
    result: {
      kind: "http-response",
      statusCode: response.status,
      retryAfterMs: parseRetryAfter(response.headers.get("retry-after")),
      bodyComplete: bodyResult === "complete",
      bodyFailure: bodyResult === "too-large" ? "too-large" : null,
    },
    cookies: rotatedCookies,
  };
}

function getOrdersGatewayOrigin(): string {
  const configured =
    process.env.TRAPIGO_GATEWAY_ORIGIN ??
    process.env.AUTH_GATEWAY_ORIGIN ??
    getAuthConfig().gatewayOrigin;
  const origin = new URL(configured);
  if (
    !["http:", "https:"].includes(origin.protocol) ||
    origin.username ||
    origin.password ||
    origin.pathname !== "/" ||
    origin.search ||
    origin.hash
  ) {
    throw new Error("TRAPIGO_GATEWAY_ORIGIN must be an HTTP(S) origin");
  }
  if (process.env.NODE_ENV === "production" && origin.protocol !== "https:") {
    throw new Error("TRAPIGO_GATEWAY_ORIGIN must use HTTPS in production");
  }
  return origin.origin;
}
