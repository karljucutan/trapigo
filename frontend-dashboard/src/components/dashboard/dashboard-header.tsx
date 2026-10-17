import { Radio } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { RequestFlowState } from "@/lib/request-flow";
import { presentRequestFlow } from "@/lib/request-flow-presentation";

export function DashboardHeader({ state }: { state: RequestFlowState }) {
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
          Inspect the configured request path and service topology.
          Observability will appear here when a request is connected.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <ConnectionBadge state={state} />
        <Button
          type="button"
          size="lg"
          disabled
          aria-describedby="traffic-controls-help"
        >
          <Radio aria-hidden="true" className="size-4" />
          Send request
        </Button>
        <span id="traffic-controls-help" className="sr-only">
          Traffic controls will be available when request integration is
          implemented.
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
