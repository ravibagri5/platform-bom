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

## Deploy with GitOps

1. Fork and clone this repository. Set `inCluster: true` and `releasesDir: /releases` in `deploy/pbom.yaml`.
2. Commit and push. Point an Argo CD Application at your fork's [Kustomize base](deploy/) (`path: deploy`).
3. Run `kubectl -n platform-bom port-forward svc/pbom 8080:80` and open <http://127.0.0.1:8080>.
4. Review discovery, then commit a release and target version. GitOps syncs subsequent changes automatically.

[Follow the step-by-step GitOps guide](docs/gitops.md) for copyable YAML,
bootstrap commands, RBAC caveats and the first-release workflow.

## Try it locally

Go 1.26+, Node 24+, Docker, kind and Helm are needed for the kind simulation:

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
| [Deployment](docs/deployment.md) | Installation, RBAC, containers and cluster deployment |
| [GitOps quick start](docs/gitops.md) | Fork, configure, sync with Argo CD, and cut a first release |
| [Kind simulation](examples/kind/simulation/README.md) | Recreate the screenshot environment and release drift |
| [Roadmap](ROADMAP.md) | Upcoming work and milestones |
| [Community](docs/community.md) | Planning, labels and participation |
| [Contributing](CONTRIBUTING.md) | Development and contribution guide |
| [Security](SECURITY.md) | Threat model and vulnerability reporting |

Platform BOM proposes a platform bill of materials; it is not an industry
standard. The resource schema is `v1alpha1` and may change. Licensed under
[Apache 2.0](LICENSE).
