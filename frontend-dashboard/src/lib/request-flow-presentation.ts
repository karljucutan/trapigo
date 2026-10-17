import type {
  PolicyName,
  PolicyOutcome,
  RequestFlowState,
  SpanRole,
} from "./request-flow";

const policyLabels: Record<PolicyName, string> = {
  "rate-limit": "Rate limiting",
  "csrf-origin": "CSRF & origin checks",
  authentication: "Authentication",
  "route-matching": "Route matching",
  "replica-selection": "Replica selection",
};

const outcomeLabels: Record<PolicyOutcome, string> = {
  passed: "Passed",
  rejected: "Rejected",
  disabled: "Disabled",
  matched: "Matched",
  "not-matched": "Not matched",
  selected: "Selected",
  "not-selected": "Not selected",
};

const spanLabels: Record<SpanRole, string> = {
  "browser-request": "Browser request",
  "bff-outbound": "BFF to gateway",
  "gateway-request": "Gateway request",
  "gateway-policy": "Gateway policy",
  "gateway-proxy": "Gateway to replica",
  "orders-service": "Orders service",
};

const policyOrder: readonly PolicyName[] = [
  "rate-limit",
  "csrf-origin",
  "authentication",
  "route-matching",
  "replica-selection",
];

export function presentRequestFlow(state: RequestFlowState) {
  const trace =
    state.status === "partial" || state.status === "available"
      ? state.trace
      : null;
  const policies = new Map(
    trace?.policies.map((item) => [item.policy, item.outcome]),
  );
  const result = "result" in state ? state.result : null;
  const unavailable = state.status === "unavailable" ? state.reason : null;

  return {
    connection:
      state.status === "not-connected"
        ? "Not connected"
        : state.status === "in-flight"
          ? "Request in flight"
          : "Connected",
    result:
      state.status === "not-connected"
        ? "Not observed"
        : state.status === "in-flight"
          ? "In progress"
          : result?.kind === "connection-failure"
            ? "Connection failed"
            : "Response received",
    httpStatus:
      result?.kind === "http-response"
        ? String(result.statusCode)
        : result
          ? "No HTTP response"
          : "Unavailable",
    retry:
      result === null
        ? "Unavailable"
        : `${result.retry.attempts} retries${
            result.retry.retryAfterMs === null
              ? ""
              : ` · retry after ${result.retry.retryAfterMs} ms`
          }`,
    selectedReplica:
      trace?.replicaSelection.status === "selected"
        ? trace.replicaSelection.replicaId
        : trace?.replicaSelection.status === "not-selected"
          ? "Not selected"
          : "Not observed",
    delivery:
      trace?.delivery.status === "delivered"
        ? `Delivered to ${trace.delivery.replicaId}`
        : trace?.delivery.status === "failed"
          ? "Delivery failed"
          : trace?.delivery.status === "not-attempted"
            ? "Not attempted"
            : "Not observed",
    traceMessage:
      state.status === "not-connected"
        ? "Unavailable · no trace observed"
        : state.status === "in-flight"
          ? "Waiting · request is in flight"
          : state.status === "waiting-for-trace"
            ? "Waiting · trace export and lookup are pending"
            : state.status === "partial"
              ? "Partial · some trace evidence is missing"
              : state.status === "available"
                ? "Available · observed trace evidence"
                : `Unavailable · ${unavailable?.replaceAll("-", " ")}`,
    policies: policyOrder.map((policy) => {
      const outcome = policies.get(policy);
      return {
        policy,
        label: policyLabels[policy],
        outcome: outcome ? outcomeLabels[outcome] : "Not observed",
      };
    }),
    spans:
      trace?.spans.map((span) => ({
        spanId: span.spanId,
        parentSpanId: span.parentSpanId,
        label: spanLabels[span.role],
        duration: span.duration ? `${span.duration.valueMs} ms` : "Unavailable",
        boundary: span.duration?.boundary ?? "Duration unavailable",
      })) ?? [],
  };
}
