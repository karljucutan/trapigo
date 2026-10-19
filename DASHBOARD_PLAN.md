# Dashboard Request Flow Showcase Plan

Status: Phases 2 and 3 infrastructure configured; gateway and orders-service
instrumentation implemented; full-chain validation pending.

For a Phase 3 architecture and YAML walkthrough, see
[phase3_explanation.md](./phase3_explanation.md).

## Goal

Visualize real requests through this topology:

```text
Client -> TanStack Start BFF -> Trapigo API gateway -> Go orders replica
                                                    |-- Replica 1
                                                    |-- Replica 2
                                                    `-- Replica 3
```

Each request must highlight the replica actually selected by Trapigo, not a
destination predicted by frontend round-robin logic.

## Confirmed Requirements

- Use real `GET /api/v1/go-orders` requests, not simulated traffic.
- Offer Send Request and Start/Stop Traffic controls. No burst generator.
- Show rate limiting, authentication, route matching, and load balancing.
- Show horizontally scaled Go orders replicas and request distribution.
- Show gateway request time, policy timings, proxy round trip, and service
  processing time. Browser and BFF tracing are out of scope.
- Preserve the existing dashboard's authentication and design conventions.
- Save this plan before making application changes.
- Use OpenTelemetry instrumentation instead of custom timing response payloads.
- Use Jaeger for distributed traces, Prometheus for aggregate metrics, and
   Grafana for operational dashboards. Keep the custom request-flow UI.
- Use standard HTTP instrumentation metrics only; do not add custom application
  counters or histograms. Use traces for per-request policy and replica evidence.
- Prefer established OSS libraries for visualization. Use React Flow
   (`@xyflow/react`) for graph nodes, handles, edge routing, and viewport behavior;
   do not hand-roll SVG connectors or a graph engine.

## Selected Architecture

```text
Request path:
Browser -> BFF -> Trapigo -> selected Go orders replica

Telemetry path:
Go OTel SDKs -> OTel Collector -- traces --> Jaeger
                                       `-- metrics endpoint <-- Prometheus scrapes

Visualization:
Custom dashboard -> authenticated BFF trace lookup -> Jaeger query API
Grafana -> Prometheus metrics + Jaeger traces
```

| Component | Responsibility |
| --- | --- |
| OpenTelemetry API/SDK and instrumentation | Record spans, metrics, context, and attributes |
| OTel Collector | Receive OTLP, batch/filter/export traces, and expose metrics for scraping |
| Jaeger | Store traces, provide trace queries, and offer a separate debugging UI |
| Prometheus | Scrape/store metrics and evaluate aggregate queries |
| Grafana | Operational views of metrics and traces from configured data sources |
| Custom dashboard | Send requests and visualize their actual path, policies, replica, and timings |

The Collector is plumbing for the selected stack, not another dashboard or
trace store. SDKs send OTLP to one configured endpoint; the Collector routes
each signal to its destination. Prometheus does not receive traces, and Jaeger
does not serve as the metrics store. Grafana queries both backends.

## Complexity

- HTTP instrumentation: Go `otelhttp` covers the gateway and orders service.
   The BFF forwards requests and reads traces without recording its own spans.
- Gateway policies: small custom OTel spans and attributes, not manual duration
   calculations or header/trailer timing transport.
- Telemetry infrastructure: moderate configuration work for Compose, the
   Collector, Jaeger storage/query, Prometheus scraping, and Grafana provisioning.
- Custom UI integration: moderate work for secure trace lookup, asynchronous
   arrivals, span normalization, and the responsive animated visualization.
- Browser and BFF tracing are deliberately excluded. The visualization focuses
   on Trapigo API Gateway and its selected orders replica.

Overall this is a moderate multi-service feature. OTel simplifies measurement
and correlation but does not remove trace retrieval and UI work. The previous
custom timer/header/buffering approach is superseded by this architecture.

## What Timings Mean

Use OTel span durations within their defined boundaries. Do not subtract
wall-clock timestamps from different containers to infer network latency.

| Display label | Measurement boundary | Includes |
| --- | --- | --- |
| Gateway request | Gateway HTTP server span | Gateway processing and downstream work; not browser/BFF time |
| Trapigo -> replica | Outgoing proxy HTTP span | Service processing and transport overhead within the instrumentor's boundary |
| Gateway policies | Local child spans for each policy | Work performed inside that policy only |
| Service processing | Orders service HTTP server span | Handler work including database calls; not client network transit |

These durations are nested. Never add them together as if they were sequential
segments: the gateway request already includes the downstream durations.

For example, a 30 ms gateway-to-replica round trip with 20 ms measured service
processing does not prove 10 ms of pure network latency. The remainder can
include buffering, response transfer, scheduling, and unmatched measurement
boundaries. If displayed, label it as an overhead estimate, not network latency.

Verify instrumentation boundaries with delayed-header and delayed-body tests.
Different HTTP client instrumentors can finish at different points. Read and
close response bodies; label spans accurately rather than claiming every span
is a full body round trip. Where needed, use a surrounding OTel request span
through body completion, not a custom timing header or buffered gateway response.

Completed traces arrive asynchronously after span export and ingestion. Animate
the observed request after retrieving its evidence; show waiting, partial,
sampled-out, and unavailable states. Do not pretend the animation is live tracing
or include animation or trace-query delays in the request's duration.

## Existing Implementation Anchors

- `frontend-dashboard/src/routes/_authenticated/dashboard.tsx`: placeholder UI.
- `frontend-dashboard/src/server/auth-gateway.server.ts`: gateway origin,
  authentication-cookie filtering, timeout, and refresh-cookie handling.
- `frontend-dashboard/src/server/auth.functions.ts`: TanStack server functions.
- `trapigo/internal/app/gateway/bootstrap/gateway_setup.go`: route matching,
  `RoundRobinAtomic`, and reverse proxy dispatch.
- `trapigo/internal/features/middleware/transporthttp/rate_limit.go`: IP-based
  token bucket.
- `trapigo/internal/features/auth/middleware/authentication.go`: actual
  authentication outcome.
- `trapigo/internal/features/middleware/transporthttp/logging.go`: existing
  gateway duration logging, currently not returned to the dashboard.
- `buildingblocks/middleware/logging.go`: service duration logging and request
  ID generation, currently not a cross-service trace system.
- `go-order-service/cmd/app/main.go`: service logging middleware registration.
- `trapigo/configs/trapigo-gateway.yaml`: Docker/Compose gateway config with
  three explicit Go container upstreams.
- `trapigo/configs/trapigo-gateway-kube.yaml`: Kubernetes gateway config that
  discovers ready Go Pod endpoints from EndpointSlices for Trapigo's own
  round-robin pool.

Actual processing order is logging, rate limit, CSRF/origin checks,
authentication, route matching, round-robin selection, and proxying.

The YAML config uses `refill-rate: 1.67`, while the config and limiter fields
are integers. Verify decoding and preserve fractional refill using `float64`
with a regression test. Do not assume this already causes startup failure.

## Phase 1: Custom Dashboard Foundation

1. Replace the placeholder with an unframed flow diagram and compact controls.
   Use React Flow (`@xyflow/react`) for the diagram, its built-in handles and
   edges for connections, and existing shadcn primitives, Tailwind styling, and
   lucide icons for node content and controls. Reuse built-in animated edges
   where appropriate; do not manually implement connector geometry. Lock node
   editing for this showcase; enable accessible viewport controls as needed.
2. Show gateway policies in execution order. Display three initial replica
   slots as configured topology, without fabricated health indicators. Prepare
   the layout to align them with actual replica metadata in Phase 6.
3. Add timing, result, bounded recent-history, and expandable parent/child
   timeline areas. Show an explicit not-connected state, unavailable timings,
   and no observed selection. Keep traffic controls disabled until Phase 6.
   Do not simulate application traffic, routing, or replica health.
4. Use responsive desktop and mobile layouts with stable nodes, readable
   timing labels, keyboard-accessible controls, and no content overlap. Use
   explicit positions for the small fixed topology and React Flow's viewport
   fitting; do not build a custom layout engine. If topology becomes dynamic,
   evaluate an established layout library such as ELK or Dagre.
5. Respect reduced motion and verify desktop/mobile layout and accessibility.
   This phase delivers the disconnected UI shell, not working requests or
   telemetry-backed animations.

## Phase 2: UI Data Contract

1. Define shared, bounded types for the real request result and trace evidence:
   correlated trace ID, HTTP status, retry information, trace availability,
   parent/child relationships, policy outcomes, selected public replica,
   delivery evidence, and span durations with explicit measurement boundaries.
2. Represent not-connected, in-flight, waiting-for-trace, partial, available,
   and unavailable states explicitly. Missing evidence must not imply a
   successful policy, selected destination, delivery, or zero-duration result.
3. Connect presentational components to the contract without adding network
   calls or enabling traffic controls. Use test-only fixtures to verify observed
   routing, rejection, failure, and incomplete-data visual states; never use
   fixtures as runtime fallback data or simulated application traffic.
4. Keep backend URLs, sensitive attributes, and raw order data out of the
   browser contract. Phase 5 must normalize authorized trace evidence into
   this contract; Phase 6 wires the real request lifecycle.

## Phase 3: Observability Infrastructure

1. Add version-pinned Jaeger, Prometheus, Grafana, and OTel Collector services
   and task-specific configuration files to Compose. Do not change order-service
   replication or unrelated databases. Use available ports; preserve existing
   gateway ports 80/8080 and frontend port 3000.
2. Configure Collector OTLP receivers, batching, bounded queues, and Jaeger OTLP
   trace export. Expose a Prometheus exporter endpoint for OTLP metrics. Preserve
   service and replica identity when converting metrics; verify distinct SDK
   producers do not collapse into the same Prometheus series.
3. Configure Prometheus scraping and retention. Keep metric label cardinality
   bounded: no trace IDs, request IDs, usernames, raw URLs, or dynamic order IDs
   as metric labels.
4. Provision Grafana Prometheus and Jaeger data sources plus an initial gateway
   dashboard. Use the standard HTTP instrumentation and Collector exporter
   metric names actually observed, not assumed names. Include traffic, error
   rates, and duration percentiles where supported. Do not require dedicated
   rate-limit outcome or selected-replica metric panels.
5. Use bounded local trace retention and 100% trace sampling for the development
   showcase; make production sampling/storage configurable. Keep ingestion and
   query endpoints internal except deliberate local debugging access. Bind
   local-only monitoring UI ports to loopback.
6. Export telemetry asynchronously. Collector/backend outages must not block
   normal gateway traffic; configure timeouts and bounded resource use. Document
   shutdown flushing and failure behavior.

## Phase 4: Go Gateway and Service Instrumentation

1. Add fractional-refill regression coverage and fix precision if confirmed.
2. Initialize OTel SDK resources and OTLP exporters in the gateway and orders
   service. Use stable service names and unique `service.instance.id` values per
   replica; apply deployment/environment attributes without leaking credentials.
3. Install W3C TraceContext propagation. Wrap gateway and service HTTP handlers
   using `otelhttp.NewHandler` and the proxy transport using
   `otelhttp.NewTransport`. Preserve context through existing middleware and
   reverse proxy rewrites; verify the downstream receives the correct parent.
4. Add child spans for rate limiting, authentication, route matching, and
   round-robin selection. Instrument each operation's actual work, not the whole
   next-handler chain as if it were policy duration.
5. Record admission/rejection/disabled outcomes, configured replica count,
   matched service/route, and actual selected public replica label. Distinguish
   gateway rejection from upstream 401/429. Span status alone is not a policy
   outcome; mark explicit attributes/events.
6. Rely on supported standard HTTP instrumentation metrics; do not add custom
   request, policy-outcome, or backend-selection counters or policy-duration
   histograms. Keep metric labels low-cardinality; exclude tokens, cookies,
   order bodies, and sensitive headers from both traces and metrics.
7. Sanitize trace attributes before returning them to the custom dashboard.
   Do not change round-robin rules or manufacture backend health. Preserve
   normal streaming, proxy cancellation, and response error handling.

## Phase 5: Fixed Request and Secure Gateway Trace Retrieval

1. Keep the browser and BFF uninstrumented. Use the existing Go telemetry
   pipeline. Generate server-owned W3C correlation context for the fixed orders
   request so the BFF can retrieve its gateway trace without recording a BFF
   span. The remote correlation parent is not exported.
2. Wire the Send Request button to a TanStack server function that calls a fixed,
   server-only `GET /api/v1/go-orders` helper. The server-function invocation is
   the browser-to-BFF mechanism; it does not change the upstream HTTP method,
   which must remain GET. Choose and verify the supported invocation method for
   the installed TanStack Start version rather than assuming it must be POST.
   Forward only auth cookies and generated trace context, preserve rotated
   cookies, use timeout/no-store/no redirects, and consume the body with a size
   bound without returning order data merely to show the flow.
3. The protected orders call must authenticate at the gateway on every request.
   Do not rely only on the UI route guard or add a hidden session check that
   changes the displayed traffic/quota. Set private/no-store and `Vary: Cookie`.
4. Return the correlated trace ID, real HTTP status, and retry information
   immediately; do not wait for export before replying. Do not interpret every
   upstream 401 as session expiry.
5. Add an authenticated BFF trace-query operation using Jaeger's documented,
   versioned query API, not its unofficial internal UI API. Bound query size,
   timeout, polling attempts, and returned fields.
6. For the single-user local demo, retain only the latest trace in one expiring
   in-memory record. Reverify login through Trapigo on trace lookup, not each
   Send Request. Do not fingerprint cookies or maintain BFF session ownership.
   A new request replaces the previous active trace. Any authenticated user who
   knows the active ID can read it; this intentionally does not support a shared
   or public dashboard. Add per-session authorization and suitable storage before
   supporting concurrent users or multiple BFF replicas. Do not expose unrestricted
   trace search or accept a client-chosen backend URL.
7. Normalize spans into the Phase 2 bounded UI contract with parent relationships,
   durations, policy evidence, and public replica labels. Missing/incomplete
   traces remain explicit, not fabricated destinations or zero-duration success.
8. Normalize only gateway and orders spans. Treat the gateway as the root of
   the displayed scope; absent browser/BFF spans are intentional, not partial
   evidence. Do not label gateway duration as browser round-trip time.
9. Keep trace queries separate from the showcased request. No browser telemetry
   ingress, Node OTel dependencies/preload, or Collector host port is needed.

### Phase 5 implementation and verification status

- Implemented GET server-function invocation and the fixed upstream GET,
  server-owned trace correlation, single-active-request storage, authenticated
  Jaeger v3 lookup, and sanitized gateway/orders normalization.
- Phase 6 now enables Send Request and renders the returned evidence.
  No orders request is issued on page load.
- Browser/BFF tracing and telemetry ingress have been removed, along with the
  preload, direct Node OTel dependencies, and Collector host-port change.
- Custom TanStack Start global middleware has been removed. Server functions use
  the framework's normal setup; gateway policies remain Trapigo's responsibility.
  Jaeger lookup revalidates login through Trapigo and only reads the active demo
  trace. Cookie fingerprinting and per-session ownership have been removed for
  the single-user local demo. A new request replaces the previous trace.
- Live Go server span names are route-based. Authenticated gateway-to-orders
  delivery and cookie-refresh smoke checks remain pending before Phase 5 sign-off.
  No Collector configuration change or restart is required for the dashboard.
- Gateway-only typecheck, Biome, the 11 existing frontend tests, and production
  build pass. Inline checks cover a complete gateway/orders trace, disconnected
  partial evidence and fixed transport. A live gateway 401 is
  retrieved from Jaeger v3 and recognized as complete rejection evidence without
  requiring browser/BFF spans.

## Phase 6: Real-Data Dashboard Integration

1. Connect the Phase 1 UI through the Phase 2 contract to the real request and
   protected trace-query operations from Phase 5. Enable Send Request and
   Start/Stop Traffic only when the real request path is connected.
2. Populate gateway policy outcomes and align replica slots with actual replica
   metadata when available. Preserve configured topology without inventing
   observed routing or backend health.
3. Show an in-flight state without guessing the destination. On completion,
   animate the observed path, selected replica, and return path. Stop rejected
   flows at the reported stage; distinguish selection from successful delivery.
4. Display timings derived from the gateway request, policies, proxy outbound
   request, and service processing
   durations. Respect the verified boundaries; show unavailable for missing data.
   Add a compact parent/child span timeline for deeper inspection.
5. Serialize single and continuous requests. Disable overlapping sends, pause
   while the tab is hidden, clean up timers on unmount, and suppress stale
   results. Stop prevents future sends; an issued request can finish.
6. Back off on 429 using `Retry-After`; stop on session rejection or connection
   failure. Respect reduced motion. Use no artificial backend delays.
7. Retain the latest five results and observed counts for the three configured
   replica labels in page-local React state. Only the active trace is queried;
   do not query historical IDs after the next request replaces the BFF record.
   Counts reset on reload/unmount and are not global gateway metrics.
8. Reverify the Phase 1 responsive layout and accessibility with real trace data,
   timing labels, history, and error states; prevent overlap and overflow.
9. Show a waiting-for-trace state after the real response, with bounded retry and
    partial/unavailable results. Count the business request once regardless of
    polling. Grafana and Jaeger UIs remain separate optional debugging views.

### Phase 6 simplifications and behavior

- No browser/BFF tracing, session fingerprinting, multi-user registry, custom
  startup middleware, or Collector changes. Gateway/orders evidence only.
- One complete request/lookup cycle at a time; up to ten trace reads separated
  by 1.5 seconds. Continuous traffic waits at least five seconds between cycles
  and honors `Retry-After`; this is pacing, not an automatic business retry.
- Trace reads revalidate login at Trapigo, so auth checks contribute real traffic
  to the shared gateway IP quota. Hidden tabs pause sends and future lookups.
- Stop allows the issued cycle to finish; unmount cancels local waits and ignores
  late responses. No new orders call is issued to retrieve missing traces.
- Policy timing labels and parent IDs are sanitized from known producers.
  Evidence highlights are finite and reduced-motion safe. Missing logging spans,
  replica health, service delivery, and timings are not invented.
- Keep the original bottom-to-top graph handles and vertical topology. Return
  evidence animates backward on those same edges, without extra side connectors.
- Implemented controls, live result states, page-local history/counts, and the
  gateway-only span inspector. Browser checks against local mock gateway/Jaeger
  cover delayed arrival, rejection, downstream 401, ten-query limits, partial
  delivery, outage, expired login, Stop, hidden-tab pause, navigation cleanup,
  mobile overflow, and reduced motion. These are not an authenticated smoke
  against the real Compose stack; that remains a Phase 7 handoff prerequisite.

## Phase 7: Documentation and Handoff

- Document Compose startup, instrumentation env vars, local monitoring URLs,
   retention, sampling, trace availability delay, and metrics semantics.
- Explain the distinction between custom page-local showcase counts and
   Grafana's aggregate metrics. Do not equate sampled trace counts with totals.
- Provide a Grafana dashboard provisioned from actual exported signals and
   document trace navigation/correlation only where configured and verified.
- Limit shared `buildingblocks` changes to reusable instrumentation that is
   needed; do not rewrite unrelated logging or service business code.
- Start the frontend dev server on an available port and provide its URL after
   implementation and verification.

## Verification

- Three test upstreams: confirm span replica labels match actual selection,
  including pool wraparound, concurrent requests, and unrelated traffic.
- Policy tests: gateway rejection, upstream 401/429, disabled policies,
  telemetry disabled, unavailable upstream, and untrusted incoming trace context.
- Timing tests: use controlled upstream delays, including delayed headers and
  a delayed body; assert completion includes body time with tolerant bounds.
- Transport tests: body bounds, timeout, cancellation, trace propagation,
   cookie allowlists, refresh cookies, and malformed/incomplete trace data.
- Use in-memory OTel exporters for focused tests; assert span parentage, ended
   spans, policy outcomes, and selected replica independently of Jaeger.
- Trace lookup tests: unauthenticated access, inactive/expired trace IDs,
   bounded polling, sanitized fields, telemetry outages, and query timeout.
   Per-user isolation is deliberately excluded from this single-user local demo.
- Metrics tests: verify scraper targets, actual standard HTTP metric names and
  availability, and bounded labels. Do not require custom counter or histogram
  increments.
- Compose validation: Collector receives both signals, Jaeger returns traces,
   Prometheus scrapes successfully, and Grafana provisions both data sources.
- UI tests: unavailable metadata, failures, expired session, 429 backoff,
  serialized traffic, stop behavior, unmount cleanup, and reduced motion.
- Live smoke: actual login and several requests through the Compose stack;
  verify observed routing and timing without requiring a fixed starting replica.
- Inspect desktop/mobile screenshots for connector alignment and overflow.
- Run frontend Node tests, typecheck, changed-file lint, build, and focused Go
  tests. Start the dev server on an available port for handoff.

## Scope and Risks

- Keep round-robin and IP-based rate-limit behavior unchanged. BFF requests
  may share an IP bucket; do not label quotas as per-user.
- Other traffic can change the selection sequence and consume rate tokens.
- Do not add simulated traffic, burst generation, health checks, dynamic
   scaling, order CRUD, or log aggregation infrastructure. Standard HTTP
   instrumentation metrics and distributed tracing are in scope; custom
   application metrics are deferred.
- Do not buffer gateway responses or add custom timing headers/trailers merely
   to measure requests; completed OTel spans travel through the telemetry path.
- Telemetry is eventually available and can be incomplete or sampled out.
   Normal requests must succeed independently of monitoring infrastructure.
- Grafana does not store the traces or metrics itself. Configure Jaeger and
   Prometheus as separate data sources; Grafana trace navigation is not automatic.
- Existing logged durations are useful anchors, not already available
  dashboard telemetry. True service timing expands scope into the service.
- Leave unrelated changes and bugs untouched; document blockers separately.

## Selected Implementation Order

Implement the numbered phases sequentially. Each phase is a separate development
unit; run its relevant verification checks before proceeding. The UI-first order
does not change the request or telemetry architecture.

1. Custom dashboard foundation.
2. UI data contract and test-only visual fixtures.
3. Observability infrastructure.
4. Go gateway and service instrumentation.
5. Fixed BFF request forwarding and protected gateway trace retrieval.
6. Real-data dashboard integration and observed-path animations.
7. End-to-end verification, documentation, and handoff.

The OTel/Jaeger/Prometheus/Grafana architecture replaces the earlier manual
timing plan. This file records the agreed direction; it does not claim that
infrastructure or application changes have already been implemented.