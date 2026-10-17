import type { LucideIcon } from "lucide-react";
import {
  ArrowDown,
  ArrowRight,
  Clock3,
  GitBranch,
  History,
  Radio,
  Server,
  Workflow,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { RequestFlowState } from "@/lib/request-flow";
import { presentRequestFlow } from "@/lib/request-flow-presentation";

export function TelemetryPanels({ state }: { state: RequestFlowState }) {
  return (
    <aside className="flex flex-col gap-5">
      <ConnectionCard state={state} />
      <TimingCard state={state} />
      <ResultCard state={state} />
      <HistoryCard />
    </aside>
  );
}

export function RequestTimeline({ state }: { state: RequestFlowState }) {
  const flow = presentRequestFlow(state);
  const parents = flow.spans.filter((span) => span.parentSpanId === null);
  const children = flow.spans.filter((span) => span.parentSpanId !== null);
  return (
    <Card className="mt-5 border-border/80 shadow-sm">
      <details>
        <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4 focus-visible:outline-2 focus-visible:outline-offset-[-3px] focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <GitBranch aria-hidden="true" className="size-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">
                Parent / child timeline
              </h2>
              <p className="mt-0.5 text-xs text-muted-foreground">
                Expand to inspect request and service spans
              </p>
            </div>
          </div>
          <ArrowDown
            aria-hidden="true"
            className="size-4 shrink-0 text-muted-foreground"
          />
        </summary>
        <div className="border-t border-border px-5 py-4">
          <div className="grid gap-3 md:grid-cols-2">
            <TimelineLane
              title="Parent request"
              icon={Workflow}
              spans={parents}
              emptyDescription={flow.traceMessage}
            />
            <TimelineLane
              title="Child service spans"
              icon={Server}
              spans={children}
              emptyDescription={flow.traceMessage}
            />
          </div>
        </div>
      </details>
    </Card>
  );
}

function ConnectionCard({ state }: { state: RequestFlowState }) {
  const flow = presentRequestFlow(state);
  return (
    <Card className="border-border/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Radio aria-hidden="true" className="size-4 text-warning" />
          Connection
        </CardTitle>
        <CardDescription className="text-xs">
          Live request and telemetry status
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between rounded-lg border border-warning/20 bg-warning/10 px-3 py-2.5">
          <span className="text-xs font-medium text-foreground">
            Request connection
          </span>
          <span className="text-xs font-semibold text-warning-foreground">
            {flow.connection}
          </span>
        </div>
        <p className="mt-2.5 text-xs leading-5 text-muted-foreground">
          This dashboard is showing configured topology only. No application
          traffic or telemetry is being simulated.
        </p>
      </CardContent>
    </Card>
  );
}

function TimingCard({ state }: { state: RequestFlowState }) {
  const flow = presentRequestFlow(state);
  const total = flow.spans.find((span) => span.label === "Browser request");
  const service = flow.spans.find((span) => span.label === "Orders service");
  return (
    <Card className="border-border/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Clock3 aria-hidden="true" className="size-4 text-muted-foreground" />
          Request timing
        </CardTitle>
        <CardDescription className="text-xs">
          Measurements require an observed request
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-2">
          <MetricPlaceholder
            label="Total request-response"
            value={total?.duration ?? "Unavailable"}
          />
          <MetricPlaceholder
            label="Service processing"
            value={service?.duration ?? "Unavailable"}
          />
        </div>
        <p className="mt-2.5 text-xs text-muted-foreground">
          {flow.traceMessage}
        </p>
      </CardContent>
    </Card>
  );
}

function MetricPlaceholder({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/50 px-3 py-2.5">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p className="mt-1 text-base font-semibold tracking-tight text-muted-foreground">
        {value}
      </p>
    </div>
  );
}

function ResultCard({ state }: { state: RequestFlowState }) {
  const flow = presentRequestFlow(state);
  return (
    <Card className="border-border/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <ArrowRight
            aria-hidden="true"
            className="size-4 text-muted-foreground"
          />
          Latest result
        </CardTitle>
        <CardDescription className="text-xs">{flow.result}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="divide-y divide-border rounded-lg border border-border px-3">
          <ResultRow label="HTTP status" value={flow.httpStatus} />
          <ResultRow label="Retry information" value={flow.retry} />
          <ResultRow label="Selected replica" value={flow.selectedReplica} />
          <ResultRow label="Delivery" value={flow.delivery} />
          {flow.policies.map((policy) => (
            <ResultRow
              key={policy.policy}
              label={policy.label}
              value={policy.outcome}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function ResultRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 py-2.5 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground/80">{value}</span>
    </div>
  );
}

function HistoryCard() {
  return (
    <Card className="border-border/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <History
            aria-hidden="true"
            className="size-4 text-muted-foreground"
          />
          Recent requests
        </CardTitle>
        <CardDescription className="text-xs">
          Bounded history · latest 5
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex min-h-20 flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/50 px-4 py-4 text-center">
          <span className="mb-2 flex size-8 items-center justify-center rounded-full bg-background text-muted-foreground ring-1 ring-border">
            <History aria-hidden="true" className="size-4" />
          </span>
          <p className="text-xs font-medium text-foreground/80">
            No requests observed yet
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">
            History will appear after request integration.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function TimelineLane({
  title,
  spans,
  emptyDescription,
  icon: Icon,
}: {
  title: string;
  spans: readonly {
    spanId: string;
    label: string;
    duration: string;
    boundary: string;
  }[];
  emptyDescription: string;
  icon: LucideIcon;
}) {
  return (
    <div className="rounded-lg border border-border bg-muted/50 p-3.5">
      <div className="flex items-center gap-2 text-xs font-medium text-foreground">
        <Icon aria-hidden="true" className="size-3.5 text-muted-foreground" />
        {title}
      </div>
      {spans.length === 0 ? (
        <p className="mt-2 text-xs text-muted-foreground">{emptyDescription}</p>
      ) : (
        <ul className="mt-2 space-y-2">
          {spans.map((span) => (
            <li
              key={span.spanId}
              className="flex flex-wrap justify-between gap-2 text-xs"
            >
              <span className="font-medium text-foreground">{span.label}</span>
              <span className="text-foreground/80">{span.duration}</span>
              <span className="text-[10px] text-muted-foreground">
                {span.boundary}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
