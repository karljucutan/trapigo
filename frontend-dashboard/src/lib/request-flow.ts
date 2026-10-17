export const MAX_TRACE_SPANS = 64;
export const MAX_POLICY_EVIDENCE = 8;
export const MAX_REQUEST_HISTORY = 5;
export const MAX_RETRY_ATTEMPTS = 10;
export const MAX_RETRY_AFTER_MS = 300_000;
export const MAX_SPAN_DURATION_MS = 300_000;

declare const traceIdBrand: unique symbol;
declare const spanIdBrand: unique symbol;
declare const replicaIdBrand: unique symbol;
declare const statusBrand: unique symbol;
declare const durationBrand: unique symbol;
declare const attemptsBrand: unique symbol;
declare const retryAfterBrand: unique symbol;
declare const boundedBrand: unique symbol;

export type TraceId = string & { [traceIdBrand]: true };
export type SpanId = string & { [spanIdBrand]: true };
export type PublicReplicaId = string & { [replicaIdBrand]: true };
export type HttpStatus = number & { [statusBrand]: true };
export type SpanDurationMs = number & { [durationBrand]: true };
export type RetryAttempts = number & { [attemptsBrand]: true };
export type RetryAfterMs = number & { [retryAfterBrand]: true };
type Bounded<T, TMax extends number> = readonly T[] & {
  [boundedBrand]: TMax;
};

function bounded<T, TMax extends number>(
  items: readonly T[],
  max: TMax,
): Bounded<T, TMax> {
  if (items.length > max) throw new RangeError(`Evidence exceeds limit ${max}`);
  return Object.freeze([...items]) as Bounded<T, TMax>;
}

export function traceId(value: string): TraceId {
  if (!/^[\da-f]{32}$/i.test(value)) throw new RangeError("Invalid trace ID");
  return value.toLowerCase() as TraceId;
}

export function spanId(value: string): SpanId {
  if (!/^[\da-f]{16}$/i.test(value)) throw new RangeError("Invalid span ID");
  return value.toLowerCase() as SpanId;
}

export function publicReplicaId(value: string): PublicReplicaId {
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(value)) {
    throw new RangeError("Invalid public replica label");
  }
  return value as PublicReplicaId;
}

export function httpStatus(value: number): HttpStatus {
  if (!Number.isInteger(value) || value < 100 || value > 599) {
    throw new RangeError("Invalid HTTP status");
  }
  return value as HttpStatus;
}

export function spanDurationMs(value: number): SpanDurationMs {
  if (!Number.isFinite(value) || value < 0 || value > MAX_SPAN_DURATION_MS) {
    throw new RangeError("Span duration is outside the supported range");
  }
  return value as SpanDurationMs;
}

export function retryAttempts(value: number): RetryAttempts {
  if (!Number.isInteger(value) || value < 0 || value > MAX_RETRY_ATTEMPTS) {
    throw new RangeError("Retry attempts are outside the supported range");
  }
  return value as RetryAttempts;
}

export function retryAfterMs(value: number): RetryAfterMs {
  if (!Number.isFinite(value) || value < 0 || value > MAX_RETRY_AFTER_MS) {
    throw new RangeError("Retry-After is outside the supported range");
  }
  return value as RetryAfterMs;
}

export type RetryInfo = {
  attempts: RetryAttempts;
  retryAfterMs: RetryAfterMs | null;
};

export type RequestResult =
  | {
      kind: "http-response";
      traceId: TraceId;
      statusCode: HttpStatus;
      retry: RetryInfo;
    }
  | { kind: "connection-failure"; traceId: TraceId; retry: RetryInfo };

export type RequestHistory = Bounded<RequestResult, typeof MAX_REQUEST_HISTORY>;

export function requestHistory(
  items: readonly RequestResult[],
): RequestHistory {
  return bounded(items, MAX_REQUEST_HISTORY);
}

export type PolicyName =
  | "rate-limit"
  | "csrf-origin"
  | "authentication"
  | "route-matching"
  | "replica-selection";

export type PolicyOutcome =
  | "passed"
  | "rejected"
  | "disabled"
  | "matched"
  | "not-matched"
  | "selected"
  | "not-selected";

export type PolicyEvidence =
  | {
      policy: "rate-limit" | "csrf-origin" | "authentication";
      outcome: "passed" | "rejected" | "disabled";
    }
  | { policy: "route-matching"; outcome: "matched" | "not-matched" }
  | { policy: "replica-selection"; outcome: "selected" | "not-selected" };

export function policyEvidence(items: readonly PolicyEvidence[]) {
  return bounded(items, MAX_POLICY_EVIDENCE);
}

export type MeasurementBoundary =
  | "browser-request-through-response"
  | "bff-outbound-through-response-body"
  | "gateway-server-request"
  | "policy-execution"
  | "proxy-outbound-through-response"
  | "service-server-request";

export type SpanRole =
  | "browser-request"
  | "bff-outbound"
  | "gateway-request"
  | "gateway-policy"
  | "gateway-proxy"
  | "orders-service";

export type TraceSpan = {
  spanId: SpanId;
  parentSpanId: SpanId | null;
  role: SpanRole;
  duration: {
    valueMs: SpanDurationMs;
    boundary: MeasurementBoundary;
  } | null;
};

export function traceSpans(items: readonly TraceSpan[]) {
  return bounded(items, MAX_TRACE_SPANS);
}

export type ReplicaSelection =
  | { status: "unknown" }
  | { status: "not-selected" }
  | { status: "selected"; replicaId: PublicReplicaId };

export type DeliveryEvidence =
  | { status: "unknown" }
  | { status: "not-attempted" }
  | { status: "failed" }
  | { status: "delivered"; replicaId: PublicReplicaId };

export type TraceEvidence = {
  traceId: TraceId;
  spans: ReturnType<typeof traceSpans>;
  policies: ReturnType<typeof policyEvidence>;
  replicaSelection: ReplicaSelection;
  delivery: DeliveryEvidence;
};

export function correlateTraceEvidence(
  result: RequestResult,
  evidence: Omit<TraceEvidence, "traceId">,
): TraceEvidence {
  return { traceId: result.traceId, ...evidence };
}

export type TraceUnavailableReason =
  | "not-found"
  | "sampled-out"
  | "timed-out"
  | "query-failed";

export type RequestFlowState =
  | { status: "not-connected" }
  | { status: "in-flight"; traceId: TraceId }
  | { status: "waiting-for-trace"; result: RequestResult }
  | { status: "partial"; result: RequestResult; trace: TraceEvidence }
  | { status: "available"; result: RequestResult; trace: TraceEvidence }
  | {
      status: "unavailable";
      result: RequestResult;
      reason: TraceUnavailableReason;
    };

export const NOT_CONNECTED: RequestFlowState = Object.freeze({
  status: "not-connected",
});
