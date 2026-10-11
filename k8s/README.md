# Deployment Layout

```text
k8s/
  base/
    kustomization.yaml
    kube.yaml
  overlays/
    local-dev/
      kustomization.yaml
      local-secrets.yaml
    production/
      kustomization.yaml
```

The base references `trapigo-monorepo-secrets` but does not create it or embed
credentials. Non-sensitive settings remain in the base. The root Kustomization
selects local-dev, which adds deliberately fake credentials and preserves the
dev gateway image, DEBUG logging, and Delve configuration.

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
- `ORDER_DB_DSN`

No cloud vault provider or operator has been selected or configured here. This
overlay is a scaffold, not a production-ready deployment. Registry image tags,
Keycloak production settings, TLS, storage, networking, and gateway configuration
must be provided before production use.

## Remaining Local Kind Work

- Replace the relative gateway hostPath with a ConfigMap or a configured node mount.
- Provide Kubernetes gateway routing instead of Compose replica names, without
  duplicating YAML mapping keys in the same configuration.
- Load the built images into Kind and provide suitable persistent storage.
- Configure external access using port forwarding or Kind port mappings.
- Adapt the retained Podman-specific debugger seccomp annotation for Kubernetes
  if needed; it is not a Kubernetes seccomp securityContext.

The existing gateway configuration is unchanged and still contains a client
secret; this reorganization does not make that separate file safe for real secrets.