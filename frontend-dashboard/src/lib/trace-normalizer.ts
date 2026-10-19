import {
  correlateTraceEvidence,
  type DeliveryEvidence,
  MAX_SPAN_DURATION_MS,
  type PolicyEvidence,
  type PolicyName,
  policyEvidence,
  publicReplicaId,
  type ReplicaSelection,
  type RequestResult,
  type SpanRole,
  spanDurationMs,
  spanId,
  type TraceEvidence,
  type TraceSpan,
  traceSpans,
} from "./request-flow.ts";

const MAX_RAW_SPANS = 256;
const MAX_TRACE_DURATION_NS = BigInt(MAX_SPAN_DURATION_MS) * 1_000_000n;
const ZERO_TRACE_ID = /^0{32}$/;
const ZERO_SPAN_ID = /^0{16}$/;
const EVIDENCE_ATTRIBUTES = new Set([
  "trapigo.policy.outcome",
  "trapigo.backend.replica",
  "http.response.status_code",
  "http.status_code",
]);
const POLICY_NAMES: Record<string, PolicyName> = {
  "gateway.rate_limit": "rate-limit",
  "gateway.origin_check": "csrf-origin",
  "gateway.csrf_check": "csrf-origin",
  "gateway.authentication": "authentication",
  "gateway.route_match": "route-matching",
  "gateway.backend_selection": "replica-selection",
};

type RawSpan = {
  traceId: string;
  spanId: string;
  parentSpanId: string;
  name: string;
  kind: string | number;
  startTimeUnixNano: string;
  endTimeUnixNano: string;
  attributes: unknown;
};

type ProducerSpan = {
  span: RawSpan;
  service: string;
  role: SpanRole;
  attributes: Map<string, unknown>;
  startNs: bigint;
  endNs: bigint;
};

export type NormalizedTrace =
  | { status: "not-found" }
  | { status: "partial"; trace: TraceEvidence }
  | { status: "available"; trace: TraceEvidence };

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function getRecord(
  value: unknown,
  key: string,
): Record<string, unknown> | null {
  return record(record(value)?.[key]);
}

function stringValue(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function stringField(value: unknown, key: string): string | null {
  return stringValue(record(value)?.[key]);
}

function positiveNanos(value: unknown): bigint | null {
  if (typeof value !== "string" || !/^\d{1,24}$/.test(value)) return null;
  try {
    const parsed = BigInt(value);
    return parsed > 0n ? parsed : null;
  } catch {
    return null;
  }
}

function spanAttributes(value: unknown): Map<string, unknown> {
  if (!Array.isArray(value) || value.length > 64) {
    throw new Error("Jaeger returned malformed span attributes");
  }
  const attributes = new Map<string, unknown>();
  for (const item of value) {
    const key = stringField(item, "key");
    if (!key) throw new Error("Jaeger returned a malformed attribute key");
    if (!EVIDENCE_ATTRIBUTES.has(key)) continue;
    const typedValue = record(record(item)?.value);
    if (!key || !typedValue || attributes.has(key)) {
      throw new Error("Jaeger returned duplicate or malformed attributes");
    }
    const valueFields = [
      "stringValue",
      "intValue",
      "boolValue",
      "doubleValue",
    ].filter((field) => Object.hasOwn(typedValue, field));
    const valueField = valueFields[0];
    const supportedValue =
      valueFields.length === 1 && valueField !== undefined
        ? typedValue[valueField]
        : undefined;
    if (
      !(
        typeof supportedValue === "string" ||
        typeof supportedValue === "number" ||
        typeof supportedValue === "boolean"
      )
    ) {
      throw new Error("Jaeger returned an unsupported attribute value");
    }
    attributes.set(key, supportedValue);
  }
  return attributes;
}

function resourceServiceName(value: unknown): string | null {
  const attributes = record(value)?.attributes;
  if (!Array.isArray(attributes) || attributes.length > 64) return null;
  let serviceName: string | null = null;
  let foundServiceName = false;
  for (const item of attributes) {
    if (stringField(item, "key") !== "service.name") continue;
    if (foundServiceName) {
      throw new Error("Jaeger returned duplicate service names");
    }
    foundServiceName = true;
    serviceName = stringField(record(item)?.value, "stringValue");
    if (!serviceName)
      throw new Error("Jaeger returned an invalid service name");
  }
  return serviceName;
}

function spanRole(service: string, name: string): SpanRole | null {
  if (
    service === "trapigo-gateway" &&
    (name === "GET /" || name === "gateway")
  ) {
    return "gateway-request";
  }
  if (
    service === "trapigo-gateway" &&
    [
      "gateway.rate_limit",
      "gateway.origin_check",
      "gateway.csrf_check",
      "gateway.authentication",
      "gateway.route_match",
      "gateway.backend_selection",
    ].includes(name)
  ) {
    return "gateway-policy";
  }
  if (service === "trapigo-gateway" && name === "HTTP GET") {
    return "gateway-proxy";
  }
  if (
    service === "go-order-service" &&
    (name === "GET /api/v1/go-orders" || name === "orders.request")
  ) {
    return "orders-service";
  }
  return null;
}

function expectedKind(role: SpanRole, kind: string | number): boolean {
  const expected =
    role === "gateway-policy"
      ? ["SPAN_KIND_INTERNAL", 1]
      : role === "gateway-proxy"
        ? ["SPAN_KIND_CLIENT", 3]
        : ["SPAN_KIND_SERVER", 2];
  return expected.includes(kind);
}

function spansFromJaeger(
  payload: unknown,
  expectedTraceId: string,
): { spans: ProducerSpan[]; rawCount: number } {
  const root = getRecord(payload, "result");
  const resourceSpans = root?.resourceSpans;
  if (!Array.isArray(resourceSpans) || resourceSpans.length > 32) {
    throw new Error("Jaeger returned an invalid trace envelope");
  }

  const output: ProducerSpan[] = [];
  const allSpanIds = new Set<string>();
  const parentById = new Map<string, string>();
  let rawCount = 0;
  for (const resourceSpan of resourceSpans) {
    const service = resourceServiceName(record(resourceSpan)?.resource);
    const scopes = record(resourceSpan)?.scopeSpans;
    if (!Array.isArray(scopes) || scopes.length > 32) {
      throw new Error("Jaeger returned malformed resource spans");
    }
    for (const scope of scopes) {
      const spans = record(scope)?.spans;
      if (!Array.isArray(spans)) {
        throw new Error("Jaeger returned malformed scope spans");
      }
      rawCount += spans.length;
      if (rawCount > MAX_RAW_SPANS) {
        throw new Error("Jaeger trace exceeds the supported span limit");
      }
      for (const candidate of spans) {
        const span = record(candidate);
        if (!span) throw new Error("Jaeger returned a malformed span");
        const id = stringField(span, "spanId");
        const trace = stringField(span, "traceId");
        if (!id || !/^[\da-f]{16}$/i.test(id) || ZERO_SPAN_ID.test(id)) {
          throw new Error("Jaeger returned an invalid span ID");
        }
        if (!trace || trace.toLowerCase() !== expectedTraceId.toLowerCase()) {
          throw new Error("Jaeger returned a span for a different trace");
        }
        if (allSpanIds.has(id.toLowerCase())) {
          throw new Error("Jaeger returned duplicate span IDs");
        }
        allSpanIds.add(id.toLowerCase());
        const parentSpanId = stringField(span, "parentSpanId") ?? "";
        if (
          parentSpanId &&
          (!/^[\da-f]{16}$/i.test(parentSpanId) ||
            ZERO_SPAN_ID.test(parentSpanId))
        ) {
          throw new Error("Jaeger returned an invalid parent span ID");
        }
        parentById.set(id.toLowerCase(), parentSpanId.toLowerCase());
        if (!service) continue;
        const name = stringField(span, "name");
        const role = name ? spanRole(service, name) : null;
        if (!role) continue;
        const kind = span.kind;
        if (
          !(typeof kind === "string" || typeof kind === "number") ||
          !expectedKind(role, kind)
        ) {
          throw new Error("Jaeger returned an unexpected span kind");
        }
        const startNs = positiveNanos(span.startTimeUnixNano);
        const endNs = positiveNanos(span.endTimeUnixNano);
        if (!startNs || !endNs || endNs < startNs) {
          throw new Error("Jaeger returned invalid span timestamps");
        }
        const rawSpan: RawSpan = {
          traceId: trace,
          spanId: id,
          parentSpanId,
          name: name ?? "",
          kind,
          startTimeUnixNano: stringField(span, "startTimeUnixNano") ?? "",
          endTimeUnixNano: stringField(span, "endTimeUnixNano") ?? "",
          attributes: span.attributes ?? [],
        };
        output.push({
          span: rawSpan,
          service,
          role,
          attributes: spanAttributes(span.attributes ?? []),
          startNs,
          endNs,
        });
      }
    }
  }
  for (const id of allSpanIds) {
    const seen = new Set<string>([id]);
    let parent = parentById.get(id);
    while (parent && allSpanIds.has(parent)) {
      if (seen.has(parent))
        throw new Error("Jaeger trace contains a span cycle");
      seen.add(parent);
      parent = parentById.get(parent);
    }
  }
  return { spans: output, rawCount };
}

function duration(producer: ProducerSpan): TraceSpan["duration"] {
  const nanos = producer.endNs - producer.startNs;
  if (nanos > MAX_TRACE_DURATION_NS) return null;
  return {
    valueMs: spanDurationMs(Number(nanos) / 1_000_000),
    boundary:
      producer.role === "gateway-request"
        ? "gateway-server-request"
        : producer.role === "gateway-policy"
          ? "policy-execution"
          : producer.role === "gateway-proxy"
            ? "proxy-outbound-through-response"
            : "service-server-request",
  };
}

function policyOutcomes(spans: ProducerSpan[]): PolicyEvidence[] {
  const values: PolicyEvidence[] = [];
  const csrf = new Map<string, "passed" | "rejected">();
  let selection: "selected" | "not-selected" | null = null;
  for (const producer of spans) {
    if (producer.role !== "gateway-policy") continue;
    const outcome = producer.attributes.get("trapigo.policy.outcome");
    switch (producer.span.name) {
      case "gateway.rate_limit":
        if (outcome === "disabled") {
          values.push({ policy: "rate-limit", outcome: "disabled" });
        } else if (outcome === "admitted") {
          values.push({ policy: "rate-limit", outcome: "passed" });
        } else if (outcome === "rejected") {
          values.push({ policy: "rate-limit", outcome: "rejected" });
        }
        break;
      case "gateway.origin_check":
      case "gateway.csrf_check":
        if (outcome === "passed" || outcome === "rejected") {
          csrf.set(producer.span.name, outcome);
        }
        break;
      case "gateway.authentication":
        if (outcome === "authenticated") {
          values.push({ policy: "authentication", outcome: "passed" });
        } else if (outcome === "rejected") {
          values.push({ policy: "authentication", outcome: "rejected" });
        }
        break;
      case "gateway.route_match":
        if (outcome === "matched") {
          values.push({ policy: "route-matching", outcome: "matched" });
        } else if (outcome === "not_matched") {
          values.push({ policy: "route-matching", outcome: "not-matched" });
        }
        break;
      case "gateway.backend_selection":
        if (outcome === "selected") selection = "selected";
        else if (outcome === "unavailable") selection = "not-selected";
        break;
    }
  }
  if (
    csrf.get("gateway.origin_check") === "rejected" ||
    csrf.get("gateway.csrf_check") === "rejected"
  ) {
    values.push({ policy: "csrf-origin", outcome: "rejected" });
  } else if (
    csrf.get("gateway.origin_check") === "passed" &&
    csrf.get("gateway.csrf_check") === "passed"
  ) {
    values.push({
      policy: "csrf-origin",
      outcome: "passed",
    });
  }
  if (selection) {
    values.push({
      policy: "replica-selection",
      outcome: selection === "selected" ? "selected" : "not-selected",
    });
  }
  return values;
}

function statusCode(producer: ProducerSpan): number | null {
  const value =
    producer.attributes.get("http.response.status_code") ??
    producer.attributes.get("http.status_code");
  const numeric = typeof value === "string" ? Number(value) : value;
  return typeof numeric === "number" && Number.isInteger(numeric)
    ? numeric
    : null;
}

export function normalizeJaegerTrace(
  payload: unknown,
  expectedTraceId: string,
  result: RequestResult,
): NormalizedTrace {
  if (
    !/^[\da-f]{32}$/i.test(expectedTraceId) ||
    ZERO_TRACE_ID.test(expectedTraceId)
  ) {
    throw new Error("Invalid expected trace ID");
  }
  if (result.traceId !== expectedTraceId.toLowerCase()) {
    throw new Error("Request result does not match the requested trace");
  }
  const parsed = spansFromJaeger(payload, expectedTraceId);
  if (parsed.rawCount === 0) return { status: "not-found" };
  const producers = parsed.spans;
  if (producers.length === 0) {
    return {
      status: "partial",
      trace: correlateTraceEvidence(result, {
        spans: traceSpans([]),
        policies: policyEvidence([]),
        replicaSelection: { status: "unknown" },
        delivery: { status: "unknown" },
      }),
    };
  }

  const gateway = producers.find(
    (producer) => producer.role === "gateway-request",
  );
  const spans: TraceSpan[] = producers.map((producer) => ({
    spanId: spanId(producer.span.spanId),
    // The gateway is the root of the displayed scope; its correlation-only
    // remote parent is deliberately not a recorded span.
    parentSpanId:
      producer.role !== "gateway-request" && producer.span.parentSpanId
        ? spanId(producer.span.parentSpanId)
        : null,
    role: producer.role,
    ...(producer.role === "gateway-policy"
      ? { policy: POLICY_NAMES[producer.span.name] }
      : {}),
    duration: duration(producer),
  }));
  const gatewayPolicies = producers.filter(
    (producer) =>
      producer.role === "gateway-policy" &&
      gateway !== undefined &&
      producer.span.parentSpanId.toLowerCase() ===
        gateway.span.spanId.toLowerCase(),
  );
  const selectionSpan = gatewayPolicies.find(
    (producer) =>
      producer.role === "gateway-policy" &&
      producer.span.name === "gateway.backend_selection" &&
      producer.attributes.get("trapigo.policy.outcome") === "selected",
  );
  const replicaLabel = selectionSpan?.attributes.get("trapigo.backend.replica");
  let replicaSelection: ReplicaSelection = { status: "unknown" };
  if (typeof replicaLabel === "string") {
    try {
      replicaSelection = {
        status: "selected",
        replicaId: publicReplicaId(replicaLabel),
      };
    } catch {
      replicaSelection = { status: "unknown" };
    }
  } else if (
    gatewayPolicies.some(
      (producer) =>
        producer.role === "gateway-policy" &&
        producer.span.name === "gateway.backend_selection" &&
        producer.attributes.get("trapigo.policy.outcome") === "unavailable",
    )
  ) {
    replicaSelection = { status: "not-selected" };
  }

  const proxy = producers.find(
    (producer) =>
      producer.role === "gateway-proxy" &&
      gateway !== undefined &&
      producer.span.parentSpanId.toLowerCase() ===
        gateway.span.spanId.toLowerCase(),
  );
  const service = producers.find(
    (producer) => producer.role === "orders-service",
  );
  let delivery: DeliveryEvidence = { status: "unknown" };
  if (proxy && service) {
    const descendant =
      service.span.parentSpanId.toLowerCase() ===
      proxy.span.spanId.toLowerCase();
    const replica =
      replicaSelection.status === "selected"
        ? replicaSelection.replicaId
        : null;
    delivery =
      descendant && replica
        ? { status: "delivered", replicaId: replica }
        : { status: "unknown" };
  } else if (proxy && (statusCode(proxy) ?? 0) >= 500) {
    delivery = { status: "failed" };
  } else if (
    gatewayPolicies.some(
      (producer) =>
        producer.role === "gateway-policy" &&
        producer.span.name === "gateway.backend_selection" &&
        ["unavailable", "proxy_unavailable"].includes(
          String(producer.attributes.get("trapigo.policy.outcome")),
        ),
    )
  ) {
    delivery = { status: "not-attempted" };
  }

  const normalizedPolicies = policyOutcomes(gatewayPolicies);
  const trace = correlateTraceEvidence(result, {
    spans: traceSpans(spans),
    policies: policyEvidence(normalizedPolicies),
    replicaSelection,
    delivery,
  });
  const normalizedIds = new Set(spans.map((span) => span.spanId));
  const parentageComplete = spans.every(
    (span) =>
      span.parentSpanId === null || normalizedIds.has(span.parentSpanId),
  );
  const hasService = producers.some(
    (producer) => producer.role === "orders-service",
  );
  const selectedAndDelivered =
    proxy !== undefined &&
    service !== undefined &&
    service.span.parentSpanId.toLowerCase() ===
      proxy.span.spanId.toLowerCase() &&
    replicaSelection.status === "selected";
  const terminalPolicyOutcome = gatewayPolicies.some(
    (producer) =>
      producer.role === "gateway-policy" &&
      ((producer.attributes.get("trapigo.policy.outcome") === "rejected" &&
        [
          "gateway.rate_limit",
          "gateway.origin_check",
          "gateway.csrf_check",
          "gateway.authentication",
        ].includes(producer.span.name)) ||
        producer.attributes.get("trapigo.policy.outcome") === "not_matched" ||
        producer.attributes.get("trapigo.policy.outcome") === "unavailable" ||
        producer.attributes.get("trapigo.policy.outcome") ===
          "proxy_unavailable"),
  );
  const proxyFailed =
    proxy !== undefined && !hasService && (statusCode(proxy) ?? 0) >= 500;
  const terminalFailure = terminalPolicyOutcome && !proxy && !hasService;
  const successfulPolicies =
    normalizedPolicies.some(
      (item) =>
        item.policy === "rate-limit" &&
        (item.outcome === "passed" || item.outcome === "disabled"),
    ) &&
    normalizedPolicies.some(
      (item) => item.policy === "csrf-origin" && item.outcome === "passed",
    ) &&
    normalizedPolicies.some(
      (item) => item.policy === "authentication" && item.outcome === "passed",
    ) &&
    normalizedPolicies.some(
      (item) => item.policy === "route-matching" && item.outcome === "matched",
    ) &&
    normalizedPolicies.some(
      (item) =>
        item.policy === "replica-selection" && item.outcome === "selected",
    );
  const coreRoles = [
    "gateway-request",
    "gateway-proxy",
    "orders-service",
  ] as const;
  const coreRolesUnique = coreRoles.every(
    (role) =>
      producers.filter((producer) => producer.role === role).length <= 1,
  );
  const policyNamesUnique =
    new Set(gatewayPolicies.map((producer) => producer.span.name)).size ===
    gatewayPolicies.length;
  const responseStatusesMatch =
    result.kind !== "connection-failure" &&
    gateway !== undefined &&
    statusCode(gateway) === result.statusCode;
  const trustedParentChain =
    gateway !== undefined &&
    gatewayPolicies.every(
      (producer) =>
        producer.span.parentSpanId.toLowerCase() ===
        gateway.span.spanId.toLowerCase(),
    ) &&
    (!proxy ||
      proxy.span.parentSpanId.toLowerCase() ===
        gateway.span.spanId.toLowerCase()) &&
    (!service ||
      (proxy !== undefined &&
        service.span.parentSpanId.toLowerCase() ===
          proxy.span.spanId.toLowerCase()));
  const complete =
    parentageComplete &&
    coreRolesUnique &&
    policyNamesUnique &&
    trustedParentChain &&
    responseStatusesMatch &&
    ((selectedAndDelivered && successfulPolicies) ||
      terminalFailure ||
      proxyFailed);
  return { status: complete ? "available" : "partial", trace };
}
