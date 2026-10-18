import {
  Background,
  Controls,
  type Edge,
  Handle,
  MarkerType,
  type Node,
  type NodeProps,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  Clock3,
  GitBranch,
  Globe,
  Layers3,
  LockKeyhole,
  Route as RouteIcon,
  Server,
  ShieldCheck,
  Waypoints,
  Workflow,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import "@xyflow/react/dist/style.css";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { PolicyName, RequestFlowState } from "@/lib/request-flow";
import { presentRequestFlow } from "@/lib/request-flow-presentation";

type StageData = {
  number: string;
  title: string;
  description: string;
  icon: LucideIcon;
  variant?: "entry" | "policy" | "routing";
  policy?: PolicyName;
  observation?: string;
};

type ReplicaPoolData = {
  replicas: string[];
  observedSelection: string;
};

type StageFlowNode = Node<StageData, "stage">;
type ReplicaPoolFlowNode = Node<ReplicaPoolData, "replicaPool">;
type DashboardFlowNode = StageFlowNode | ReplicaPoolFlowNode;

const stages: StageData[] = [
  {
    number: "IN",
    title: "Incoming request",
    description: "Gateway entry point",
    icon: Globe,
    variant: "entry",
  },
  {
    number: "01",
    title: "Request logging",
    description: "Request ID and duration logging",
    icon: Activity,
  },
  {
    number: "02",
    title: "Rate limiting",
    description: "IP-based token bucket",
    icon: Clock3,
    policy: "rate-limit",
  },
  {
    number: "03",
    title: "CSRF & origin checks",
    description: "Origin validation and CSRF protection",
    icon: ShieldCheck,
    policy: "csrf-origin",
  },
  {
    number: "04",
    title: "Authentication",
    description: "Validate caller credentials",
    icon: LockKeyhole,
    policy: "authentication",
  },
  {
    number: "05",
    title: "Route matching",
    description: "Match gateway route configuration",
    icon: RouteIcon,
    policy: "route-matching",
  },
  {
    number: "06",
    title: "Replica selection",
    description: "Round-robin upstream selection",
    icon: GitBranch,
    variant: "routing",
    policy: "replica-selection",
  },
  {
    number: "07",
    title: "Reverse proxy",
    description: "Forward to the selected upstream",
    icon: Waypoints,
  },
];

const nodeTypes = {
  stage: StageNodeCard,
  replicaPool: ReplicaPoolNodeCard,
};

export function RequestPathPanel({ state }: { state: RequestFlowState }) {
  const flow = presentRequestFlow(state);
  return (
    <section aria-labelledby="flow-heading">
      <Card className="border-border/80 bg-card shadow-sm">
        <CardHeader className="flex-row items-center justify-between gap-3 border-b border-border px-5 py-4">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Workflow aria-hidden="true" className="size-4.5" />
            </div>
            <div>
              <CardTitle id="flow-heading" className="text-sm">
                Request path
              </CardTitle>
              <CardDescription className="mt-0.5 text-xs">
                Gateway processing order · configured topology
              </CardDescription>
            </div>
          </div>
          <span className="hidden items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-[11px] font-medium text-muted-foreground sm:inline-flex">
            <Layers3 aria-hidden="true" className="size-3.5" />3 configured
            replicas
          </span>
        </CardHeader>
        <CardContent className="p-0">
          <fieldset className="m-0 min-w-0 border-0 p-0">
            <legend className="sr-only">Gateway request flow diagram</legend>
            <div className="h-[1010px] w-full overflow-hidden rounded-b-xl bg-[radial-gradient(var(--border)_0.8px,transparent_0.8px)] [background-size:18px_18px]">
              <ReactFlowProvider>
                <GatewayFlow
                  policies={flow.policies}
                  selectedReplica={flow.selectedReplica}
                />
              </ReactFlowProvider>
            </div>
          </fieldset>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-border px-5 py-3 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <span className="size-2 rounded-full bg-primary" />
              Gateway
            </span>
            <span className="inline-flex items-center gap-2">
              <span className="size-2 rounded-sm border border-border bg-background" />
              Configured service slot
            </span>
            <span className="ml-auto">Select and drag to pan</span>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

function GatewayFlow({
  policies,
  selectedReplica,
}: {
  policies: ReturnType<typeof presentRequestFlow>["policies"];
  selectedReplica: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [canvasWidth, setCanvasWidth] = useState(0);
  const { fitView } = useReactFlow<DashboardFlowNode>();

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    const observer = new ResizeObserver((entries) => {
      setCanvasWidth(entries[0].contentRect.width);
    });
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  const isNarrow = canvasWidth < 480;
  const cardWidth = isNarrow ? Math.max(280, canvasWidth - 36) : 460;
  const x = Math.max(12, (canvasWidth - cardWidth) / 2);

  const stageNodes: StageFlowNode[] = stages.map((stage, index) => ({
    id: `stage-${stage.number}`,
    type: "stage",
    position: { x, y: index === 0 ? 18 : 112 + (index - 1) * 108 },
    data: {
      ...stage,
      observation: stage.policy
        ? policies.find((policy) => policy.policy === stage.policy)?.outcome
        : undefined,
    },
    style: { width: cardWidth },
    draggable: false,
    selectable: false,
  }));

  const poolNode: ReplicaPoolFlowNode = {
    id: "replica-pool",
    type: "replicaPool",
    position: { x, y: 900 },
    data: {
      replicas: ["Replica 01", "Replica 02", "Replica 03"],
      observedSelection: selectedReplica,
    },
    style: { width: cardWidth },
    draggable: false,
    selectable: false,
  };

  const nodes: DashboardFlowNode[] = [...stageNodes, poolNode];
  const edges: Edge[] = nodes.slice(0, -1).map((node, index) => ({
    id: `edge-${node.id}-${nodes[index + 1]?.id}`,
    source: node.id,
    target: nodes[index + 1]?.id ?? "",
    type: "smoothstep",
    markerEnd: { type: MarkerType.ArrowClosed, color: "var(--border)" },
    style: { stroke: "var(--border)", strokeWidth: 1.5 },
  }));

  useEffect(() => {
    if (canvasWidth > 0) {
      void fitView({ padding: 0.12, minZoom: 0.68, maxZoom: 1 });
    }
  }, [canvasWidth, fitView]);

  return (
    <div ref={containerRef} className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.12, minZoom: 0.68, maxZoom: 1 }}
        minZoom={0.68}
        maxZoom={1.25}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        deleteKeyCode={null}
        panOnDrag
        zoomOnScroll={false}
        zoomOnPinch
        proOptions={{ hideAttribution: true }}
        className="!bg-transparent"
      >
        <Background color="var(--border)" gap={18} size={1} />
        <Controls
          position="bottom-right"
          showInteractive={false}
          className="!overflow-hidden !rounded-lg !border !border-border !bg-background !shadow-sm [&>button]:!border-border [&>button]:!bg-background [&>button]:!text-muted-foreground [&>button:hover]:!bg-muted"
        />
      </ReactFlow>
    </div>
  );
}

function StageNodeCard({ data }: NodeProps<StageFlowNode>) {
  const Icon = data.icon;
  const isEntry = data.variant === "entry";
  const isRouting = data.variant === "routing";

  return (
    <article
      aria-labelledby={`stage-title-${data.number}`}
      className={`relative flex min-h-[76px] items-center gap-3 rounded-xl border px-3.5 py-3 shadow-sm ${
        isEntry ? "border-primary/30 bg-primary/10" : "border-border bg-card"
      }`}
    >
      {!isEntry && (
        <Handle
          type="target"
          position={Position.Top}
          className="!size-2 !border-0 !bg-muted-foreground"
        />
      )}
      <div
        className={`flex size-9 shrink-0 items-center justify-center rounded-lg ${
          isEntry || isRouting
            ? "bg-primary/15 text-primary"
            : "bg-muted text-muted-foreground"
        }`}
      >
        <Icon aria-hidden="true" className="size-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span
            className={`text-[10px] font-semibold tracking-[0.12em] ${
              isEntry ? "text-primary" : "text-muted-foreground"
            }`}
          >
            {data.number}
          </span>
          <h3
            id={`stage-title-${data.number}`}
            className="break-words text-[13px] font-semibold text-foreground"
          >
            {data.title}
          </h3>
        </div>
        <p className="mt-1 whitespace-normal break-words text-xs text-muted-foreground">
          {data.description}
        </p>
      </div>
      {isRouting && (
        <span className="shrink-0 rounded-md bg-primary/10 px-2 py-1 text-[10px] font-medium text-primary">
          configured
        </span>
      )}
      {data.observation && (
        <span className="ml-auto shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
          {data.observation}
        </span>
      )}
      <Handle
        type="source"
        position={Position.Bottom}
        className={`!size-2 !border-0 ${isEntry ? "!bg-primary" : "!bg-muted-foreground"}`}
      />
    </article>
  );
}

function ReplicaPoolNodeCard({ data }: NodeProps<ReplicaPoolFlowNode>) {
  return (
    <div className="relative rounded-xl border border-border bg-card p-3.5 shadow-sm">
      <Handle
        type="target"
        position={Position.Top}
        className="!size-2 !border-0 !bg-muted-foreground"
      />
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <Server aria-hidden="true" className="size-4" />
          </div>
          <div>
            <h3 className="text-xs font-semibold text-foreground">
              Order service pool
            </h3>
            <p className="mt-0.5 text-[11px] text-muted-foreground">
              Configured topology · health not observed
            </p>
          </div>
        </div>
        <span className="rounded-full bg-muted px-2 py-1 text-[10px] font-medium text-muted-foreground">
          3 slots
        </span>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {data.replicas.map((replica) => (
          <div
            key={replica}
            className="rounded-lg border border-border bg-muted/50 px-2 py-2.5 text-center"
          >
            <span className="block text-[11px] font-medium text-foreground">
              {replica}
            </span>
            <span className="mt-1 block text-[10px] text-muted-foreground">
              configured
            </span>
          </div>
        ))}
      </div>
      <p className="mt-3 border-t border-border pt-2 text-[11px] text-muted-foreground">
        Observed selection: {data.observedSelection}
      </p>
    </div>
  );
}
