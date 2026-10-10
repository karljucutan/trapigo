# Docker Compose Commands

## Start (build + run)

```bash
docker compose -f compose.yml up --build
```

## Stop and remove containers/network

```bash
docker compose down
```

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

## Local Podman kube configuration

For local `podman kube play`, maintain [one shared manifest](./kube/base/podman-kube.yaml)
and [a small development overlay](./kube/dev/kustomization.yaml), rather than
two full copies. [The root Kustomization](./kustomization.yaml) selects the
shared manifest. Kustomize is included in `kubectl`; rendering does not require
a Kubernetes cluster.

Render either configuration from the repository root:

```bash
# Base: compiled gateway image, INFO logging, no debugger port.
kubectl kustomize .

# Development: dev gateway image, DEBUG logging, Delve port 40000.
kubectl kustomize kube/dev
```

The dev overlay changes only the gateway image, logging, debugger port, and
container seccomp annotation. The annotation is the format consumed by Podman's
kube player; it intentionally replaces the unsupported `seccompProfile` field
from the previous dev manifest. It disables seccomp only for the gateway
container in development.

Build images explicitly; the kube YAML does not select Dockerfile build targets:

```bash
podman build --target prod -t localhost/trapigo:prod -f trapigo/Dockerfile .
podman build --target dev -t localhost/trapigo:dev -f trapigo/Dockerfile .
podman build -t localhost/go-order-service:latest -f go-order-service/Dockerfile .
```

The configuration bind mount uses `./trapigo/configs`, resolved from Podman's
working directory. Run local Podman commands from the repository root. Remote
Podman requires a path on the remote host instead. Dev and base configurations
share workload names, host ports, and database volumes; they are alternatives,
not environments to run simultaneously. The base selects the compiled gateway,
but is not a production deployment: Keycloak still uses `start-dev`.

### Remaining Podman runtime limitations

The overlays eliminate manifest duplication; they do not make Podman a
Kubernetes cluster. Before treating this stack as equivalent to Compose:

- Podman 5.8.x skips `Service` resources and limits each Deployment to one pod,
  regardless of `replicas: 3`. Three order-service instances require explicitly
  defined pods.
- Configure a shared DNS-enabled Podman network and reachable backend names.
  Deployment pod names receive a `-pod` suffix; the existing gateway configuration
  uses Compose replica names instead. Kubernetes Services do not resolve this
  mismatch under Podman.
- Readiness probes do not reproduce Compose's `depends_on: service_healthy`.
  The order service currently connects without a database startup retry.
- Host port 80 requires rootful Podman or host configuration permitting rootless
  binding to privileged ports. Build and run images in the same Podman user
  context.

These runtime issues need separate changes before an end-to-end
`podman kube play` launch can be considered validated. Do not use
`kubectl apply` for this local Podman configuration.

For another environment, add an overlay that references `../base`
and patches only the differences. Podman does not substitute `${ENV_VAR}` in
these YAML files; render the chosen overlay before passing it to Podman, and
keep generated full manifests out of source control.
