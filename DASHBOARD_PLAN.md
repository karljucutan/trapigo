# Dashboard Request Flow Showcase Plan

Status: observability stack selected; application implementation not started.

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
- Show total request-response time and timings between request boundaries.
- Preserve the existing dashboard's authentication and design conventions.
- Save this plan before making application changes.
- Use OpenTelemetry instrumentation instead of custom timing response payloads.
- Use Jaeger for distributed traces, Prometheus for aggregate metrics, and
   Grafana for operational dashboards. Keep the custom request-flow UI.
- Prefer established OSS libraries for visualization. Use React Flow
   (`@xyflow/react`) for graph nodes, handles, edge routing, and viewport behavior;
   do not hand-roll SVG connectors or a graph engine.

## Selected Architecture

```text
Request path:
Browser -> BFF -> Trapigo -> selected Go orders replica

Telemetry path:
OTel SDKs -> OTel Collector -- traces --> Jaeger
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

- HTTP instrumentation: relatively straightforward with Go `otelhttp` and
   supported Node HTTP/fetch instrumentation. Verify Node fetch/Undici support
   rather than assuming an HTTP-only instrumentor covers fetch.
- Gateway policies: small custom OTel spans and attributes, not manual duration
   calculations or header/trailer timing transport.
- Telemetry infrastructure: moderate configuration work for Compose, the
   Collector, Jaeger storage/query, Prometheus scraping, and Grafana provisioning.
- Custom UI integration: moderate work for secure trace lookup, asynchronous
   arrivals, span normalization, and the responsive animated visualization.
- Browser tracing: additional setup to measure the actual browser round trip;
   Go-only instrumentation cannot cover the BFF or browser boundary.

Overall this is a moderate multi-service feature. OTel simplifies measurement
and correlation but does not remove trace retrieval and UI work. The previous
custom timer/header/buffering approach is superseded by this architecture.

## What Timings Mean

Use OTel span durations within their defined boundaries. Do not subtract
wall-clock timestamps from different containers to infer network latency.

| Display label | Measurement boundary | Includes |
| --- | --- | --- |
| Total request-response | Browser request/interaction span through resolved result | Nested calls, serialization, and client/server overhead |
| BFF -> Trapigo | Outgoing BFF HTTP span | Gateway processing, downstream work, and transport overhead within the instrumentor's boundary |
| Trapigo -> replica | Outgoing proxy HTTP span | Service processing and transport overhead within the instrumentor's boundary |
| Gateway policies | Local child spans for each policy | Work performed inside that policy only |
| Service processing | Orders service HTTP server span | Handler work including database calls; not client network transit |

These durations are nested. Never add them together as if they were sequential
segments: the browser total already includes the downstream durations.

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
- `trapigo/configs/trapigo-gateway.yaml`: three explicit Go service upstreams.

Actual processing order is logging, rate limit, CSRF/origin checks,
authentication, route matching, round-robin selection, and proxying.

The YAML config uses `refill-rate: 1.67`, while the config and limiter fields
are integers. Verify decoding and preserve fractional refill using `float64`
with a regression test. Do not assume this already causes startup failure.

## Phase 1: Observability Infrastructure

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
   dashboard. Use the Collector exporter metric names actually observed, not
   assumed names. Include traffic, error rates, duration percentiles, rate-limit
   admissions/rejections, and selected-replica distribution.
5. Use bounded local trace retention and 100% trace sampling for the development
   showcase; make production sampling/storage configurable. Keep ingestion and
   query endpoints internal except deliberate local debugging access. Bind
   local-only monitoring UI ports to loopback.
6. Export telemetry asynchronously. Collector/backend outages must not block
   normal gateway traffic; configure timeouts and bounded resource use. Document
   shutdown flushing and failure behavior.

## Phase 2: Go Gateway and Service Instrumentation

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
6. Add counters for requests, policy outcomes, and backend selections plus
   duration histograms as needed alongside supported HTTP instrumentor metrics.
   Keep low-cardinality labels; exclude tokens, cookies, order bodies, and
   sensitive headers from both traces and metrics.
7. Sanitize trace attributes before returning them to the custom dashboard.
   Do not change round-robin rules or manufacture backend health. Preserve
   normal streaming, proxy cancellation, and response error handling.

## Phase 3: BFF, Browser, and Secure Trace Retrieval

1. Initialize Node OTel before instrumented modules load. Verify TanStack Start
   server request and outgoing fetch instrumentation; cover unsupported boundaries
   with explicit OTel spans. Propagate trace context to Trapigo.
2. Add a fixed `GET /api/v1/go-orders` server-only helper and POST server function.
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
6. Authorize trace access independently of knowing its ID. Bind issued showcase
   traces to the authenticated session/user, including gateway-rejected requests.
   Use an expiring bounded registry for a single-BFF demo; use a shared store for
   multiple BFF instances. No unrestricted trace search or client-chosen backend
   URL. Verify refreshed session identity and avoid cross-user data exposure.
7. Normalize spans into a bounded UI contract with parent relationships,
   durations, policy evidence, and public replica labels. Missing/incomplete
   traces remain explicit, not fabricated destinations or zero-duration success.
8. Use browser OTel request/interaction spans to include the actual browser
   boundary. Keep trace-query polling and animation outside the request span.
   Define a same-origin, authenticated, size/rate-bounded telemetry ingress or
   equivalent verified transport for browser export; do not publicly expose the
   Collector. Treat browser telemetry as untrusted for authorization and routing.
9. Ensure trace-query requests and telemetry-ingress traffic do not pollute the
   showcased business trace or create recursive telemetry/export behavior.

## Phase 4: Custom Dashboard

1. Replace the placeholder with an unframed flow diagram and compact controls.
   Use React Flow (`@xyflow/react`) for the diagram, its built-in handles and
   edges for connections, and existing shadcn primitives, Tailwind styling, and
   lucide icons for node content and controls. Reuse built-in animated edges
   where appropriate; do not manually implement connector geometry. Lock node
   editing for this showcase; enable accessible viewport controls as needed.
2. Show gateway policies in execution order. Display three initial replica
   slots as configured topology, without fabricated health indicators; align
   them with actual replica metadata when available.
3. Show an in-flight state without guessing the destination. On completion,
   animate the observed path, selected replica, and return path. Stop rejected
   flows at the reported stage; distinguish selection from successful delivery.
4. Display timings derived from the corresponding OTel spans on BFF/gateway and
   gateway/replica connections, plus total request-response and service processing
   durations. Respect the verified boundaries; show unavailable for missing data.
   Add a compact parent/child span timeline for deeper inspection.
5. Serialize single and continuous requests. Disable overlapping sends, pause
   while the tab is hidden, clean up timers on unmount, and suppress stale
   results. Stop prevents future sends; an issued request can finish.
6. Back off on 429 using `Retry-After`; stop on session rejection or connection
   failure. Respect reduced motion. Use no artificial backend delays.
7. Show bounded recent history and session-local per-replica selection counts.
   Do not claim these are global gateway metrics.
8. Use responsive desktop and mobile layouts with stable nodes, readable
   timing labels, keyboard-accessible controls, and no content overlap. Use
   explicit positions for the small fixed topology and React Flow's viewport
   fitting; do not build a custom layout engine. If topology becomes dynamic,
   evaluate an established layout library such as ELK or Dagre.
9. Show a waiting-for-trace state after the real response, with bounded retry and
    partial/unavailable results. Count the business request once regardless of
    polling. Grafana and Jaeger UIs remain separate optional debugging views.

## Phase 5: Documentation and Handoff

- Document Compose startup, instrumentation env vars, local monitoring URLs,
   retention, sampling, trace availability delay, and metrics semantics.
- Explain the distinction between custom session-local showcase counts and
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
- Trace lookup tests: unauthenticated access, cross-user IDs, expired registry,
   bounded polling, sanitized fields, telemetry outages, and query timeout.
- Metrics tests: verify scraper targets, exported metric names, counter increments,
   histogram semantics, and distinct replica series without high-cardinality labels.
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
   scaling, order CRUD, or log aggregation infrastructure. Aggregate metrics and
   distributed tracing are now explicitly in scope.
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

1. Provision Collector, Jaeger, Prometheus, and Grafana.
2. Instrument the Go gateway and orders service; verify traces and metrics.
3. Instrument the BFF/browser and implement protected trace retrieval.
4. Build the custom request-flow UI using actual trace data.
5. Verify the complete workflow and document the monitoring views.

The OTel/Jaeger/Prometheus/Grafana architecture replaces the earlier manual
timing plan. This file records the agreed direction; it does not claim that
infrastructure or application changes have already been implemented.