import { ArrowUpRight, Play, Radio, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { RequestFlowState } from "@/lib/request-flow";
import { presentRequestFlow } from "@/lib/request-flow-presentation";

export function DashboardHeader({
  state,
  onSendRequest,
  onSendDirectRequest,
  onStartTraffic,
  onStopTraffic,
  busy,
  running,
  hidden,
  cooldownSeconds,
}: {
  state: RequestFlowState;
  onSendRequest: () => Promise<unknown>;
  onSendDirectRequest: () => Promise<unknown>;
  onStartTraffic: () => void;
  onStopTraffic: () => void;
  busy: boolean;
  running: boolean;
  hidden: boolean;
  cooldownSeconds: number;
}) {
  return (
    <header className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="mb-2 flex items-center gap-2 text-xs font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          <span className="size-1.5 rounded-full bg-primary" />
          Trapigo / Overview
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
          Gateway dashboard
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm leading-6 text-muted-foreground">
          Send GET /api/v1/go-orders through Trapigo and inspect its actual
          gateway policies, selected replica, and stored trace evidence.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <ConnectionBadge state={state} />
        <Button
          type="button"
          size="lg"
          variant="outline"
          disabled={busy || running || hidden || cooldownSeconds > 0}
          onClick={onSendDirectRequest}
          aria-describedby="traffic-controls-help"
        >
          <ArrowUpRight aria-hidden="true" className="size-4" />
          Send request - direct to Trapigo
        </Button>
        <Button
          type="button"
          size="lg"
          disabled={busy || running || hidden || cooldownSeconds > 0}
          onClick={onSendRequest}
          aria-describedby="traffic-controls-help"
        >
          <Radio aria-hidden="true" className="size-4" />
          Send request
        </Button>
        <Button
          type="button"
          size="lg"
          variant="outline"
          disabled={!running && (busy || hidden || cooldownSeconds > 0)}
          onClick={running ? onStopTraffic : onStartTraffic}
          aria-describedby="traffic-controls-help"
        >
          {running ? (
            <Square aria-hidden="true" />
          ) : (
            <Play aria-hidden="true" />
          )}
          {running ? "Stop traffic" : "Start traffic"}
        </Button>
        {cooldownSeconds > 0 && !busy && (
          <span className="text-xs text-muted-foreground">
            Next send in {cooldownSeconds}s
          </span>
        )}
        <span id="traffic-controls-help" className="sr-only">
          Requests run one at a time, including trace lookup. Direct requests
          send the browser's auth cookies and require credentialed CORS when the
          gateway is on a different origin. Traffic waits at least five seconds
          between cycles and pauses in hidden tabs. Stop prevents the next
          request; the current request can finish.
        </span>
      </div>
    </header>
  );
}

function ConnectionBadge({ state }: { state: RequestFlowState }) {
  const connection = presentRequestFlow(state).connection;
  return (
    <span className="inline-flex h-9 items-center gap-2 rounded-lg border border-warning/25 bg-warning/10 px-3 text-xs font-medium text-warning-foreground">
      <span className="size-1.5 rounded-full bg-warning" />
      {connection}
    </span>
  );
}
