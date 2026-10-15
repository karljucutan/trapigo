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

export function TelemetryPanels() {
  return (
    <aside className="flex flex-col gap-5">
      <ConnectionCard />
      <TimingCard />
      <ResultCard />
      <HistoryCard />
    </aside>
  );
}

export function RequestTimeline() {
  return (
    <Card className="mt-5 border-slate-200/80 shadow-sm">
      <details>
        <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-4 focus-visible:outline-2 focus-visible:outline-offset-[-3px] focus-visible:outline-indigo-500 [&::-webkit-details-marker]:hidden">
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-xl bg-slate-100 text-slate-600">
              <GitBranch aria-hidden="true" className="size-4" />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-slate-900">
                Parent / child timeline
              </h2>
              <p className="mt-0.5 text-xs text-slate-500">
                Expand to inspect request and service spans
              </p>
            </div>
          </div>
          <ArrowDown
            aria-hidden="true"
            className="size-4 shrink-0 text-slate-400"
          />
        </summary>
        <div className="border-t border-slate-100 px-5 py-4">
          <div className="grid gap-3 md:grid-cols-2">
            <TimelineLane
              title="Parent request"
              icon={Workflow}
              description="Unavailable · no request observed"
            />
            <TimelineLane
              title="Child service spans"
              icon={Server}
              description="Unavailable · no trace observed"
            />
          </div>
        </div>
      </details>
    </Card>
  );
}

function ConnectionCard() {
  return (
    <Card className="border-slate-200/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Radio aria-hidden="true" className="size-4 text-amber-600" />
          Connection
        </CardTitle>
        <CardDescription className="text-xs">
          Live request and telemetry status
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between rounded-lg border border-amber-100 bg-amber-50/70 px-3 py-2.5">
          <span className="text-xs font-medium text-slate-700">
            Gateway connection
          </span>
          <span className="text-xs font-semibold text-amber-800">
            Not connected
          </span>
        </div>
        <p className="mt-2.5 text-xs leading-5 text-slate-500">
          This dashboard is showing configured topology only. No application
          traffic or telemetry is being simulated.
        </p>
      </CardContent>
    </Card>
  );
}

function TimingCard() {
  return (
    <Card className="border-slate-200/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Clock3 aria-hidden="true" className="size-4 text-slate-500" />
          Request timing
        </CardTitle>
        <CardDescription className="text-xs">
          Measurements require an observed request
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-2">
          <MetricPlaceholder label="Gateway total" />
          <MetricPlaceholder label="Service time" />
        </div>
        <p className="mt-2.5 text-xs text-slate-500">
          Timings unavailable · no request observed
        </p>
      </CardContent>
    </Card>
  );
}

function MetricPlaceholder({ label }: { label: string }) {
  return (
    <div className="rounded-lg bg-slate-50 px-3 py-2.5">
      <p className="text-[11px] text-slate-500">{label}</p>
      <p className="mt-1 text-base font-semibold tracking-tight text-slate-400">
        —
        <span className="ml-1 text-[10px] font-normal tracking-normal">
          unavailable
        </span>
      </p>
    </div>
  );
}

function ResultCard() {
  return (
    <Card className="border-slate-200/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <ArrowRight aria-hidden="true" className="size-4 text-slate-500" />
          Latest result
        </CardTitle>
        <CardDescription className="text-xs">
          No observed request
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="divide-y divide-slate-100 rounded-lg border border-slate-100 px-3">
          <ResultRow label="HTTP status" value="Unavailable" />
          <ResultRow label="Outcome" value="Not observed" />
          <ResultRow label="Selected replica" value="None observed" />
        </div>
      </CardContent>
    </Card>
  );
}

function ResultRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 py-2.5 text-xs">
      <span className="text-slate-500">{label}</span>
      <span className="text-right font-medium text-slate-600">{value}</span>
    </div>
  );
}

function HistoryCard() {
  return (
    <Card className="border-slate-200/80 shadow-sm">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <History aria-hidden="true" className="size-4 text-slate-500" />
          Recent requests
        </CardTitle>
        <CardDescription className="text-xs">
          Bounded history · latest 5
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex min-h-20 flex-col items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50/60 px-4 py-4 text-center">
          <span className="mb-2 flex size-8 items-center justify-center rounded-full bg-white text-slate-400 ring-1 ring-slate-200">
            <History aria-hidden="true" className="size-4" />
          </span>
          <p className="text-xs font-medium text-slate-600">
            No requests observed yet
          </p>
          <p className="mt-1 text-[11px] text-slate-500">
            History will appear after request integration.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function TimelineLane({
  title,
  description,
  icon: Icon,
}: {
  title: string;
  description: string;
  icon: LucideIcon;
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50/70 p-3.5">
      <div className="flex items-center gap-2 text-xs font-medium text-slate-700">
        <Icon aria-hidden="true" className="size-3.5 text-slate-500" />
        {title}
      </div>
      <p className="mt-2 text-xs text-slate-500">{description}</p>
    </div>
  );
}
