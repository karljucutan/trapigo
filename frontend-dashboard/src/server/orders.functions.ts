import { randomBytes } from "node:crypto";
import { createServerFn } from "@tanstack/react-start";
import { getRequestHeader } from "@tanstack/react-start/server";
import {
  httpStatus,
  traceId as makeTraceId,
  type RequestResult,
  retryAfterMs,
  retryAttempts,
} from "../lib/request-flow";
import { forwardAuthCookies, setPrivateNoStoreHeaders } from "./auth.server";
import { requestOrders } from "./orders-transport.server";
import {
  registerShowcaseTrace,
  saveShowcaseResult,
} from "./showcase-trace.server";

function randomTraceId(): string {
  let value: string;
  do {
    value = randomBytes(16).toString("hex");
  } while (/^0{32}$/.test(value));
  return value;
}

export const sendOrdersRequest = createServerFn({ method: "GET" }).handler(
  async (): Promise<RequestResult> => {
    setPrivateNoStoreHeaders();
    const cookieHeader = getRequestHeader("cookie") ?? "";

    const traceId = makeTraceId(randomTraceId());
    registerShowcaseTrace(traceId);
    const response = await requestOrders(cookieHeader, { traceId });
    forwardAuthCookies(response.cookies);

    const retry = {
      attempts: retryAttempts(0),
      retryAfterMs:
        response.result.retryAfterMs === null
          ? null
          : retryAfterMs(response.result.retryAfterMs),
    };
    let result: RequestResult;
    if (response.result.kind === "http-response") {
      result =
        response.result.bodyFailure === null
          ? {
              kind: "http-response",
              traceId,
              statusCode: httpStatus(response.result.statusCode),
              retry,
            }
          : {
              kind: "response-body-failure",
              traceId,
              statusCode: httpStatus(response.result.statusCode),
              failure: response.result.bodyFailure,
              retry,
            };
    } else {
      result = {
        kind: "connection-failure",
        traceId,
        failure: response.result.failure,
        retry,
      };
    }
    if (!saveShowcaseResult(result)) {
      throw new Error("The active demo request was replaced or expired");
    }
    return result;
  },
);
