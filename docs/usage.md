# Using Platform BOM

This is the reference for the `pbom` CLI, the configuration files, discovery,
upstream checks and the HTTP API. For the concepts behind them, start with the
[README](../README.md).

- [Describing your platform](#describing-your-platform)
- [CLI](#cli)
- [Configuration](#configuration)
- [How discovery works](#how-discovery-works)
- [Upstream updates](#upstream-updates)
- [Releases, drift and CI](#releases-drift-and-ci)
- [The web UI](#the-web-ui)
- [HTTP API](#http-api)

## Describing your platform

1. **Check each cluster is reachable.**

   ```shell
   pbom discover --context prod-cluster
   ```

2. **Describe your platform** in a `pbom.yaml`, one environment per cluster:

   ```yaml
   apiVersion: pbom.dev/v1alpha1
   kind: Platform
   metadata:
     name: acme
     displayName: Acme Platform
   spec:
     tagline: The paved road for shipping services at Acme.
     environments:
       - name: dev
         kubeContext: dev-cluster
       - name: prod
         kubeContext: prod-cluster
   ```

3. **Cut your first release** from what production runs today:

   ```shell
   pbom release create 1.0.0 --from-env prod --summary "Baseline"
   ```

4. **Set `targetRelease: "1.0.0"`** on each environment, then look at
   `pbom drift` or open the UI with `pbom serve`.

5. **Commit `pbom.yaml` and `releases/` to Git.** Releases are reviewed like any
   other change, and `git log releases/` is your platform changelog.

## CLI

| Command | What it does |
| --- | --- |
| `pbom discover [--env E \| --context C] [-o yaml\|json]` | Discover components. `-o yaml` output can be saved and used as an `inventoryFile`. |
| `pbom matrix [-o json]` | Component × environment versions, with drift and upstream markers. |
| `pbom drift [--exit-code]` | Components that differ from each environment's target release. Exits 2 on drift with `--exit-code`. |
| `pbom updates [--why] [-o json]` | Upgrade recommendations from upstream releases, with the reasoning. |
| `pbom release list \| show NAME` | Browse platform releases. |
| `pbom release create NAME --from-env ENV` | Cut a release from what an environment is running. |
| `pbom release diff FROM TO [--all]` | Compare two releases. |
| `pbom catalog` | Known component definitions. |
| `pbom serve [--addr] [--refresh-interval]` | Web UI and JSON API. Listens on `127.0.0.1:8080` by default. |
| `pbom version` | Print the version. |

Global flags: `-c, --config` (default `pbom.yaml`, or `$PBOM_CONFIG`) and
`--no-upstream`.

Set `GITHUB_TOKEN` to avoid GitHub's anonymous rate limit when fetching
upstream releases, or pass `--no-upstream` to work offline.

## Configuration

All documents use `apiVersion: pbom.dev/v1alpha1`. The schema is `v1alpha1`
and may change between minor releases.

### Platform

```yaml
apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: acme-platform
  displayName: Acme Internal Platform
spec:
  tagline: The paved road for shipping services at Acme.
  description: A Kubernetes platform with GitOps delivery and self-service infrastructure.
  owners:
    - name: Platform Engineering
      channel: "#platform-help"
  links:
    - title: Getting started
      url: https://docs.acme.example/platform
  guarantees:
    - Every change delivered through GitOps
  readmeFile: PLATFORM.md             # rendered on the Platform page
  offerings:
    - name: self-service-infra
      displayName: Self-service infrastructure
      category: data
      status: ga                      # planned, alpha, beta, ga or deprecated
      description: Databases and storage through Crossplane APIs.
      components: [crossplane, provider-upjet-azure]
  environments:
    - name: prod
      displayName: Production
      tier: production
      kubeContext: prod-aks           # or inventoryFile: inventories/prod.yaml
      kubeconfig: /path/to/kubeconfig # optional
      inCluster: false                # true: discover the cluster pbom runs in
      targetRelease: "1.1.0"
  releasesDir: releases               # default
  componentsDir: components           # optional extra or overriding Component definitions
  discovery:
    hideUnclassified: false           # skip Helm releases that match no catalog component
    ignoreNamespaces: [team-a]
```

### PlatformRelease

```yaml
apiVersion: pbom.dev/v1alpha1
kind: PlatformRelease
metadata:
  name: "1.2.0"
spec:
  version: "1.2.0"
  date: "2026-09-15"
  summary: Azure infrastructure and Kubernetes 1.34.
  highlights:
    - Kubernetes upgraded to 1.34
  notes: |
    Markdown release notes.
  components:
    kubernetes: "1.34"     # matches any 1.34.x
    crossplane: "2.0.2"    # matches exactly 2.0.2
    postgres: "2.0.1-rc.3" # prereleases must match exactly
  offerings: [app-delivery, self-service-infra]
```

Versions match on the parts you write, so `"1.34"` accepts any patch release
while `"1.34.2"` pins one.

### Component

Component definitions teach discovery how to recognise a building block and
where its releases are published. The builtin catalog lives in
[internal/catalog/components](../internal/catalog/components); add or override
definitions with `componentsDir`.

```yaml
apiVersion: pbom.dev/v1alpha1
kind: Component
metadata:
  name: karpenter
spec:
  displayName: Karpenter
  category: compute
  description: Just-in-time node provisioning.
  homepage: https://karpenter.sh
  partOf: ""                        # nest under another component, e.g. crossplane
  discovery:
    images: ["karpenter/controller"] # matches any registry or mirror prefix
    helmCharts: ["karpenter"]
    apiGroups: ["karpenter.sh"]
    crossplanePackages: []
  upstream:
    github: aws/karpenter-provider-aws
    tagPrefix: ""                   # e.g. "controller-v" for ingress-nginx
    supportedMinors: 0              # e.g. 3 for Kubernetes
```

### Inventory

`pbom discover -o yaml` writes an `Inventory`: what one environment was found
to be running, with the evidence for each component. Point an environment's
`inventoryFile` at a saved inventory to use it instead of a live cluster, for
example for clusters the machine running `pbom` cannot reach:

```shell
pbom discover --env prod -o yaml > inventories/prod.yaml
```

## How discovery works

For each environment, `pbom` reads:

| Source | What it looks at |
| --- | --- |
| Cluster | Server version and nodes, to identify EKS, AKS, GKE, kind, k3s and RKE2 |
| Workloads | Container images in Deployments, StatefulSets and DaemonSets |
| Helm | Release metadata in Secrets labelled `owner=helm,status=deployed` |
| Crossplane | Providers, Functions and Configurations in `pkg.crossplane.io` |
| API groups | Served groups, including those of CRDs, to spot components whose version cannot be read |

Evidence is matched against the catalog. Image patterns ignore registry and
mirror prefixes, so `argoproj/argocd` matches
`registry.corp.example/mirror/argoproj/argocd`. When several sources report a
version, the most trustworthy wins: cluster version, then Crossplane package
tag, then container image tag, then Helm `appVersion`. Prerelease suffixes such
as `-rc.3` are kept.

Nothing that is discovered is thrown away. Components with no catalog
definition are still reported and placed in the tree:

| Found | Shown as |
| --- | --- |
| A Crossplane Configuration | A **Platform API** |
| A Function named `<configuration>_<function>` | Nested under that configuration |
| Any other Provider or Function | Nested under Crossplane |
| A Helm release | Under *Other*, unless `hideUnclassified` is set |

Adding a definition for one of these gives it a proper name, category and
upstream update checks.

## Upstream updates

For every component with an `upstream`, `pbom` fetches GitHub releases (cached
on disk for six hours) and compares them with what each environment runs:

```text
$ pbom updates --why

Kubernetes 1.33.5
  - Upstream supports the latest 3 minor releases; 1.33.5 is 4 minors behind 1.37.0 and out of upstream support.
  - 1.37.0 was released on 26 Aug 2026.
  - Low-risk step: patch 1.33.13 is available within your current line.
  - Environments run different versions: dev 1.34.1, prod 1.33.5, staging 1.34.1.
```

Recommendations are rule-based and explained, never a bare "upgrade". The UI
shows the release notes for every version between yours and the latest.

## Releases, drift and CI

A typical flow:

1. Upgrade components in dev and staging as you normally would.
2. `pbom release create 1.3.0 --from-env staging`, edit the notes, open a pull
   request.
3. Point dev and staging at `1.3.0`. Promote production by changing its
   `targetRelease` in a one-line pull request.
4. Run drift checks on a schedule:

```yaml
# .github/workflows/platform-drift.yaml
on:
  schedule: [{ cron: "0 6 * * *" }]
jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      # Authenticate to your clusters here.
      - run: pbom drift --exit-code
```

## The web UI

`pbom serve` embeds a single-page UI with five views:

| View | Shows |
| --- | --- |
| **Platform** | The product page: tagline, current release, offerings, guarantees, where it runs and your README |
| **Components** | Everything the platform is made of, grouped by capability, with providers and functions nested under Crossplane and embedded functions under their configuration. Links to **Updates**, the upgrade recommendations with upstream release notes |
| **Offerings** | The capabilities you promise, their maturity and the components behind them |
| **Releases** | A timeline of platform releases with notes, bill of materials and diffs |
| **Environments** | Component × environment matrix with drift, colour-coded kinds and collapsible groups |

The UI never changes a cluster. **Rediscover** re-reads the clusters on demand;
otherwise the server rediscovers on `--refresh-interval` (default 10 minutes).

## HTTP API

The API is read-only JSON, and the web UI is built entirely on it. It is not
yet versioned; expect changes before v1.

| Endpoint | Returns |
| --- | --- |
| `GET /api/platform` | Platform definition, README and current release |
| `GET /api/matrix[?refresh=true]` | Environment matrix and summary |
| `GET /api/inventory` | Raw inventories with evidence |
| `GET /api/updates` | Upgrade recommendations |
| `GET /api/releases`, `/api/releases/{name}` | Platform releases |
| `GET /api/diff?from=A&to=B` | Release diff |
| `GET /api/catalog` | Component definitions |
| `GET /healthz` | Liveness |
