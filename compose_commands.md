# Docker Compose Commands

## Start (build + run)

```bash
docker compose -f compose.yml up --build
```

The local observability stack is included in the same Compose project:

- Grafana: <http://localhost:3001> (`admin` / `admin` by default)
- Jaeger: <http://localhost:16686>
- Prometheus: <http://localhost:9090>

Grafana provisions Prometheus and Jaeger data sources and the Trapigo Gateway
dashboard. Gateway request panels become populated after the Phase 4
instrumentation is enabled. Collector OTLP ingestion and Prometheus scrape
ports remain internal to the Compose network. Jaeger stores up to 10,000 traces
in memory by default; its local traces are lost when the container is recreated.
`JAEGER_MEMORY_MAX_TRACES` overrides the local trace-count limit. Jaeger v2's
storage and OTLP receiver are configured in
[observability/jaeger-config.yaml](./observability/jaeger-config.yaml).
Production storage requires a deliberate change to that configuration; the old
Jaeger v1 `SPAN_STORAGE_TYPE` environment variable is not used.
Prometheus retains data for 7 days or 2 GB by default, configurable with
`PROMETHEUS_RETENTION_TIME` and `PROMETHEUS_RETENTION_SIZE`. Set
`GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD` to override local defaults.
The Collector accepts OTLP only on the Compose network. Its trace exporter uses
a bounded in-memory queue of 1,000 batches and retries for at most 5 minutes;
queue overflow, Collector termination, or an unavailable Jaeger can lose
telemetry, but telemetry export is asynchronous and must not block business
requests. Stop the stack gracefully to allow SDK flush and Collector shutdown;
the Collector has a 30-second stop grace period, but queued data is not
persistent and cannot be guaranteed if Jaeger is unavailable.

Monitoring images are pinned to versions checked on 2026-10-17: Jaeger 2.20.0,
OTel Collector Contrib 0.163.1, Prometheus 3.15.0, and Grafana 13.2.4. The latter
three are the latest non-prerelease upstream releases at that check.
Jaeger 2.20.0 is the newest compatible release: 2.21.0 and 2.22.0 no longer
provide `/api/services`, which Grafana's bundled Jaeger plugin 13.0.5 requires.
Upgrade Jaeger after Grafana's plugin supports the newer API. Exact tags make
upgrades explicit rather than allowing a moving `latest` tag to introduce
unreviewed breaking changes.
Back up existing monitoring volumes before major-version upgrades; the
validation of this configuration uses fresh storage, not a migration of
existing Prometheus or Grafana data.

The pinned combination passed an isolated monitoring-only smoke test: both
Grafana data sources were healthy, the dashboard provisioned, Prometheus scraped
the Collector, synthetic OTLP traces were retrievable through Jaeger's v3 API
and Grafana, and two metric producers retained separate instance series. This
does not yet verify the application-specific dashboard queries or Go telemetry.

## Stop and remove containers/network

```bash
docker compose down
```

Use `docker compose down -v` only when intentionally removing local Prometheus
and Grafana data volumes.

## Run with Delve debugger

This project already has a debugger override in `compose.override.yml` (it sets `target: dev` and exposes Delve on port `40000`).

Run Compose with both files:

```bash
docker compose -f compose.yml -f compose.override.yml up --build
```
or

```bash
docker compose up --build
```

Then attach your debugger to:

- Host: `localhost`
- Port: `40000`

## Podman Compose Commands

When running with Podman, use the same commands with `podman` in place of `docker`.

### Start (build + run)

```bash
podman compose -f compose.yml up --build
```

### Stop and remove containers/network

```bash
podman compose down
```

### Run with Delve debugger

```bash
podman compose -f compose.yml -f compose.override.yml up --build
```

or

```bash
podman compose up --build
```

Then attach your debugger to `localhost:40000`.

## Deployment Manifests

Manifests now live under [k8s](./k8s/README.md). The shared base contains
Secret references, while the local-dev overlay supplies fake credentials.
The root Kustomization selects local-dev. The production overlay supplies no
credentials and remains a scaffold pending environment-specific configuration.

Local manifests target Podman `kube play`, not Kind. See
[local kube commands](./podman_kube_commands.md) for image builds, rendering, startup,
and teardown. ConfigMaps and persistent volumes are handled by Podman; order
migrations and Keycloak realm/client configuration remain separate steps.
