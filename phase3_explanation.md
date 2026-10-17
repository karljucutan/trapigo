# Phase 3: Observability Infrastructure — Learning Guide

This guide explains what Phase 3 configured, why each piece is needed, how to
read its YAML, and how the services fit together. It is written as a
configuration walkthrough rather than assuming prior OpenTelemetry experience.

## Why use exact versions instead of the `latest` tag?

Use current compatible releases, but pin their exact versions. As checked on
2026-10-17, this setup uses Jaeger 2.20.0, Collector Contrib 0.163.1,
Prometheus 3.15.0, and Grafana 13.2.4. The latter three are the latest upstream
non-prerelease releases at that check.
The Collector still uses a `0.x` version; "stable release" here means the
upstream release is not marked as a prerelease, not that every Collector
component has the same stability guarantee.

A moving `latest` tag can point to different software tomorrow. An exact tag
keeps the chosen release explicit and makes it possible to validate upgrades
before changing the stack. Tags are not immutable content hashes; digest
pinning would be the next step if stronger artifact reproducibility is needed.

This upgrade illustrates why configuration must be reviewed along with image
versions: Jaeger v2 uses a YAML configuration and a different image repository
instead of the old all-in-one image and v1 flags/environment variables.

It also illustrates why "latest stable" does not guarantee compatibility
between projects. Jaeger 2.22.0 is the newest upstream release, but both 2.21.0
and 2.22.0 have removed `/api/services`. Grafana 13.2.4's bundled Jaeger plugin
13.0.5 still requires that endpoint, causing its data-source health check to
fail with 404 against 2.22.0. We therefore pin Jaeger to 2.20.0, the newest
release retaining that endpoint, while keeping the other components current.
Revisit this exception when Grafana's plugin supports Jaeger's newer API.
The future BFF trace lookup should still use Jaeger's documented v3 API;
Grafana's legacy dependency is not a reason to copy it into application code.

## What Phase 3 does (and does not do)

Phase 3 provides the observability backends and the plumbing between them:

```text
                         ┌── traces ──► Jaeger
Trapigo and orders ─OTLP─► OTel Collector
                         └── metrics ─► Prometheus ──► Grafana
                                              Jaeger ──► Grafana
```

The Go gateway and orders service are not sending telemetry yet. Their
instrumentation, trace propagation, and application metrics are Phase 4. As a
result, the infrastructure configuration can be validated now, but seeing
real business traces and populated application panels requires later phases.

The tools have separate responsibilities:

| Component | What it does | Useful mental model |
| --- | --- | --- |
| OpenTelemetry SDKs (Phase 4) | Create traces and metrics in the applications and send them using OTLP. | Producers |
| OTel Collector | Receives OTLP, processes it, and routes each signal to an exporter. | Router and processing pipeline |
| Jaeger | Stores and searches traces. | Trace store and trace UI |
| Prometheus | Scrapes and stores time-series metrics. | Metrics store |
| Grafana | Queries data sources and displays dashboards. | Viewer; it is not the trace or metrics store |

Traces and metrics answer different questions. A trace follows one request
through services and spans. A metric aggregates measurements over time, such as
requests per second or a duration histogram. Prometheus does not store traces;
Jaeger does not store the Prometheus metrics in this design.

## Recommended configuration and learning order

There are two distinct orders: the order that makes the configuration easiest
to understand, and the startup dependencies in Compose.

### Learning/configuration order

1. **Decide where each signal will land.** This setup stores traces in Jaeger
   and metrics in Prometheus.
2. **Configure Jaeger.** The Collector needs a trace receiver to export to.
   Decide the local storage behavior and how many traces to retain.
3. **Configure the Collector.** Define the OTLP receivers, processing steps,
   and the trace and metric exporters. The trace exporter points at Jaeger; the
   metrics exporter exposes an endpoint for Prometheus to scrape.
4. **Configure Prometheus.** Add a scrape target for the Collector's metrics
   endpoint.
5. **Configure Grafana.** Add Prometheus and Jaeger data sources, then load a
   dashboard that queries those data sources.
6. **Instrument the applications.** In Phase 4, configure the gateway and
   orders service SDKs to send OTLP to the Collector.

The Collector is central but need not be the first thing configured: its
export destinations should be understood first. In practice you will often
iterate among the files as a set, because endpoint names and ports must agree.

### Compose startup order

Compose's `depends_on` relationships in this setup are:

```text
Jaeger ──► OTel Collector ──► Prometheus
   └────────────────────────► Grafana
Prometheus ─────────────────► Grafana
```

The arrows mean “start this dependency before starting the dependent
container.” They do **not** mean “wait until the dependency is healthy and
ready.” These services do not currently define health checks. The Collector's
export retry helps with a temporarily unavailable Jaeger, but `depends_on`
alone is not a readiness guarantee.

## YAML basics used by these configurations

YAML represents structured data using indentation:

- `key: value` is a mapping entry. For example, `job_name: otel-collector`.
- A newline followed by a more-indented mapping groups child settings under
  their parent. For example, `global:` contains `scrape_interval` and
  `evaluation_interval`.
- `- value` starts a list item. For example, each entry under `scrape_configs`
  is a scrape job.
- Indentation is meaningful. Use spaces consistently; do not use tabs.
- Strings that contain punctuation or could be interpreted as another type
  are often quoted. For example, `"false"` is an environment-variable string;
  without quotes, YAML would parse `false` as a boolean.
- `[a, b]` is a compact YAML list. It is equivalent to a multi-line list.
- `#` starts a comment when it is outside a quoted string.

YAML syntax only describes values. Each application (Compose, Prometheus, the
Collector, Grafana) defines the meaning and allowed fields of its own YAML
configuration.

## Walkthrough of the YAML files

### 1. `compose.yml`: run and connect the containers

[compose.yml](./compose.yml) is the top-level run configuration. The new
services are below the existing application/database services.

#### Jaeger

```yaml
jaeger:
  image: docker.io/jaegertracing/jaeger:2.20.0
  command: ["--config=/etc/jaeger/config.yaml"]
  environment:
    JAEGER_MEMORY_MAX_TRACES: "${JAEGER_MEMORY_MAX_TRACES:-10000}"
  volumes:
    - ./observability/jaeger-config.yaml:/etc/jaeger/config.yaml:ro,Z
  ports:
    - "127.0.0.1:16686:16686"
```

- `image` pins the Jaeger version instead of using a moving `latest` tag.
- `command` loads Jaeger v2's YAML config from the mounted file.
- `JAEGER_MEMORY_MAX_TRACES` supplies a default trace-count limit of 10,000.
  `${NAME:-default}` is Compose interpolation: use `NAME` from the environment
  if set and nonempty; otherwise use `default`.
- The YAML config selects in-memory storage and enables OTLP. The old v1
  `SPAN_STORAGE_TYPE` and `COLLECTOR_OTLP_ENABLED` variables are no longer used.
  Recreating Jaeger loses its traces.
- `ports` publishes Jaeger's web UI on host port 16686. The `127.0.0.1` prefix
  binds it only on the local machine; the value after the colon is the
  container port.

In a mapping like `environment`, quoted values are strings because environment
variables are strings. Do not assume that setting an environment variable
configures a similarly named option in every version of an image; check that
image's version-specific documentation when changing it.

#### OTel Collector

```yaml
otel-collector:
  image: docker.io/otel/opentelemetry-collector-contrib:0.163.1
  command: ["--config=/etc/otelcol-contrib/config.yaml"]
  stop_grace_period: 30s
  volumes:
    - ./observability/otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml:ro,Z
  depends_on:
    - jaeger
```

- `image` pins the Collector Contrib distribution, which includes the
  configured exporters.
- `command` tells the Collector where its in-container configuration file is.
- `volumes` mounts the repository file into the container. The left side is
  the host path; the right side is the container path. `ro` makes the mount
  read-only. `Z` is a Podman/SELinux relabel option for the mounted path.
- `depends_on` starts Jaeger before the Collector, but does not wait for
  Jaeger readiness.
- `stop_grace_period` gives the Collector up to 30 seconds to shut down
  cleanly. It cannot make the in-memory queue durable.

There is no `ports` entry here. Containers in the Compose network can reach
the Collector, but OTLP ingestion is not published on the host.

#### Prometheus

```yaml
prometheus:
  image: docker.io/prom/prometheus:v3.15.0
  command:
    - "--config.file=/etc/prometheus/prometheus.yml"
    - "--storage.tsdb.path=/prometheus"
    - "--storage.tsdb.retention.time=${PROMETHEUS_RETENTION_TIME:-7d}"
    - "--storage.tsdb.retention.size=${PROMETHEUS_RETENTION_SIZE:-2GB}"
  volumes:
    - ./observability/prometheus.yml:/etc/prometheus/prometheus.yml:ro,Z
    - prometheus-data:/prometheus
  ports:
    - "127.0.0.1:9090:9090"
  depends_on:
    - otel-collector
```

- `command` is a YAML list of arguments passed to Prometheus. It identifies
  the config file, data directory, and two retention limits.
- Retention defaults to 7 days or 2 GB. Whichever configured limit is reached
  first can constrain stored history.
- The first volume mounts the repository scrape configuration read-only.
- `prometheus-data:/prometheus` is a named volume, so metric data can persist
  across container recreation. `docker compose down -v` removes named volumes;
  use it only when you intend to discard that local data.
- Port 9090 is available to the host only on loopback.
- Prometheus starts after the Collector, because it scrapes the Collector's
  exporter endpoint.

#### Grafana

```yaml
grafana:
  image: docker.io/grafana/grafana:13.2.4
  environment:
    GF_ANALYTICS_REPORTING_ENABLED: "false"
    GF_ANALYTICS_CHECK_FOR_UPDATES: "false"
    GF_SECURITY_ADMIN_USER: "${GRAFANA_ADMIN_USER:-admin}"
    GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_ADMIN_PASSWORD:-admin}"
    GF_USERS_ALLOW_SIGN_UP: "false"
  volumes:
    - ./observability/grafana/provisioning:/etc/grafana/provisioning:ro,Z
    - grafana-data:/var/lib/grafana
  ports:
    - "127.0.0.1:3001:3000"
  depends_on:
    - prometheus
    - jaeger
```

- `environment` turns off Grafana's update/reporting checks, disables public
  sign-up, and sets local admin credentials. The defaults are for local
  development; configure real credentials for a shared environment.
- The provisioning directory is mounted read-only so Grafana can load the
  checked-in data source and dashboard definitions.
- `grafana-data` persists Grafana state.
- Host port 3001 maps to Grafana's container port 3000. This avoids using the
  frontend's host port 3000.
- Grafana depends on both backends because it queries both of them.

At the bottom of the Compose file:

```yaml
volumes:
  prometheus-data:
  grafana-data:
```

declares named volumes. A service volume reference uses the same name, which
lets Compose create and manage the persistent storage.

### 2. `observability/jaeger-config.yaml`: Jaeger v2 storage and ingestion

[jaeger-config.yaml](./observability/jaeger-config.yaml) configures Jaeger v2
using the same receiver/processor/exporter pipeline concepts as the Collector:

- `extensions.jaeger_storage.backends.local.memory` declares an in-memory
  backend named `local`. `max_traces` reads the container environment variable
  using `${env:JAEGER_MEMORY_MAX_TRACES}`. This syntax is interpreted by
  Jaeger's configuration loader, not Compose: the YAML file is bind-mounted,
  rather than embedded in the Compose document.
- `extensions.jaeger_query` makes stored traces queryable and serves the UI.
  `storage.traces: local` points queries at that same backend.
  `http.endpoint: 0.0.0.0:16686` allows connections from Grafana and the
  loopback-published host port.
- `receivers.otlp.protocols.grpc` accepts traces on the internal port 4317.
- `processors.batch` groups spans before writing them to storage.
- `exporters.jaeger_storage_exporter.trace_storage: local` writes spans to
  that backend.
- `service.extensions` activates the storage/query extensions.
  `service.pipelines.traces` wires the receiver, batch processor, and storage
  exporter together.

Here, "export to storage" happens inside Jaeger; this is distinct from the
separate Collector exporting OTLP across the network to Jaeger. No adaptive
sampling processor is configured, so Jaeger does not deliberately discard a
percentage of received spans. Application sampling remains a Phase 4 concern.

### 3. `observability/otel-collector-config.yaml`: receive, process, export

This is a Collector-specific YAML configuration. Its top-level sections
describe available receivers, processors, exporters, and the service pipelines
that connect them.

#### Receivers: how telemetry enters

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

`receivers` defines input plugins. Here there is one receiver named `otlp`,
which accepts either OTLP/gRPC on 4317 or OTLP/HTTP on 4318. `0.0.0.0` means
listen on all interfaces inside the Collector container. Since Compose does
not publish these ports, clients must use the internal service address, such
as `otel-collector:4317` or `otel-collector:4318`.

#### Processors: what happens between receive and export

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
    spike_limit_mib: 64
  batch:
    send_batch_size: 512
    timeout: 5s
```

- `memory_limiter` checks memory every second and applies configured memory
  limits to reduce the chance that telemetry load exhausts the Collector.
- `batch` groups telemetry to send efficiently. It sends when a batch reaches
  the configured size or when the timeout elapses.

Defining a processor here does not automatically apply it. The pipeline must
list it under `processors`.

#### Exporters: where telemetry leaves

The `otlp` exporter sends traces to Jaeger's internal OTLP gRPC address,
`jaeger:4317`. The service name `jaeger` is Compose DNS; containers can use it
without knowing an IP address.

```yaml
  otlp:
    endpoint: jaeger:4317
    tls:
      insecure: true
    timeout: 10s
```

TLS is marked insecure because this is unencrypted local Compose traffic; do
not copy this setting unchanged to a network that requires transport security.
The timeout bounds an export attempt.

The rest of this exporter config enables retry with increasing intervals,
stops retrying after a maximum elapsed time, and queues a bounded amount of
trace data in memory. A bounded queue protects resources but can overflow; an
in-memory queue can be lost at shutdown.

The `prometheus` exporter exposes metrics at `0.0.0.0:8889`. Prometheus scrapes
that endpoint. `resource_constant_labels.included` explicitly permits
`service.name`, `service.namespace`, `service.instance.id`, and
`deployment.environment.name` as resource-derived metric labels. This replaces
the deprecated `resource_to_telemetry_conversion` option in the newer Collector
and preserves service/replica identity without copying every resource attribute.
Request IDs, trace IDs, usernames, and order IDs would create excessive metric
cardinality and must not be added as resource labels or metric attributes.
`translation_strategy: UnderscoreEscapingWithSuffixes` keeps conventional
Prometheus names: punctuation becomes underscores and unit/type suffixes are
added, so a label such as `service.name` becomes `service_name`.

#### Pipelines: wire the pieces together

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
```

A pipeline names a signal type and then selects its receiver(s), processor(s),
and exporter(s). The trace path is:

```text
OTLP receiver → memory limiter → batch → OTLP exporter → Jaeger
```

The metric path is:

```text
OTLP receiver → memory limiter → batch → Prometheus exporter endpoint
```

The bracket syntax such as `[otlp]` is YAML's compact form for a list. The
exporter section name and the pipeline reference have the same name here, but
they do different things: the exporter defines settings; the pipeline wires it
into use.

### 4. `observability/prometheus.yml`: scrape the Collector

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: otel-collector
    static_configs:
      - targets: ["otel-collector:8889"]
```

- `global` sets defaults for Prometheus. It scrapes every 15 seconds and
  evaluates configured rules on the same interval (there are no custom rules
  in this file yet).
- `scrape_configs` is a list of scrape jobs. Each job describes a target set.
- `static_configs` lists targets directly rather than discovering them.
- `otel-collector:8889` is the Collector's service name and internal metrics
  endpoint, not a host-mapped port.

This is a pull model: Prometheus periodically requests metrics from the
Collector. The Collector's Prometheus exporter does not push metrics into
Prometheus.

### 5. Grafana provisioning YAML

#### `datasources.yaml`

The `datasources` list registers two sources:

- Prometheus at `http://prometheus:9090`, marked as Grafana's default source.
- Jaeger at `http://jaeger:16686`.

These URLs use Compose service names and internal container ports. Grafana
queries them server-to-server over the Compose network; the browser only needs
to reach Grafana.

The `uid` values (`prometheus` and `jaeger`) are stable identifiers referenced
by dashboard panels. `editable: false` makes these provisioned sources
read-only through the Grafana UI; change their source configuration in the
checked-in provisioning file.

#### `dashboards.yaml`

The `providers` list tells Grafana to load dashboard files from
`/etc/grafana/provisioning/dashboards`, the path mounted by Compose. It places
them in a `Trapigo` folder. `disableDeletion: true` and `editable: false`
protect provisioned dashboards from changes/deletion in the UI; edit the
checked-in dashboard file instead.

## Is `trapigo-gateway.json` temporary?

No, it is not a temporary runtime export. It is the **checked-in, provisioned
source file** for the initial Trapigo Gateway dashboard. Grafana reads it from
the mounted provisioning directory. Keeping it in version control makes the
dashboard reproducible when the stack is recreated and lets dashboard changes
be reviewed like code.

It is still an **initial scaffold**, not a finished or verified operational
dashboard. Some queries refer to signals expected from Phase 4
instrumentation, such as gateway backend-selection and policy-outcome
counters. They will not show real gateway data until those metrics exist.
HTTP metric names and labels also need to be checked against actual exported
metrics before treating the corresponding panels as correct. The dashboard
description explicitly notes that gateway signals arrive after Phase 4.

The JSON contains Grafana dashboard metadata, panel layout, data-source UIDs,
PromQL queries, legends, units, and display options. Although the other
provisioning files use YAML, Grafana's dashboard provisioning format here is
JSON. `editable: false` also means dashboard edits belong in this file, rather
than being saved only in the Grafana UI. The file can be revised as actual
instrumentation and metrics are verified; it should not be discarded merely
because it starts with placeholder queries.

## All files changed for Phase 3

| File | Change |
| --- | --- |
| [DASHBOARD_PLAN.md](./DASHBOARD_PLAN.md) | Updated the implementation status to reflect Phase 3 infrastructure configuration. |
| [compose.yml](./compose.yml) | Added Jaeger, the OTel Collector, Prometheus, and Grafana services, their dependencies and host ports, plus Prometheus/Grafana persistent volumes. |
| [compose_commands.md](./compose_commands.md) | Documented local UI URLs, default Grafana credentials, retention controls, and telemetry loss/shutdown behavior. |
| [observability/otel-collector-config.yaml](./observability/otel-collector-config.yaml) | Configured OTLP receivers, memory limiting and batching, a Jaeger trace exporter, and a Prometheus metrics exporter. |
| [observability/jaeger-config.yaml](./observability/jaeger-config.yaml) | Configured Jaeger v2's bounded memory storage, query/UI extension, and OTLP ingestion pipeline. |
| [observability/prometheus.yml](./observability/prometheus.yml) | Configured Prometheus to scrape the Collector's metrics endpoint. |
| [observability/grafana/provisioning/datasources/datasources.yaml](./observability/grafana/provisioning/datasources/datasources.yaml) | Provisioned Prometheus and Jaeger as Grafana data sources. |
| [observability/grafana/provisioning/dashboards/dashboards.yaml](./observability/grafana/provisioning/dashboards/dashboards.yaml) | Configured Grafana to load dashboard files from the mounted provisioning directory. |
| [observability/grafana/provisioning/dashboards/trapigo-gateway.json](./observability/grafana/provisioning/dashboards/trapigo-gateway.json) | Added the initial, version-controlled gateway dashboard scaffold and its panels/queries. |
| [phase3_explanation.md](./phase3_explanation.md) | Added this architecture and YAML learning walkthrough. |

## Validation and next learning step

The configuration passed Compose validation, Collector and Jaeger validation
with their pinned images, Prometheus configuration validation, and
`git diff --check`. An isolated monitoring-only stack with fresh volumes also
passed these live checks on 2026-10-17:

- Grafana 13.2.4 started and provisioned both data sources and all six panels.
- Both Grafana data-source health checks returned `OK`.
- Prometheus successfully scraped the Collector.
- A synthetic OTLP test span reached Jaeger and was retrieved through its
  documented v3 query API, then through Grafana's Jaeger data-source proxy.
- Synthetic OTLP metrics from two service instances were exposed as separate
  `infra_smoke_requests_total` series with `service_instance_id` labels, each
  with the expected value of 1.

Those synthetic signals were only infrastructure test inputs, not dashboard
fallback data or application traffic. The isolated test containers and their
volumes were removed afterward. These tests do not verify real business
requests, gateway dashboard query correctness, or upgrades of existing data
volumes. Phase 4 instrumentation and later end-to-end verification are still
required for those application-specific checks. Trace arrival is asynchronous;
an initial not-found response can occur before the Collector and Jaeger finish
batching/exporting a completed span.

When ready to experiment, start the stack with:

```bash
podman compose -f compose.yml up --build
```

or:

```bash
docker compose -f compose.yml up --build
```

Then open Grafana at <http://localhost:3001>, Jaeger at
<http://localhost:16686>, and Prometheus at <http://localhost:9090>. At this
stage, use the Collector scrape target to learn whether Prometheus can scrape
the Collector. Do not expect gateway request panels to have application data
until the services are instrumented.
