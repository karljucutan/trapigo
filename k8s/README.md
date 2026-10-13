# Deployment Layout

```text
kustomization.yaml
trapigo/
  configs/
    trapigo-gateway-kube.yaml
k8s/
  base/
    kustomization.yaml
    kube.yaml
  overlays/
    local-dev/
      kustomization.yaml
      gateway-config.yaml
      local-secrets.yaml
    production/
      kustomization.yaml
```

The base generates a ConfigMap from
`trapigo/configs/trapigo-gateway-kube.yaml` and mounts it at `/app/configs`.
Kustomize uses the source file's basename as the ConfigMap key, so the mounted
file is `/app/configs/trapigo-gateway-kube.yaml`. The Kubernetes Deployment sets
`TRAPIGO_GATEWAY_CONFIG` to that path. Compose continues to use its own
`trapigo/configs/trapigo-gateway.yaml` without sharing or changing this
Kubernetes config. The gateway loader expands
`${KEYCLOAK_CLIENT_SECRET}` from the container environment, so the ConfigMap
contains no client credential. The base references `trapigo-monorepo-secrets`
but does not create or embed credentials. The local-dev overlay supplies
explicit development-only Secret values, selects the dev gateway image, and
adds DEBUG logging and Delve configuration. Configure the local Keycloak client
with the same development client secret.

The base also generates environment ConfigMaps for the gateway, Keycloak, and
the Go Order Service and imports each into its matching container using
`envFrom.configMapRef`. `POD_NAMESPACE` remains a Downward API environment
entry because Kubernetes supplies its value per Pod. Database credentials,
`ORDER_DB_DSN`, and `KEYCLOAK_CLIENT_SECRET` remain explicit `secretKeyRef`
entries; secrets are not placed in ConfigMaps.

Because the ConfigMap source file is outside `k8s/base`, Kustomize's default
load restriction rejects it. Build with `--load-restrictor=LoadRestrictionsNone`
from this trusted repository:

```sh
kubectl kustomize k8s/overlays/local-dev --load-restrictor=LoadRestrictionsNone
```

This only renders local manifests; it does not contact a cluster. Local-dev
targets Podman `kube play`, not Kind. It converts the Deployments to named Pods,
removes Services, ServiceAccount, and RBAC, and uses static Pod DNS backends
instead of Kubernetes discovery. It runs one order-service Pod, publishes the
gateway on `localhost:8081`, and retains the Podman debugger seccomp annotation.
The root Kustomization selects this overlay. See [local run commands](../podman_kube_commands.md)
for image builds, DNS networking, startup, migrations, and teardown.

Production remains a Kubernetes overlay. Apply it only to an intended cluster
after satisfying the production requirements below:

```sh
kubectl kustomize k8s/overlays/production --load-restrictor=LoadRestrictionsNone | kubectl apply -f -
```

`LoadRestrictionsNone` permits local Kustomizations to read files outside their
root and makes these builds non-relocatable. Use it only with trusted repository
contents. To inspect the generated manifests without applying them, omit the
pipe to `kubectl apply -f -`.

The production Go Order Service Deployment has three replicas. The Kubernetes gateway
config's `kubernetes.service` and `kubernetes.port` fields name the Service and
its `http` port. At startup Trapigo lists that Service's EndpointSlices, waits
up to one minute for ready Pod endpoints, and adds their Pod IPs to its own
round-robin pool. The request path therefore goes directly from Trapigo to a
discovered Go Order Service Pod, not through the Service ClusterIP. The Service
remains the source used by Kubernetes to select the workload and publish its
endpoints.

The gateway uses a dedicated ServiceAccount and a namespace-scoped Role that
can only list EndpointSlices. Discovery happens once during startup; if Pods
are replaced or their IPs change later, restart Trapigo to refresh its backend
list. If discovery fails or no endpoints become ready within one minute,
gateway startup fails with an explicit error.


`local-secrets.yaml` is safe to commit only while its values remain fake. Do not
replace them with real credentials. Local database initialization uses
`go-order-service`; changing POSTGRES_DB does not update an existing volume.
Order schema migrations and Keycloak realm/client setup remain separate steps.

## Production Secret Contract

The production overlay includes no Secret manifests. Your externally configured
vault integration must create the Kubernetes Secret `trapigo-monorepo-secrets`
in the workloads' namespace, with these keys:

- `SERVICES_DB_USER`
- `SERVICES_DB_PASSWORD`
- `SERVICES_DB_NAME`
- `KC_DB_USER`
- `KC_DB_PASSWORD`
- `KC_DB_NAME`
- `KC_BOOTSTRAP_ADMIN_USERNAME`
- `KC_BOOTSTRAP_ADMIN_PASSWORD`
- `KEYCLOAK_CLIENT_SECRET`
- `ORDER_DB_DSN`

No cloud vault provider or operator has been selected or configured here. This
overlay is a scaffold, not a production-ready deployment. Registry image tags,
Keycloak production settings, TLS, storage, networking, and gateway configuration
must be provided before production use.

Compose's existing gateway config still contains a client secret; this
Kubernetes config split does not make that separate file safe for real secrets.