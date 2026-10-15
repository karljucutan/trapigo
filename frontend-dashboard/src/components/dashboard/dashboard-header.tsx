import { Radio } from "lucide-react";
import { Button } from "@/components/ui/button";

export function DashboardHeader() {
  return (
    <header className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="mb-2 flex items-center gap-2 text-xs font-semibold tracking-[0.16em] text-slate-500 uppercase">
          <span className="size-1.5 rounded-full bg-indigo-500" />
          Trapigo / Overview
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-950 sm:text-3xl">
          Gateway dashboard
        </h1>
        <p className="mt-1.5 max-w-2xl text-sm leading-6 text-slate-600">
          Inspect the configured request path and service topology.
          Observability will appear here when a request is connected.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <ConnectionBadge />
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

function ConnectionBadge() {
  return (
    <span className="inline-flex h-9 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 text-xs font-medium text-amber-800">
      <span className="size-1.5 rounded-full bg-amber-500" />
      Not connected
    </span>
  );
}
