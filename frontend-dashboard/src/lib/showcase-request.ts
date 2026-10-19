import { recordDirectShowcaseResult } from "../server/direct-trace.functions";
import { sendOrdersRequest } from "../server/orders.functions";
import { getShowcaseTrace } from "../server/trace.functions";
import { requestOrdersDirectly } from "./direct-orders-request";

export async function runShowcaseRequest() {
  const startedAt = performance.now();
  const result = await sendOrdersRequest();
  const durationMs = Number((performance.now() - startedAt).toFixed(1));
  return {
    result,
    durationMs,
    readTrace: () => getShowcaseTrace({ data: result.traceId }),
  };
}

export async function runDirectShowcaseRequest() {
  const { result, durationMs } = await requestOrdersDirectly();
  let traceRecorded = true;
  try {
    await recordDirectShowcaseResult({ data: result });
  } catch (error) {
    traceRecorded = false;
    console.error("Could not register direct request trace", error);
  }
  return {
    result,
    durationMs,
    readTrace: () =>
      traceRecorded && result.kind !== "connection-failure"
        ? getShowcaseTrace({ data: result.traceId })
        : Promise.resolve({
            status: "unavailable" as const,
            reason: "query-failed" as const,
          }),
  };
}
