# Local Podman Kube Commands

Run from the repository root. Local development uses Podman, not Kind or a
Kubernetes cluster. Once images and the rendered manifest are prepared, these
are the two lifecycle commands.

## Command Flow

Use the matching scenario below, then run the commands in the named sections.
If the Pods are already stopped, skip Stop.

| Scenario | Flow |
| --- | --- |
| First run | Build -> Render -> Start |
| Normal use, nothing changed | Start -> Stop |
| Application code or Dockerfile changed | Stop -> Build -> Start |
| Manifests, gateway configuration, ConfigMaps, or local Secrets changed | Stop -> Render -> Start |
| Both application code and configuration changed | Stop -> Build -> Render -> Start |
| Pods are stopped and the temporary YAML was deleted | Render -> Start |
| An application image was deleted | Stop -> Build -> Start |

Build and Render refer to the two command blocks in Prepare or Update below.
Build only the affected images, or both on the first run.

Stop before overwriting the rendered YAML: teardown should use the manifest
that started the current Pods. If that file is missing while Pods are running,
recreate it from the original configuration before Stop.

Database migrations are needed initially and when new migrations are added.
Keycloak realm/client setup is needed initially or after deleting its database
data. Stop preserves database volumes, so routine restarts do not repeat this
setup. See Application Setup below.

## Start

```bash
podman kube play --build=false "/tmp/trapigo-local-${UID}.yaml"
```

- `kube play` creates and starts the Pods described in the rendered YAML.
- `--build=false` uses the explicitly built application images, preserving the
	gateway's `dev` stage and Delve. Missing PostgreSQL and Keycloak images are
	pulled according to their image pull policies.
- With no `--network` flag, Podman creates/uses its shared DNS-enabled
	`podman-default-kube-network`. No separate network command is needed.
- Gateway: <http://localhost:8081>; admin health: <http://localhost:8080/health>.
	Attach Delve at `localhost:40000` and continue execution to start the gateway.

## Stop

```bash
podman kube down "/tmp/trapigo-local-${UID}.yaml"
```

- `kube down` stops and removes the Pods identified by the same manifest.
- Database volumes are preserved. Do not add `--force` unless you intend to
	delete their data.
- Stop before starting again or replacing the rendered manifest.

## Prepare or Update

Build once initially; repeat these builds when application code changes:

```bash
podman build --target dev -t localhost/trapigo:dev -f trapigo/Dockerfile .
podman build -t localhost/go-order-service:latest -f go-order-service/Dockerfile .
```

Render once initially; repeat after changing manifests, ConfigMaps, or local
Secrets, after stopping the old Pods:

```bash
umask 077
kubectl kustomize k8s/overlays/local-dev --load-restrictor=LoadRestrictionsNone > "/tmp/trapigo-local-${UID}.yaml"
```

`kubectl kustomize` only renders files locally; it does not contact a cluster.
Standalone `kustomize build` can be used instead. The load-restrictor flag allows
the base to read its config outside its directory; use it only with this trusted
repository. The output contains development Secrets; `umask 077` restricts newly
created file permissions. Do not share or commit it. Render again if `/tmp` is
cleared.

## Why Not Just --build?

Podman supports automatic Dockerfile builds, but `kube play --build` does not
select a build target. Trapigo's final Dockerfile stage is `prod`, not `dev`;
the image's `:dev` tag does not select that stage. Both Dockerfiles also require
the repository root as their build context to copy `buildingblocks`.
An automatic build must satisfy that context requirement as well as stage
selection. The earlier suggestion that adding `--context-dir .` alone made
this a drop-in debugger build was incomplete. The explicit builds above are
the supported workflow for the current files.

## Application Setup

Ports 5432, 5433, 8081, 8080, and 40000 must be available. Local-dev runs one
order-service Pod with static backend DNS, not production EndpointSlice
discovery. Podman does not provide Compose-style dependency ordering; inspect
logs if services start before their databases are ready.

Order migrations and Keycloak realm/client setup are still separate initial
setup, just as with Compose. See [migration commands](go-order-service/db/migrations/README.md).
Use order database `go-order-service`, realm `CompanyRealmA`, client
`trapigo-apigateway`, and callback `http://localhost:8081/web/auth/callback`.
Use the local client secret from [local-secrets.yaml](k8s/overlays/local-dev/local-secrets.yaml).
Existing database volumes retain their original initialization settings.
The dashboard must be started separately for browser login.