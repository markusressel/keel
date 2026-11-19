# Keel static Kubernetes manifests

Ready-to-apply manifests for running Keel without Helm. The Helm chart in
[`chart/keel`](../../../chart/keel) is the recommended install method: it is the
one that receives release testing, and it handles upgrades and configuration
through `values.yaml`. Use these manifests when you cannot use Helm, or want to
see exactly what Keel runs.

If you arrived from an older guide that installed Keel through a hosted install
helper, note that those helpers are no longer reachable — install with the Helm
chart or with the manifests here.

## Files

Applied in filename order; the numeric prefixes keep dependencies first
(namespace, then RBAC, then the workloads that consume them).

| File | Purpose |
| --- | --- |
| `00-namespace.yaml` | `keel` namespace |
| `10-service-account.yaml` | Service Account used by the Deployment and ClusterRoleBinding |
| `11-clusterrole.yaml` | Permissions Keel needs to watch and update workloads |
| `12-clusterrolebinding.yaml` | Binds the ClusterRole to the Service Account |
| `20-secret.yaml` | Basic Auth password (placeholder — edit before applying) |
| `30-deployment.yaml` | Keel Deployment, probes on `/healthz:9300` |
| `40-service.yaml` | ClusterIP Service for the Keel API/UI and registry webhooks |

## Install

1. Set your Basic Auth password in `20-secret.yaml`, or create the secret
   yourself instead of applying that file:

   ```bash
   kubectl -n keel create secret generic keel \
     --from-literal=BASIC_AUTH_PASSWORD='your-password'
   ```

2. Apply the directory. Ordering is encoded in the filenames, so a single apply
   is safe on an empty cluster:

   ```bash
   kubectl apply -f docs/manifests/keel/
   ```

3. Verify Keel is up:

   ```bash
   kubectl -n keel get pods -l app=keel
   kubectl -n keel port-forward deploy/keel 9300:9300
   curl -s http://127.0.0.1:9300/healthz
   ```

   The Admin UI is at `http://127.0.0.1:9300` and authenticates with the Basic
   Auth user from `30-deployment.yaml` (`admin`) plus the password from the
   secret.

To remove it: `kubectl delete -f docs/manifests/keel/`.

## Placeholders and notes

- **Image** — `30-deployment.yaml` pins `ghcr.io/keel-hq/keel:0.22.3`, matching
  the `appVersion` in `chart/keel/Chart.yaml` at the time of writing. Check that
  file for the newest release and bump the pin manually; unlike the chart, these
  manifests cannot self-update. `keelhq/keel` on Docker Hub carries the same
  releases, and a `nightly` tag exists on both registries, but nightly builds are
  pre-releases and are not supported in production.
- **Basic Auth** — Keel requires `BASIC_AUTH_USER` and `BASIC_AUTH_PASSWORD` to
  be set together or both unset, and exits otherwise. `BASIC_AUTH_USER` is an
  env var in `30-deployment.yaml`; the password comes from the secret through
  `envFrom`. Turning Basic Auth off means removing both.
- **Helm provider** — `HELM3_PROVIDER` is `"true"` to match the chart default.
  Set it to `"false"` if the cluster has no Helm releases; image polling and the
  Kubernetes provider are unaffected.
- **RBAC** — `11-clusterrole.yaml` follows the chart's default ClusterRole, with
  two deliberate differences: `pods/portforward` is omitted (this Keel version
  never calls it), and the `secrets`/`configmaps` rules are marked as
  Helm-provider-only in comments, since Keel reads those only through Helm's
  release-storage driver. Set `HELM3_PROVIDER="false"` and drop those two rules if
  the cluster has no Helm releases. Keel is cluster-scoped by design — it watches
  workloads across namespaces — hence a ClusterRole rather than a namespaced Role.
- **Service** — `40-service.yaml` is a ClusterIP, so Keel is reachable only
  inside the cluster. Registry webhooks need an externally reachable address;
  change the Service type or put an Ingress in front of it.

## Prerequisites

- `kubectl` configured for a Kubernetes cluster.
- Helm is not required for this install method.

See the main [README](../../../readme.md) for the recommended Helm install
instructions and the configuration reference.
