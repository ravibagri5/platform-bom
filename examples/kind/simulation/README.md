# Three-cluster PBOM discovery simulation

This fixture installs **19 common components in each cluster**, with an extra
OpenTelemetry Operator fixture in dev. It covers delivery, infrastructure,
backup, networking, observability, security, compute and FinOps (OpenCost).
The original `1.0.0` release is installed in prod; dev and QA run the newer
`1.1.0` release. All three target `1.1.0`, so PBOM's matrix shows prod
running the older release with 16 version drifts and dev's extra OpenTelemetry
Operator fixture as untracked. Grafana, Velero and OpenCost use the upstream
versions observed when the example was prepared; they are current in all three
environments and make the upstream-currency comparison visible.

The Helm chart creates zero-replica Deployments with versioned container images.
PBOM reads their pod specifications, so these are **inventory simulation
fixtures, not running installations** of Argo CD, Vault or the other tools.
No images are pulled, services exposed, credentials created or controllers
started. Image tags are illustrative and are not checked against registries.
Use the upstream charts separately if you need operational products.

From the repository root, with Docker, kind and Helm installed:

```shell
make kind-clusters
make kind-simulation
go run ./cmd/pbom -c examples/kind/simulation/pbom.yaml matrix
go run ./cmd/pbom -c examples/kind/simulation/pbom.yaml serve
```

Open http://127.0.0.1:8080 to view the matrix. Upstream versions come from
GitHub releases: do not pass `--no-upstream` if you want the Latest column.
Set `GITHUB_TOKEN` in your shell if GitHub's unauthenticated API rate limit is
exhausted; components with no reachable or parseable releases remain blank.

To reproduce the rollout from scratch, first apply the old release's image
profile to all three clusters, then upgrade dev and QA with
`make kind-simulation` (the target also keeps prod on the old profile):

```shell
for environment in dev qa prod; do
  helm upgrade --install pbom-simulation examples/kind/simulation/chart \
    --kube-context "kind-pbom-$environment" -n pbom-simulation \
    --create-namespace -f examples/kind/simulation/prod.yaml --wait
done
make kind-simulation
```

The simulation uses its own platform file and catalog definition for OpenCost;
it does not change `deploy/platform/pbom.yaml` or its exported inventories.
Re-running `make kind-simulation` upgrades the same Helm releases. To change
the profile, edit `dev.yaml`, `qa.yaml` or `prod.yaml`, then re-run the target. To remove
only the fixtures (and preserve the kind clusters):

```shell
for environment in dev qa prod; do
  helm uninstall pbom-simulation --kube-context "kind-pbom-$environment" -n pbom-simulation
done
```