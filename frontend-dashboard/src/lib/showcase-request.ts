import { sendOrdersRequest } from "../server/orders.functions";
import { getShowcaseTrace } from "../server/trace.functions";

export async function runShowcaseRequest() {
  const result = await sendOrdersRequest();
  return {
    result,
    readTrace: () => getShowcaseTrace({ data: result.traceId }),
  };
}
