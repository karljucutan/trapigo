import assert from "node:assert/strict";
import { test } from "node:test";
import {
  correlateTraceEvidence,
  httpStatus,
  MAX_POLICY_EVIDENCE,
  MAX_REQUEST_HISTORY,
  MAX_RETRY_ATTEMPTS,
  MAX_TRACE_SPANS,
  policyEvidence,
  publicReplicaId,
  type RequestResult,
  requestHistory,
  retryAfterMs,
  retryAttempts,
  spanDurationMs,
  spanId,
  type TraceEvidence,
  traceId,
  traceSpans,
} from "./request-flow.ts";
import { presentRequestFlow } from "./request-flow-presentation.ts";

const id = traceId("1234567890abcdef1234567890abcdef");
const rootSpan = spanId("1234567890abcdef");

function response(status: number): RequestResult {
  return {
    kind: "http-response",
    traceId: id,
    statusCode: httpStatus(status),
    retry: { attempts: retryAttempts(0), retryAfterMs: null },
  };
}

function evidence(overrides: Partial<TraceEvidence> = {}): TraceEvidence {
  return {
    traceId: id,
    spans: traceSpans([]),
    policies: policyEvidence([]),
    replicaSelection: { status: "unknown" },
    delivery: { status: "unknown" },
    ...overrides,
  };
}

test("observed selection does not claim successful delivery", () => {
  const view = presentRequestFlow({
    status: "available",
    result: response(200),
    trace: evidence({
      spans: traceSpans([
        {
          spanId: rootSpan,
          parentSpanId: null,
          role: "browser-request",
          duration: {
            valueMs: spanDurationMs(24),
            boundary: "browser-request-through-response",
          },
        },
      ]),
      policies: policyEvidence([
        { policy: "authentication", outcome: "passed" },
      ]),
      replicaSelection: {
        status: "selected",
        replicaId: publicReplicaId("orders-2"),
      },
      delivery: { status: "failed" },
    }),
  });
  assert.equal(view.selectedReplica, "orders-2");
  assert.equal(view.delivery, "Delivery failed");
  assert.equal(view.spans[0]?.duration, "24 ms");
  assert.equal(view.policies[2]?.outcome, "Passed");
});

test("rejection, connection failure, and incomplete evidence stay explicit", () => {
  const rejected = presentRequestFlow({
    status: "partial",
    result: response(429),
    trace: evidence({
      policies: policyEvidence([{ policy: "rate-limit", outcome: "rejected" }]),
      delivery: { status: "not-attempted" },
    }),
  });
  assert.equal(rejected.policies[0]?.outcome, "Rejected");
  assert.equal(rejected.selectedReplica, "Not observed");
  assert.equal(rejected.delivery, "Not attempted");

  const failed = presentRequestFlow({
    status: "unavailable",
    result: {
      kind: "connection-failure",
      traceId: id,
      retry: { attempts: retryAttempts(2), retryAfterMs: null },
    },
    reason: "timed-out",
  });
  assert.equal(failed.result, "Connection failed");
  assert.equal(failed.httpStatus, "No HTTP response");
  assert.equal(failed.selectedReplica, "Not observed");
  assert.match(failed.traceMessage, /timed out/);

  const partial = presentRequestFlow({
    status: "partial",
    result: response(502),
    trace: evidence(),
  });
  assert.ok(
    partial.policies.every((policy) => policy.outcome === "Not observed"),
  );
  assert.equal(partial.delivery, "Not observed");
});

test("lifecycle states and bounded contract constructors", () => {
  assert.equal(
    presentRequestFlow({ status: "not-connected" }).connection,
    "Not connected",
  );
  assert.equal(
    presentRequestFlow({ status: "in-flight", traceId: id }).selectedReplica,
    "Not observed",
  );
  assert.match(
    presentRequestFlow({ status: "waiting-for-trace", result: response(200) })
      .traceMessage,
    /^Waiting/,
  );
  assert.throws(() => traceId("invalid"));
  assert.throws(() => spanId("invalid"));
  assert.throws(() => publicReplicaId("https://internal.example"));
  assert.throws(() => httpStatus(99));
  assert.throws(() => retryAttempts(MAX_RETRY_ATTEMPTS + 1));
  assert.throws(() => retryAfterMs(300_001));
  assert.throws(() => spanDurationMs(300_001));
  assert.throws(() =>
    traceSpans(
      Array.from({ length: MAX_TRACE_SPANS + 1 }, () => ({
        spanId: rootSpan,
        parentSpanId: null,
        role: "browser-request" as const,
        duration: null,
      })),
    ),
  );
  assert.throws(() =>
    policyEvidence(
      Array.from({ length: MAX_POLICY_EVIDENCE + 1 }, () => ({
        policy: "authentication" as const,
        outcome: "passed" as const,
      })),
    ),
  );
  assert.throws(() =>
    requestHistory(
      Array.from({ length: MAX_REQUEST_HISTORY + 1 }, () => response(200)),
    ),
  );
});

test("trace evidence is correlated to the request trace ID", () => {
  const result = response(204);
  assert.equal(
    correlateTraceEvidence(result, {
      spans: traceSpans([]),
      policies: policyEvidence([]),
      replicaSelection: { status: "unknown" },
      delivery: { status: "unknown" },
    }).traceId,
    result.traceId,
  );
});
