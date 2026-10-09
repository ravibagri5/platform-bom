# Platform BOM

[![CI](https://github.com/ravibagri5/platform-bom/actions/workflows/ci.yaml/badge.svg)](https://github.com/ravibagri5/platform-bom/actions/workflows/ci.yaml)
[![GitHub release](https://img.shields.io/github/v/release/ravibagri5/platform-bom?sort=semver)](https://github.com/ravibagri5/platform-bom/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**Know what your platform provides, what runs in each environment, and what needs attention.**

Platform BOM (`pbom`) is a read-only inventory for platform teams. It connects
your declared platform releases to evidence discovered from Kubernetes, Helm
and Crossplane, then compares versions across environments and with upstream.

- **Explain the platform:** Link capabilities offered to teams with the components behind them.
- **See release drift:** Compare what dev, QA and prod run with their target platform release.
- **Plan updates:** See upstream versions and why a component may need upgrading.

## What is a Platform BOM?

A **Software Bill of Materials (SBOM)** describes what a software artifact is
made of. It is an established practice with established specifications, notably
[CycloneDX](https://cyclonedx.org/) and [SPDX](https://spdx.dev/).

A **Platform Bill of Materials (PBOM)** applies the same idea one level up: it
is a machine-readable description of an internal platform's components,
capabilities, versions and relationships across releases and environments.
PBOM is a concept proposed by this project, not an established industry
standard.

## The problem

An internal platform is a product assembled from Kubernetes, GitOps, networking,
security, observability, infrastructure services and the capabilities exposed
to application teams. These parts are defined and operated in different
repositories and tools, and change at different times across development,
testing and production environments.

That fragmentation makes basic questions hard to answer reliably: What does the
platform provide, and what implements each capability? Which versions are
running in each environment? Does production match its declared release? What
will an upgrade affect? Teams often reconstruct the answers from dashboards,
Git history, spreadsheets and tribal knowledge, so drift and upgrade impact are
easy to miss.

## Why existing tools are not enough

Existing tools provide important evidence, but each describes only part of the
picture:

| Tool | What it describes | What remains unanswered |
| --- | --- | --- |
| Kubernetes dashboards | Resources running in a cluster | Which resources make up the platform, what it offers, and how environments compare with the declared release |
| Developer portals | Applications, services, teams and ownership | The platform components and versions those services depend on |
| Cloud inventories | Cloud resources and configuration | How resources implement platform capabilities and fit into a versioned platform release |
| Version dashboards | Installed versions | The intended platform baseline, release drift, and which offerings depend on a component |
| SBOM tools such as CycloneDX and SPDX | Packages in a software artifact | The components, capabilities and versions of a running internal platform |

Platform BOM connects those views at the platform level. It links declared
offerings and versioned releases to discovered component evidence, compares
environments with their targets, and highlights drift and upstream updates. The
result is a shared, Git-reviewable account of what the platform provides, what
is actually running, and where teams should focus next. PBOM complements these
tools; it does not replace them.

## See it in action

The [three-cluster kind simulation](examples/kind/simulation/README.md) shows
21 components and 8 offerings. Dev and QA match release `1.1.0`; prod still
runs `1.0.0` and has 16 component versions off target. Three components were
current with upstream when these screenshots were taken.

The feature screenshots use the same landscape viewport. Select an image to
see it at full size. A compact overview is also available [here](docs/screenshots/platform-compact.jpg).

**Platform:** A quick view of components, offerings, environment count, release
alignment, upstream currency, and the capabilities teams receive.

[![Platform overview with offerings and environment health](docs/screenshots/platform.png)](docs/screenshots/platform.png)

 [Compact platform overview](docs/screenshots/platform-compact.jpg)

| Screen | Purpose | Preview |
| --- | --- | --- |
| **Components** | Scan technology categories and observed versions; expand one for evidence and environment details. | [![Discovered component versions](docs/screenshots/components.png)](docs/screenshots/components.png) |
| **Offerings** | See the capabilities teams receive; expand one for its implementing components and availability. | [![Capabilities linked to components](docs/screenshots/offerings.png)](docs/screenshots/offerings.png) |
| **Releases** | Review the declared versioned platform snapshots and compare what changed between them. | [![Platform release snapshots](docs/screenshots/releases.png)](docs/screenshots/releases.png) |
| **Environments** | Compare observed component versions with the Git-declared target release; pinpoint drift and untracked components. | [![Environment comparison and drift](docs/screenshots/environments.png)](docs/screenshots/environments.png) |
| **Updates** | Prioritize newer upstream releases with per-environment versions and reasons for attention. | [![Explainable upstream updates](docs/screenshots/updates.png)](docs/screenshots/updates.png) |

The simulation uses zero-replica discovery fixtures; it does not install or
operate the real products. Upstream releases and percentages can change over time.

## Getting started

Install the [Helm chart](charts/platform-bom/) in the cluster you want to
inventory. No platform YAML, declared releases, GitOps controller, Go, Node.js
or Docker installation is needed on your workstation.

**Prerequisites:** Helm 3.14+ or 4, `kubectl`, a working cluster context, and
permission to create a namespace, ClusterRole and ClusterRoleBinding. The
cluster must be able to pull the image from GHCR. The default reader includes
cluster-wide Secret access for Helm metadata; add `--set rbac.helmSecrets=false`
to skip it.

Until the chart is published, install it from a checkout (Git is needed for
this route):

```sh
git clone https://github.com/ravibagri5/platform-bom.git
cd platform-bom
helm upgrade --install pbom ./charts/platform-bom \
  --namespace platform-bom --create-namespace --wait --timeout 3m
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Open <http://127.0.0.1:8080>. PBOM discovers this cluster using its read-only
service account. The UI has no built-in authentication; keep the Service
private or use an authenticating proxy.

Next, [describe your platform in YAML](docs/getting-started.md#4-describe-your-platform-in-yaml):
a `Platform` with offerings and environments, `Component` files for tools PBOM
does not know, and `PlatformRelease` baselines. Apply them with
`kubectl apply -k`, then keep them in Git and let any CD tool apply them. The
[getting-started guide](docs/getting-started.md) walks through every step.

## Try it locally

Choose the prerequisites for the local route you need:

| Route | Prerequisites |
| --- | --- |
| Prebuilt `pbom` binary | Release archive for your OS/CPU; kubeconfig and any cluster credential plugin for live discovery. See [installing the binary](docs/deployment.md#install-the-pbom-binary) |
| Build the binary and UI | Git, Go 1.26+, Node.js 24+, npm (bundled with Node.js), Make |
| Three-cluster kind simulation | All build prerequisites, Docker with its daemon running, kind, `kubectl`, Helm 3.8+ or 4; enough memory for three Kubernetes nodes |
| Container | Docker with its daemon running, a platform config and a usable kubeconfig; no Go or Node.js |

From the repository root, verify the simulation tools first:

```sh
git --version
go version
node --version
npm --version
make --version
docker info
kind version
kubectl version --client
helm version
```

Then build and run the kind simulation:

```sh
make kind-clusters
make kind-simulation
make ui build
./bin/pbom -c examples/kind/simulation/pbom.yaml serve
```

Open <http://127.0.0.1:8080>. Set `GITHUB_TOKEN` if you hit GitHub's API rate
limit; `--no-upstream` skips upstream checks. For a cluster-free example, run
`./bin/pbom serve -c examples/acme/pbom.yaml` instead.

## Documentation

| Guide | Contents |
| --- | --- |
| [Overview and concepts](docs/overview.md) | The problem, platform model, evidence, architecture, integrations, principles and roadmap |
| [Usage](docs/usage.md) | Configuration, discovery, CLI, drift checks, web UI and API |
| [Getting started](docs/getting-started.md) | Install with Helm, describe the platform in YAML, apply it, keep it in Git, troubleshooting |
| [Deployment](docs/deployment.md) | Binary and container alternatives, RBAC and advanced cluster deployment |
| [Kind simulation](examples/kind/simulation/README.md) | Recreate the screenshot environment and release drift |
| [Roadmap](ROADMAP.md) | Upcoming work and milestones |
| [Community](docs/community.md) | Planning, labels and participation |
| [Contributing](CONTRIBUTING.md) | Development and contribution guide |
| [Security](SECURITY.md) | Threat model and vulnerability reporting |

Platform BOM proposes a platform bill of materials; it is not an industry
standard. The resource schema is `v1alpha1` and may change. Licensed under
[Apache 2.0](LICENSE).
