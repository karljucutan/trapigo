const QUERY_TIMEOUT_MS = 3_000;
const MAX_TRACE_RESPONSE_BYTES = 1_048_576;

function jaegerOrigin(): string {
  const configured =
    process.env.JAEGER_QUERY_ORIGIN ??
    (process.env.NODE_ENV === "production" ? "" : "http://localhost:16686");
  if (!configured) {
    throw new Error("JAEGER_QUERY_ORIGIN is required in production");
  }
  const url = new URL(configured);
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  ) {
    throw new Error("JAEGER_QUERY_ORIGIN must be an HTTP(S) origin");
  }
  if (process.env.NODE_ENV === "production" && url.protocol !== "https:") {
    throw new Error("JAEGER_QUERY_ORIGIN must use HTTPS in production");
  }
  return url.origin;
}

async function readLimited(response: Response): Promise<unknown> {
  if (!response.body) throw new Error("Jaeger trace response has no body");
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      total += value.byteLength;
      if (total > MAX_TRACE_RESPONSE_BYTES) {
        await reader.cancel(
          "Jaeger trace response exceeds the configured limit",
        );
        throw new Error("Jaeger trace response exceeds the configured limit");
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  try {
    return JSON.parse(new TextDecoder().decode(bytes)) as unknown;
  } catch {
    throw new Error("Jaeger returned invalid JSON");
  }
}

export async function getJaegerTrace(
  traceId: string,
  request: typeof fetch = fetch,
): Promise<unknown | null> {
  const url = new URL(
    `/api/v3/traces/${encodeURIComponent(traceId)}`,
    jaegerOrigin(),
  );
  const response = await request(url, {
    method: "GET",
    headers: { Accept: "application/json" },
    cache: "no-store",
    redirect: "error",
    signal: AbortSignal.timeout(QUERY_TIMEOUT_MS),
  });
  if (response.status === 404) {
    await response.body?.cancel();
    return null;
  }
  if (!response.ok) {
    const status = response.status;
    await response.body?.cancel();
    throw new Error(`Jaeger returned status ${status}`);
  }
  return readLimited(response);
}
