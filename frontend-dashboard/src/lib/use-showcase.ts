import { useEffect, useEffectEvent, useRef, useState } from "react";
import {
  MAX_REQUEST_HISTORY,
  NOT_CONNECTED,
  type RequestFlowState,
  type RequestResult,
} from "./request-flow";
import { runShowcaseRequest } from "./showcase-request";

const TRAFFIC_INTERVAL_MS = 5_000;
const TRACE_POLL_INTERVAL_MS = 1_500;
const MAX_TRACE_LOOKUPS = 10;

export type ShowcaseHistoryEntry = {
  result: RequestResult;
  selectedReplica: string | null;
};

function wait(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const abort = () => {
      clearTimeout(timer);
      reject(signal.reason);
    };
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", abort);
      resolve();
    }, milliseconds);
    if (signal.aborted) abort();
    else signal.addEventListener("abort", abort, { once: true });
  });
}

export function useShowcase() {
  const [state, setState] = useState<RequestFlowState>(NOT_CONNECTED);
  const [busy, setBusy] = useState(false);
  const [running, setRunning] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [cooldownSeconds, setCooldownSeconds] = useState(0);
  const [message, setMessage] = useState<string | null>(null);
  const [history, setHistory] = useState<ShowcaseHistoryEntry[]>([]);
  const [counts, setCounts] = useState<Record<string, number>>({
    "replica-1": 0,
    "replica-2": 0,
    "replica-3": 0,
  });
  const active = useRef(false);
  const traffic = useRef(false);
  const nextSendAt = useRef(0);
  const lifecycle = useRef<AbortController | null>(null);

  function stopTraffic() {
    traffic.current = false;
    setRunning(false);
  }

  async function sendRequest() {
    const signal = lifecycle.current?.signal;
    if (
      !signal ||
      signal.aborted ||
      active.current ||
      document.hidden ||
      Date.now() < nextSendAt.current
    ) {
      return;
    }
    active.current = true;
    setBusy(true);
    setMessage(null);
    setState({ status: "in-flight" });
    let result: RequestResult | null = null;
    let latest: RequestFlowState | null = null;
    let authenticationRequired = false;
    try {
      const request = await runShowcaseRequest();
      if (signal.aborted) return;
      result = request.result;
      latest = { status: "waiting-for-trace", result };
      setState(latest);
      if (result.kind !== "http-response") {
        stopTraffic();
        setMessage(
          "Traffic stopped: the gateway connection or response body failed.",
        );
      }
      if (result.kind !== "connection-failure" && result.statusCode === 429) {
        nextSendAt.current =
          Date.now() + (result.retry.retryAfterMs ?? TRAFFIC_INTERVAL_MS);
      }

      for (let attempt = 0; attempt < MAX_TRACE_LOOKUPS; attempt++) {
        await wait(TRACE_POLL_INTERVAL_MS, signal);
        while (document.hidden) await wait(500, signal);
        const evidence = await request.readTrace();
        if (signal.aborted) return;
        if (evidence.status === "authentication-required") {
          authenticationRequired = true;
          stopTraffic();
          setMessage(
            "Your login is no longer valid. Sign in again before sending another request.",
          );
          latest = { status: "unavailable", result, reason: "query-failed" };
          setState(latest);
          break;
        }
        if (evidence.status === "available" || evidence.status === "partial") {
          latest = { ...evidence, result };
          setState(latest);
          if (
            evidence.trace.policies.some(
              (policy) =>
                policy.policy === "authentication" &&
                policy.outcome === "rejected",
            )
          ) {
            stopTraffic();
            setMessage(
              "Traffic stopped: Trapigo rejected authentication. Check your login before continuing.",
            );
          }
          if (evidence.status === "available") break;
        } else if (evidence.status === "unavailable") {
          if (latest.status !== "partial") {
            latest = { ...evidence, result };
            setState(latest);
          }
          setMessage(
            "Trace lookup did not complete. The orders request will not be replayed.",
          );
          stopTraffic();
          break;
        }
      }
      if (latest.status === "waiting-for-trace") {
        latest = { status: "unavailable", result, reason: "not-found" };
        setState(latest);
      }
      const authenticationOutcome =
        "trace" in latest
          ? latest.trace.policies.find(
              (policy) => policy.policy === "authentication",
            )?.outcome
          : undefined;
      if (
        !authenticationRequired &&
        result.kind !== "connection-failure" &&
        result.statusCode === 401 &&
        authenticationOutcome !== "passed" &&
        authenticationOutcome !== "rejected"
      ) {
        stopTraffic();
        setMessage(
          "Traffic paused: HTTP 401 could not be attributed to the orders service. Check your login.",
        );
      }
    } catch (error) {
      if (signal.aborted) return;
      console.error("Dashboard request or trace lookup failed", error);
      stopTraffic();
      setMessage(
        result
          ? "Trace lookup failed. The real request result is preserved; no orders request was retried."
          : "The BFF request failed. No gateway HTTP result is available.",
      );
      if (result && latest?.status !== "partial") {
        setState({ status: "unavailable", result, reason: "query-failed" });
      } else if (!result) {
        setState(NOT_CONNECTED);
      }
    } finally {
      if (!signal.aborted) {
        if (result) {
          const selection =
            latest && "trace" in latest ? latest.trace.replicaSelection : null;
          const selectedReplica =
            selection?.status === "selected" ? selection.replicaId : null;
          const entry = { result, selectedReplica };
          setHistory((previous) =>
            [entry, ...previous].slice(0, MAX_REQUEST_HISTORY),
          );
          if (selectedReplica) {
            setCounts((previous) =>
              Object.hasOwn(previous, selectedReplica)
                ? {
                    ...previous,
                    [selectedReplica]: previous[selectedReplica] + 1,
                  }
                : previous,
            );
          }
        }
        nextSendAt.current = Math.max(
          nextSendAt.current,
          Date.now() + TRAFFIC_INTERVAL_MS,
        );
        setCooldownSeconds(
          Math.ceil((nextSendAt.current - Date.now()) / 1_000),
        );
        active.current = false;
        setBusy(false);
      }
    }
  }

  function startTraffic() {
    traffic.current = true;
    setRunning(true);
    void sendRequest();
  }

  const tick = useEffectEvent(() => {
    setCooldownSeconds(
      Math.max(0, Math.ceil((nextSendAt.current - Date.now()) / 1_000)),
    );
    if (traffic.current) void sendRequest();
  });

  useEffect(() => {
    const controller = new AbortController();
    lifecycle.current = controller;
    const visibility = () => setHidden(document.hidden);
    visibility();
    document.addEventListener("visibilitychange", visibility);
    const timer = setInterval(tick, 500);
    return () => {
      controller.abort();
      traffic.current = false;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, []);

  return {
    state,
    busy,
    running,
    hidden,
    cooldownSeconds,
    message,
    history,
    counts,
    sendRequest,
    startTraffic,
    stopTraffic,
  };
}
