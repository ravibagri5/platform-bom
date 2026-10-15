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
| `pbom init DIR [--context C] [--env E] [--release V] [--components]` | Create a platform directory from a live cluster: `platform.yaml`, a first release, optional Component drafts for unclassified Helm releases, and a `kustomization.yaml` for the Helm chart. |
| `pbom kustomize DIR` | Regenerate `DIR/kustomization.yaml` after adding or removing files. |
| `pbom discover [--env E \| --context C] [-o yaml\|json]` | Discover components. `-o yaml` output can be saved and used as an `inventoryFile`, or applied as an `Inventory` resource. |
| `pbom matrix [-o json]` | Component × environment versions, with drift and upstream markers. |
| `pbom drift [--exit-code]` | Components that differ from each environment's target release. Exits 2 on drift with `--exit-code`. |
| `pbom updates [--why] [-o json]` | Upgrade recommendations from upstream releases, with the reasoning. |
| `pbom release list \| show NAME` | Browse platform releases. |
| `pbom release create NAME --from-env ENV [--context C]` | Cut a release from what an environment is running; `--context` discovers through a kube context instead, for example for an `inCluster` environment. Updates a `kustomization.yaml` written by `pbom init`. |
| `pbom release diff FROM TO [--all]` | Compare two releases. |
| `pbom catalog` | Known component definitions. |
| `pbom serve [--addr] [--refresh-interval] [--config-check-interval] [--metrics-addr] [--log-format] [--log-level]` | Web UI and JSON API. Listens on `127.0.0.1:8080` by default and reloads configuration files when they change (checked every 15s). See [monitoring and logging](#monitoring-and-logging). |
| `pbom serve --config-source=crd [--crd-namespace N] [--crd-context C] [--platform-name P]` | Serve the `pbom.dev` custom resources in a namespace instead of files, applying changes as they happen. The Helm chart runs this; with `--crd-context` it also runs from a workstation. |
| `pbom version` | Print the version. |

Global flags: `-c, --config` (default `pbom.yaml`, or `$PBOM_CONFIG`),
`--releases-dir` and `--components-dir` (override the platform file) and
`--no-upstream`. `pbom init` and `pbom serve` also take `--landscape-url`; see
[CNCF landscape](#cncf-landscape).

Set `GITHUB_TOKEN` for complete upstream release data (without it, `pbom`
falls back to public release feeds once GitHub's anonymous API limit runs
out), or pass `--no-upstream` to work offline.

## Configuration

All documents use `apiVersion: pbom.dev/v1alpha1`. The schema is `v1alpha1`
and may change between minor releases. They are plain YAML files read by
`pbom`, not Kubernetes resources, and there are four kinds:

| Kind | Answers | Written by | Needed? |
| --- | --- | --- | --- |
| [`Platform`](#platform) | What is the platform, what does it offer, and which environments should run which release? | You | Yes, one per platform (the Helm chart provides a default) |
| [`Component`](#component) | How is a tool recognised in a cluster, and where are its releases published? | Built-in catalog; you add or override | Only for tools the catalog does not know |
| [`PlatformRelease`](#platformrelease) | Which component versions and offerings make up platform version X? | You, or `pbom release create` | Only to track drift against a baseline |
| [`Inventory`](#inventory) | What was one environment running, and what is the evidence? | `pbom discover -o yaml` | Only for clusters `pbom` cannot reach |

Declared documents (`Platform`, `PlatformRelease`, `Component`) live in Git and
are reviewed like code. An `Inventory` is observed output: editing it does not
classify anything, so to name an unclassified tool write a `Component`.

With the Helm chart, `Platform`, `PlatformRelease`, `Component` and exported
`Inventory` documents are Kubernetes custom resources (CRDs installed by the
chart), applied with `kubectl apply -k` or any CD tool, or rendered from the
chart's `platform`, `components` and `releases` values; see
[getting started](getting-started.md#4-describe-your-platform-in-yaml).
`pbom serve --config-source=crd` watches them and applies changes without a
restart. In a `Platform` resource, use `readme` instead of `readmeFile` and
`inventory` (the name of an `Inventory` resource) instead of `inventoryFile`,
and set the display name with the `pbom.dev/display-name` annotation instead
of `metadata.displayName`. Without the chart, the CLI and `pbom serve` read
the same documents from files.

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

Each environment uses one source:

| Source | Fields |
| --- | --- |
| The cluster `pbom` runs in, with its own service account | `inCluster: true` |
| A kubeconfig context | `kubeContext`, optionally `kubeconfig` |
| A service-account token, such as a mounted token Secret | `server` (`https://`), `tokenFile`, optionally `caFile` |
| An exported inventory | `inventoryFile` (files), or `inventory` naming an `Inventory` resource (Helm chart) |

See [multi-cluster setup](multi-cluster.md) for running one `pbom` for
several clusters.

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

Built-in categories are the platform pillars `runtime`, `infrastructure`
(compute and infrastructure), `delivery`, `networking`, `security`,
`observability`, `data`, `streaming` and `developer-experience`. Any other
value creates your own category; `compute` and `composition` from earlier
releases are read as `infrastructure`.

```yaml
apiVersion: pbom.dev/v1alpha1
kind: Component
metadata:
  name: karpenter
spec:
  displayName: Karpenter
  category: infrastructure
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
example for clusters the machine running `pbom` cannot reach. With the Helm
chart, `kubectl apply` the export and set the environment's `inventory` to its
name, which is the `--env` value:

```shell
pbom discover --env prod -o yaml > inventories/prod.yaml
kubectl apply -n platform-bom -f inventories/prod.yaml   # Helm chart only
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
| A Helm release named after a CNCF project | That project's name and category, from the CNCF landscape |
| Any other Helm release | Under *Other*, unless `hideUnclassified` is set |

Adding a definition for one of these gives it a proper name, category and
upstream update checks.

### CNCF landscape

Helm releases the catalog does not know are matched by chart name against the
230 graduated, incubating and sandbox projects of the
[CNCF landscape](https://landscape.cncf.io), such as `volcano`, `spiderpool` or
`chaos-mesh`, and placed in the matching category. Your own `Component`
definitions and the built-in catalog always take precedence.

A snapshot of the landscape is built into `pbom`, so this works without network
access. `pbom serve` and `pbom init` also try to download the current landscape
at startup from `--landscape-url` (or `PBOM_LANDSCAPE_URL`; default
`https://raw.githubusercontent.com/cncf/landscape/master/landscape.yml`). If the
download fails, or the file no longer has the expected format, they log a
warning and keep using the snapshot. Point the flag at an internal mirror, or
set it to `""` to use only the snapshot. Maintainers refresh the snapshot with
`make landscape`.

### Drafts from discovery

`pbom serve` fills in what a `Platform` leaves empty from what discovery
found, and the UI marks each draft:

| Left empty | Drafted |
| --- | --- |
| `description` | One line on the Kubernetes version, environments and offerings |
| `offerings` | One offering per category of recognised components |
| No `PlatformRelease` at all | Release `1.0.0`: what a production environment, or the first one, runs now |
| An environment's `targetRelease` | The newest release, so drift is shown from the start |

Nothing is ever written back. Setting the field, or applying a release,
replaces the draft. The **Releases** page turns any environment into a
release you can save: pick the environment and version, then download the
YAML or pipe it into `kubectl apply`. It comes from
`GET /api/snapshot?env=E&name=V`. The CLI and `pbom drift` never use drafts,
so CI checks only what you declared. `pbom init` writes the same drafts into
`platform.yaml`, with comments that explain each field.

## Upstream updates

For every component with an `upstream`, `pbom` fetches GitHub releases (cached
on disk for six hours) and compares them with what each environment runs.
Without a token GitHub allows 60 API requests an hour; when that runs out,
`pbom` reads each project's public releases feed instead, which has no API
limit but lists only the 10 newest releases. Set `GITHUB_TOKEN` (chart value
`githubTokenSecret`) for complete data.

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

`pbom serve` embeds a single-page UI with six views:

| View | Shows |
| --- | --- |
| **Platform** | The product page: tagline, current release, offerings, guarantees, where it runs and your README |
| **Components** | Everything the platform is made of, grouped by capability, with providers and functions nested under Crossplane and embedded functions under their configuration |
| **Offerings** | The capabilities you promise, their maturity and the components behind them |
| **Releases** | A timeline of platform releases with notes, bill of materials and diffs, and a snapshot of any environment to save as the next release |
| **Environments** | Component × environment matrix with drift, colour-coded kinds and collapsible groups |
| **Updates** | Upgrade recommendations with upstream release notes and the reasons behind them |

The UI never changes a cluster. **Rediscover** re-reads the clusters on demand;
otherwise the server rediscovers on `--refresh-interval` (default 10 minutes).

The UI follows the operating system's light or dark setting. The selector in
the top bar overrides it with **Light** or **Dark**; the choice is remembered
in the browser.

## Monitoring and logging

### Metrics

`pbom serve --metrics-addr 0.0.0.0:9090` serves Prometheus metrics at
`/metrics` on their own port, outside sign-in. In the Helm chart, set
`metrics.enabled=true`; see [Monitoring](../charts/platform-bom/README.md#monitoring) for the
ServiceMonitor, alert rules and Grafana dashboard. Scrapes read the results of
the last discovery and never start one, so they are cheap and do not call
GitHub.

| Metric | Labels | Meaning |
| --- | --- | --- |
| `pbom_environment_compliant` | `environment`, `target_release`, `running_release` | 1 when the environment runs its declared target release, else 0 |
| `pbom_component_status` | `environment`, `component`, `status` | `aligned`, `drift`, `missing`, `untracked`, `present` or `absent` against the target |
| `pbom_component_freshness` | `environment`, `component`, `freshness` | `latest`, `patch-available`, `minor-behind`, `major-behind`, `unsupported` or `unknown` |
| `pbom_component_info` | `environment`, `component`, `version`, `category` | Every running version |
| `pbom_component_upstream_info` | `component`, `latest` | Newest upstream release |
| `pbom_platform_info` | `platform`, `current_release` | The platform served |
| `pbom_discovery_duration_seconds` | `environment` | Duration of the last discovery |
| `pbom_discovery_last_success_timestamp_seconds` | `environment` | Time of the last successful discovery |
| `pbom_discovery_errors_total` | `environment` | Failed discoveries |
| `pbom_upstream_requests_total` | `source` (`api`, `feed`), `result` (`ok`, `error`, `rate_limited`) | Upstream release lookups |
| `pbom_config_last_reload_successful`, `pbom_config_reloads_total` | `result` | Whether configuration changes were accepted |
| `pbom_http_requests_total`, `pbom_http_request_duration_seconds` | `method`, `code` | UI and API traffic |
| `pbom_build_info` | `version`, `goversion` | The running version |

Go runtime (`go_*`) and process (`process_*`) metrics are included. Values
with a label are 1 ("info" style), so `count` and `sum` by label give the
numbers. Series are bounded by components × environments. The platform
metrics use only what you declared, never drafts, so alerts fire on your own
targets. The metrics show component versions: keep the port inside the cluster.

### Logs

`pbom serve` logs to stderr with `--log-format text` (default) or `json`, and
`--log-level debug|info|warn|error` (`--debug` is `--log-level debug`). The
Helm chart uses `json` (`logFormat`, `logLevel`). Each line has a `msg` and
key-value fields such as `environment` and `error`:

| Message | Level | When |
| --- | --- | --- |
| `serving platform`, `serving metrics` | info | Startup, with the version and addresses |
| `reconciled PBOM resources`, `configuration reloaded` | info | A configuration change was applied |
| `PBOM resources are invalid; keeping the previous configuration` | warn | A change was rejected; the reason is in `error` |
| `discovery failed` | warn | An environment could not be read |
| `cannot refresh the CNCF landscape; using the built-in snapshot` | warn | The landscape download failed |
| `request` | debug, or warn for 5xx | One API request, with status and duration |

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
| `GET /api/snapshot?env=E&name=V[&download=1]` | What an environment runs now, as `PlatformRelease` YAML to apply |
| `GET /healthz`, `GET /readyz` | Liveness; readiness (not ready until a `Platform` is loaded) |

Prometheus metrics are served on a separate port; see
[monitoring and logging](#monitoring-and-logging).
