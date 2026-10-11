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

## Deployment Manifests

Manifests now live under [k8s](./k8s/README.md). The shared base contains
Secret references, while the local-dev overlay supplies fake credentials.
The root Kustomization selects local-dev. The production overlay supplies no
credentials and remains a scaffold pending environment-specific configuration.

The folder reorganization does not yet resolve Kind image loading, storage,
gateway configuration mounts, or external port access. See the deployment notes
before attempting to launch the stack.
