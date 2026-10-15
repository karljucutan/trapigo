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
import { useEffect, useMemo, useRef, useState } from "react";
import "@xyflow/react/dist/style.css";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

type StageData = {
  number: string;
  title: string;
  description: string;
  icon: LucideIcon;
  variant?: "entry" | "policy" | "routing";
};

type ReplicaPoolData = {
  replicas: string[];
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
  },
  {
    number: "03",
    title: "CSRF & origin checks",
    description: "Origin validation and CSRF protection",
    icon: ShieldCheck,
  },
  {
    number: "04",
    title: "Authentication",
    description: "Validate caller credentials",
    icon: LockKeyhole,
  },
  {
    number: "05",
    title: "Route matching",
    description: "Match gateway route configuration",
    icon: RouteIcon,
  },
  {
    number: "06",
    title: "Replica selection",
    description: "Round-robin upstream selection",
    icon: GitBranch,
    variant: "routing",
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

export function RequestPathPanel() {
  return (
    <section aria-labelledby="flow-heading">
      <Card className="border-slate-200/80 bg-white shadow-sm">
        <CardHeader className="flex-row items-center justify-between gap-3 border-b border-slate-100 px-5 py-4">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600">
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
          <span className="hidden items-center gap-1.5 rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-medium text-slate-600 sm:inline-flex">
            <Layers3 aria-hidden="true" className="size-3.5" />3 configured
            replicas
          </span>
        </CardHeader>
        <CardContent className="p-0">
          <div
            aria-label="Gateway request flow diagram"
            className="h-[1010px] w-full overflow-hidden rounded-b-xl bg-[radial-gradient(#dce2ec_0.8px,transparent_0.8px)] [background-size:18px_18px]"
          >
            <ReactFlowProvider>
              <GatewayFlow />
            </ReactFlowProvider>
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-slate-100 px-5 py-3 text-xs text-slate-500">
            <span className="inline-flex items-center gap-2">
              <span className="size-2 rounded-full bg-indigo-500" />
              Gateway
            </span>
            <span className="inline-flex items-center gap-2">
              <span className="size-2 rounded-sm border border-slate-300 bg-white" />
              Configured service slot
            </span>
            <span className="ml-auto">Select and drag to pan</span>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

function GatewayFlow() {
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

  const nodes = useMemo<DashboardFlowNode[]>(() => {
    const isNarrow = canvasWidth < 480;
    const cardWidth = isNarrow ? Math.max(280, canvasWidth - 36) : 360;
    const x = Math.max(12, (canvasWidth - cardWidth) / 2);

    const stageNodes: StageFlowNode[] = stages.map((stage, index) => ({
      id: `stage-${stage.number}`,
      type: "stage",
      position: { x, y: index === 0 ? 18 : 112 + (index - 1) * 96 },
      data: stage,
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
      },
      style: { width: cardWidth },
      draggable: false,
      selectable: false,
    };

    return [...stageNodes, poolNode];
  }, [canvasWidth]);

  const edges = useMemo<Edge[]>(
    () =>
      nodes.slice(0, -1).map((node, index) => ({
        id: `edge-${node.id}-${nodes[index + 1]?.id}`,
        source: node.id,
        target: nodes[index + 1]?.id ?? "",
        type: "smoothstep",
        markerEnd: { type: MarkerType.ArrowClosed, color: "#cbd5e1" },
        style: { stroke: "#cbd5e1", strokeWidth: 1.5 },
      })),
    [nodes],
  );

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
        <Background color="#cbd5e1" gap={18} size={1} />
        <Controls
          position="bottom-right"
          showInteractive={false}
          className="!overflow-hidden !rounded-lg !border !border-slate-200 !bg-white !shadow-sm [&>button]:!border-slate-100 [&>button]:!bg-white [&>button]:!text-slate-600 [&>button:hover]:!bg-slate-50"
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
    <div
      role="group"
      aria-label={`${data.number === "IN" ? "Gateway entry" : `Step ${data.number}`}: ${data.title}`}
      className={`relative flex min-h-[76px] items-center gap-3 rounded-xl border px-3.5 py-3 shadow-sm ${
        isEntry
          ? "border-indigo-200 bg-indigo-50/95"
          : "border-slate-200 bg-white/95"
      }`}
    >
      {!isEntry && (
        <Handle
          type="target"
          position={Position.Top}
          className="!size-2 !border-0 !bg-slate-300"
        />
      )}
      <div
        className={`flex size-9 shrink-0 items-center justify-center rounded-lg ${
          isEntry || isRouting
            ? "bg-indigo-100 text-indigo-700"
            : "bg-slate-100 text-slate-600"
        }`}
      >
        <Icon aria-hidden="true" className="size-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span
            className={`text-[10px] font-semibold tracking-[0.12em] ${
              isEntry ? "text-indigo-700" : "text-slate-400"
            }`}
          >
            {data.number}
          </span>
          <h3 className="truncate text-[13px] font-semibold text-slate-900">
            {data.title}
          </h3>
        </div>
        <p className="mt-1 truncate text-xs text-slate-500">
          {data.description}
        </p>
      </div>
      {isRouting && (
        <span className="shrink-0 rounded-md bg-indigo-50 px-2 py-1 text-[10px] font-medium text-indigo-700">
          configured
        </span>
      )}
      <Handle
        type="source"
        position={Position.Bottom}
        className={`!size-2 !border-0 ${isEntry ? "!bg-indigo-400" : "!bg-slate-300"}`}
      />
    </div>
  );
}

function ReplicaPoolNodeCard({ data }: NodeProps<ReplicaPoolFlowNode>) {
  return (
    <div className="relative rounded-xl border border-slate-200 bg-white/95 p-3.5 shadow-sm">
      <Handle
        type="target"
        position={Position.Top}
        className="!size-2 !border-0 !bg-slate-300"
      />
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
            <Server aria-hidden="true" className="size-4" />
          </div>
          <div>
            <h3 className="text-xs font-semibold text-slate-900">
              Order service pool
            </h3>
            <p className="mt-0.5 text-[11px] text-slate-500">
              Configured topology · health not observed
            </p>
          </div>
        </div>
        <span className="rounded-full bg-slate-100 px-2 py-1 text-[10px] font-medium text-slate-600">
          3 slots
        </span>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {data.replicas.map((replica) => (
          <div
            key={replica}
            className="rounded-lg border border-slate-200 bg-slate-50 px-2 py-2.5 text-center"
          >
            <span className="block text-[11px] font-medium text-slate-700">
              {replica}
            </span>
            <span className="mt-1 block text-[10px] text-slate-400">
              configured
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
