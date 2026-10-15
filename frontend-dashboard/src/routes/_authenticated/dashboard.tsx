import { createFileRoute } from "@tanstack/react-router";
import { DashboardHeader } from "@/components/dashboard/dashboard-header";
import { RequestPathPanel } from "@/components/dashboard/request-path-panel";
import {
  RequestTimeline,
  TelemetryPanels,
} from "@/components/dashboard/telemetry-panels";

export const Route = createFileRoute("/_authenticated/dashboard")({
  component: DashboardPage,
});

function DashboardPage() {
  return (
    <main className="min-h-[calc(100svh-64px)] bg-slate-50/80 px-4 py-6 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-[1480px]">
        <DashboardHeader />
        <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(330px,0.85fr)]">
          <RequestPathPanel />
          <TelemetryPanels />
        </div>
        <RequestTimeline />
      </div>
    </main>
  );
}
