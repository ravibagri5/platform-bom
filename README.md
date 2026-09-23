# Platform BOM

[![CI](https://github.com/ravibagri5/platform-bom/actions/workflows/ci.yaml/badge.svg)](https://github.com/ravibagri5/platform-bom/actions/workflows/ci.yaml)
[![CodeQL](https://github.com/ravibagri5/platform-bom/actions/workflows/codeql.yaml/badge.svg)](https://github.com/ravibagri5/platform-bom/actions/workflows/codeql.yaml)
[![GitHub release](https://img.shields.io/github/v/release/ravibagri5/platform-bom?sort=semver)](https://github.com/ravibagri5/platform-bom/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/ravibagri5/platform-bom)](go.mod)
[![golangci-lint](https://img.shields.io/badge/lint-golangci--lint-00ADD8?logo=go&logoColor=white)](https://golangci-lint.run/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**Your internal platform, as a versioned product.**

Application teams have SBOMs, release trains and changelogs. Platform teams
mostly have a wiki page and a spreadsheet of versions that was accurate once.
Platform BOM (`pbom`) treats the platform itself as the product: it discovers
what every environment is actually running, groups it into the capabilities
you offer, lets you publish numbered platform releases, and tells you how far
each building block is behind upstream, and why that matters.

```text
$ pbom matrix

ENVIRONMENT  KUBERNETES  TARGET  RUNNING  STATUS
dev          1.34.1 eks  1.2.0   1.2.0    compliant
staging      1.34.1 eks  1.2.0   -        1 drift, 1 missing
prod         1.33.5 eks  1.1.0   1.1.0    compliant

CATEGORY        COMPONENT                  DEV       STAGING   PROD       RELEASE  LATEST
runtime         Kubernetes                 1.34.1 ✓  1.34.1 ✓  1.33.5 ✓ ↑ 1.34     1.37.0
delivery        Argo CD                    3.1.5 ✓   3.1.5 ✓   3.0.6 ✓ ↑  3.1.5    3.5.3
infrastructure  Crossplane                 2.0.2 ✓   2.0.2 ✓   2.0.0 ✓ ↑  2.0.2    2.4.2
infrastructure  Crossplane Provider Azure  2.0.0 ✓   - ✗       -          2.0.0    2.7.0
security        Kyverno                    1.15.1 ✓  1.15.0 ≠  1.14.2 ✓ ↑ 1.15.1   1.19.1
```

Everything `pbom` does against a cluster is **read-only**: it lists workloads,
Helm release metadata and Crossplane packages, and never writes. Your platform
definition and releases are plain YAML that you keep in Git.

## Contents

- [Why Platform BOM?](#why-platform-bom)
- [The model](#the-model)
- [Quick start](#quick-start)
- [Installation](#installation)
- [Using it with your clusters](#using-it-with-your-clusters)
- [The web UI](#the-web-ui)
- [CLI](#cli)
- [Configuration](#configuration)
- [How discovery works](#how-discovery-works)
- [Upstream updates](#upstream-updates)
- [Releases, drift and CI](#releases-drift-and-ci)
- [Required RBAC](#required-rbac)
- [Running in a container](#running-in-a-container)
- [Running in a cluster](#running-in-a-cluster)
- [HTTP API](#http-api)
- [Development](#development)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Why Platform BOM?

There is no shortage of tools that show you Kubernetes objects, and plenty that
track application versions across environments. What is missing is the view
from the platform team's side of the table:

| Question | Typical answer today | With `pbom` |
| --- | --- | --- |
| *What does our platform offer, and what is it built on?* | A wiki page, if someone kept it current | A product page generated from `pbom.yaml` and live discovery |
| *Which version of the platform is production on?* | "Mostly the same as staging" | Each environment reports the newest release it fully satisfies |
| *What changed between platform 1.1 and 1.2?* | Git archaeology across a dozen repos | `pbom release diff 1.1.0 1.2.0` |
| *Is staging what we said it would be?* | Nobody knows until something breaks | Drift against the environment's target release, component by component |
| *How far behind upstream are we, and does it matter?* | A quarterly spreadsheet | Explainable recommendations: minors behind, support windows, the release notes in between |

The primary object is the **Platform**, not the cluster. Clusters, Helm,
container images and Crossplane packages are only *evidence* that a component
is present.

## The model

Four concepts, all plain YAML under `apiVersion: pbom.dev/v1alpha1`:

```text
                 PLATFORM
                    │
        ┌───────────┼───────────┐
        │           │           │
    COMPONENTS   OFFERINGS   RELEASES
        │           │           │
        └───────────┼───────────┘
                    │
              ENVIRONMENTS
```

| Concept | What it is | Example |
| --- | --- | --- |
| **Component** | A building block, recognised by a catalog definition | Kubernetes, Argo CD, Crossplane, provider-upjet-azure |
| **Offering** | A capability you promise to users, backed by components | *Self-service databases*, *GitOps deployment*, *Secrets* |
| **Release** | A numbered bundle of component versions and offerings | Platform `1.2.0`: Kubernetes 1.34, Crossplane 2.0.2, … |
| **Environment** | Where the platform runs, and which release it should be on | `prod` targets `1.1.0` |

## Quick start

Try it on the bundled example platform. It uses exported inventories, so no
cluster is needed:

```shell
git clone https://github.com/ravibagri5/platform-bom
cd platform-bom
make ui build                       # needs Go 1.24+ and Node 20+
./bin/pbom serve -c examples/acme/pbom.yaml
```

Open <http://127.0.0.1:8080>.

Then point it at a real cluster. With no platform file, `pbom` uses the builtin
catalog only:

```shell
./bin/pbom discover --context my-cluster
```

No cluster handy? `make kind-demo` creates a kind cluster with Argo CD,
cert-manager and Crossplane, and [examples/kind](examples/kind) describes it.

Set `GITHUB_TOKEN` to avoid GitHub's anonymous rate limit when fetching
upstream releases, or pass `--no-upstream` to work offline.

## Installation

### From source

```shell
go install github.com/ravibagri5/platform-bom/cmd/pbom@latest
```

`go install` builds without the web UI, since the UI is compiled separately.
The CLI works fully; `pbom serve` answers the JSON API and explains how to
build the UI. For the complete binary, use `make ui build` or a release.

### Release binaries

Download an archive for your platform from the
[releases page](https://github.com/ravibagri5/platform-bom/releases). Archives
are published for Linux, macOS and Windows on amd64 and arm64, with SBOMs and a
cosign-signed checksum file.

### Container image

```shell
docker run --rm -p 8080:8080 \
  -v "$PWD:/config:ro" \
  ghcr.io/ravibagri5/platform-bom:latest
```

See [Running in a container](#running-in-a-container) for credentials.

## Using it with your clusters

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

## Configuration

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
[internal/catalog/components](internal/catalog/components); add or override
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

## How discovery works

For each environment, `pbom` reads:

| Source | What it looks at |
| --- | --- |
| Cluster | Server version and nodes, to identify EKS, AKS, GKE, kind, k3s and RKE2 |
| Workloads | Container images in Deployments, StatefulSets and DaemonSets |
| Helm | Release metadata in Secrets labelled `owner=helm,status=deployed` |
| Crossplane | Providers, Functions and Configurations in `pkg.crossplane.io` |
| API groups | Served groups, to spot components whose version cannot be read |

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

For clusters that the machine running `pbom` cannot reach, export inventories
where you do have access and commit them:

```shell
pbom discover --env prod -o yaml > inventories/prod.yaml
```

## Required RBAC

Discovery only uses `get` and `list`. A least-privilege ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: platform-bom-reader
rules:
  - apiGroups: [""]
    resources: [nodes, secrets]   # secrets: Helm release metadata; drop to skip Helm
    verbs: [get, list]
  - apiGroups: [apps]
    resources: [deployments, statefulsets, daemonsets]
    verbs: [get, list]
  - apiGroups: [pkg.crossplane.io]
    resources: [providers, functions, configurations]
    verbs: [get, list]
```

Helm stores release values in Secrets, and Kubernetes RBAC cannot restrict
access by label, so reading Helm metadata requires cluster-wide `list secrets`.
`pbom` requests only Secrets labelled `owner=helm` and reads nothing but the
chart name and versions from them, but the permission is broad. Leave it out if
that is not acceptable; Helm discovery is then skipped with a warning. See
[SECURITY.md](SECURITY.md) for the full threat model.

## Running in a container

```shell
docker run --rm -p 8080:8080 \
  -v "$PWD:/config:ro" \
  -v "$HOME/.kube/config:/home/nonroot/.kube/config:ro" \
  -e GITHUB_TOKEN \
  ghcr.io/ravibagri5/platform-bom:latest
```

The image runs as a non-root user and expects the platform file at
`/config/pbom.yaml`. Kubeconfigs that use exec plugins such as `kubelogin` or
`aws eks get-token` need those binaries, so for those, run the binary directly
or use exported inventories.

The UI has no authentication of its own. It binds to `127.0.0.1` by default;
put it behind your organisation's authenticating proxy before exposing it.

## Running in a cluster

[deploy/](deploy) is a kustomize base that runs `pbom serve` in its own
namespace with a read-only ClusterRole. The pod runs as non-root with a
read-only root filesystem and no capabilities, and the namespace enforces the
`restricted` Pod Security Standard.

1. **Describe the platform** in [deploy/platform/pbom.yaml](deploy/platform/pbom.yaml).
   The cluster `pbom` runs in is an environment with `inCluster: true`:

   ```yaml
   environments:
     - name: prod
       inCluster: true
       targetRelease: "1.0.0"
   ```

2. **Add releases** by copying them into `deploy/releases/` and listing them
   under the `pbom-releases` generator in
   [deploy/kustomization.yaml](deploy/kustomization.yaml). Both files become
   ConfigMaps whose names carry a content hash, so changing either rolls the
   Deployment.

3. **Optionally add a GitHub token** for upstream release checks:

   ```shell
   kubectl create namespace platform-bom
   kubectl -n platform-bom create secret generic pbom-github --from-literal=token="$GITHUB_TOKEN"
   ```

4. **Apply and open it:**

   ```shell
   kubectl apply -k deploy
   kubectl -n platform-bom port-forward svc/pbom 8080:80
   ```

   Then browse to <http://127.0.0.1:8080>.

### Discovering other clusters from inside one

A single in-cluster `pbom` can discover other clusters through a kubeconfig.
Create a Secret holding a kubeconfig with one context per cluster, uncomment
the `kubeconfig` volume in [deploy/deployment.yaml](deploy/deployment.yaml),
and reference it from each environment:

```shell
kubectl -n platform-bom create secret generic pbom-kubeconfig --from-file=config=./pbom-kubeconfig
```

```yaml
    - name: staging
      kubeconfig: /kubeconfig/config
      kubeContext: staging
```

The image contains no credential plugins such as `kubelogin` or
`aws eks get-token`, so the kubeconfig must carry a token, typically for a
service account bound to the same read-only ClusterRole in each target cluster.
Where that is not possible, run `pbom discover -o yaml` in a job that can reach
the cluster and use the result as an `inventoryFile`.

### Trying it on kind

Until a release is published, build the image locally and load it into kind:

```shell
docker build -t ghcr.io/ravibagri5/platform-bom:dev .
kind load docker-image ghcr.io/ravibagri5/platform-bom:dev --name <cluster>
(cd deploy && kustomize edit set image ghcr.io/ravibagri5/platform-bom:dev)
kubectl apply -k deploy
```

### Exposing it

The Service is `ClusterIP` on purpose: the UI has no authentication. To share
it, put it behind an Ingress or Gateway that authenticates users, for example
with oauth2-proxy or your identity-aware proxy.

## HTTP API

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

## Development

You need Go 1.24+ and Node 20+.

```shell
make ui build      # build the UI and the binary
make test          # unit tests
make lint          # golangci-lint
make check         # everything CI runs
make run           # serve the example platform
make dev-ui        # Vite dev server on :5173, proxying /api to :8080
```

```text
cmd/pbom             CLI entry point
internal/api         resource model: Platform, PlatformRelease, Component, Inventory
internal/catalog     component catalog, builtin definitions and matching
internal/discovery   Kubernetes evidence collection
internal/upstream    GitHub releases client with disk cache
internal/analysis    matrix, drift, freshness and recommendations
internal/release     release load, create and diff
internal/service     wiring shared by the CLI and the server
internal/server      HTTP API and embedded UI
web/                 React UI, built into internal/ui/dist
examples/            an example platform and a kind demo
```

## Roadmap

Highlights from [ROADMAP.md](ROADMAP.md):

- Argo CD and Flux as evidence sources, to compare Git's desired state with the
  running version
- End-of-life data and security advisories alongside upstream releases
- Compatibility rules between components, for example Karpenter and Kubernetes
- Inventory history and a "what changed this week" view
- An in-cluster agent that pushes inventories to a central `pbom`

## Contributing

Contributions are very welcome, and the easiest place to start is the component
catalog: one YAML file teaches `pbom` about a new tool for everyone. See
[CONTRIBUTING.md](CONTRIBUTING.md). This project follows the
[CNCF Code of Conduct](CODE_OF_CONDUCT.md).

## Security

Please report vulnerabilities privately through
[GitHub security advisories](https://github.com/ravibagri5/platform-bom/security/advisories/new),
not public issues. See [SECURITY.md](SECURITY.md).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
