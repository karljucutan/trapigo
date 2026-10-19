import { createFileRoute } from "@tanstack/react-router";
import { DashboardHeader } from "@/components/dashboard/dashboard-header";
import { RequestPathPanel } from "@/components/dashboard/request-path-panel";
import {
  RequestTimeline,
  TelemetryPanels,
} from "@/components/dashboard/telemetry-panels";
import { useShowcase } from "@/lib/use-showcase";

export const Route = createFileRoute("/_authenticated/dashboard")({
  component: DashboardPage,
});

function DashboardPage() {
  const showcase = useShowcase();
  return (
    <main className="min-h-[calc(100svh-64px)] bg-muted/50 px-4 py-6 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-[1480px]">
        <DashboardHeader
          state={showcase.state}
          busy={showcase.busy}
          running={showcase.running}
          hidden={showcase.hidden}
          cooldownSeconds={showcase.cooldownSeconds}
          onSendRequest={showcase.sendRequest}
          onSendDirectRequest={showcase.sendDirectRequest}
          onStartTraffic={showcase.startTraffic}
          onStopTraffic={showcase.stopTraffic}
        />
        <div
          aria-live="polite"
          role="status"
          className="mb-4 text-sm text-muted-foreground"
        >
          {showcase.message ??
            (showcase.hidden && showcase.running
              ? "Traffic paused while this tab is hidden."
              : showcase.busy
                ? "Finishing this request and its bounded trace lookup before another send."
                : "Ready. Requests only run when you use the controls.")}
        </div>
        <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(330px,0.85fr)]">
          <RequestPathPanel state={showcase.state} counts={showcase.counts} />
          <TelemetryPanels
            state={showcase.state}
            history={showcase.history}
            totalResponseTimeMs={showcase.totalResponseTimeMs}
          />
        </div>
        <RequestTimeline state={showcase.state} />
      </div>
    </main>
  );
}
