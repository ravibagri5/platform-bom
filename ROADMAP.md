# Roadmap

This is where the project is heading. It is a statement of intent, not a
commitment to dates. If something here matters to you, say so on the issue or
open a discussion; that is how priorities move.

The guiding question for every item: *does this help a platform team see,
version or explain the platform they run, from the tools they already use?*

## How planning works

- **Milestones are themes**, M0 to M11, matching the roadmap in the
  [README](README.md#roadmap). Each has a GitHub milestone with its issues.
- **Releases are cut from `main`** when enough has landed, not per milestone.
  A release usually contains work from several milestones.
- **Priority labels decide the order.** `priority/high` issues are what the
  next releases focus on.
- Every issue is on the
  [platform-bom Roadmap](https://github.com/users/ravibagri5/projects/2) board.
  See [docs/community.md](docs/community.md#planning).

## Next: the foundation for integrations

Every integration needs a stable contract, so the versioned API comes first.
In priority order:

1. [#8](https://github.com/ravibagri5/platform-bom/issues/8) Versioned REST API
   with an OpenAPI specification
2. [#13](https://github.com/ravibagri5/platform-bom/issues/13) MCP server
3. [#14](https://github.com/ravibagri5/platform-bom/issues/14) Backstage plugin
4. [#1](https://github.com/ravibagri5/platform-bom/issues/1) Argo CD
   Applications as evidence
5. [#4](https://github.com/ravibagri5/platform-bom/issues/4) End-of-life data
   and managed Kubernetes support windows

Good places to contribute alongside: the
[k9s plugin](https://github.com/ravibagri5/platform-bom/issues/16),
[Prometheus metrics](https://github.com/ravibagri5/platform-bom/issues/9) and
the [Headlamp plugin](https://github.com/ravibagri5/platform-bom/issues/15).

## By milestone

### M0 — Project foundation · M1 — Kubernetes discovery

Done in `v0.1`: the resource model (`Platform`, `PlatformRelease`, `Component`,
`Inventory`), read-only discovery of cluster version and distribution,
workload images, Helm releases, API groups and Crossplane packages, the builtin
catalog, the CLI, the JSON API and the web UI.

### M2 — Platform inventory

- [#7](https://github.com/ravibagri5/platform-bom/issues/7) Inventory history
  and what changed this week
- [#24](https://github.com/ravibagri5/platform-bom/issues/24) In-cluster agent
  that pushes inventories to a central pbom

### M3 — Platform releases and PBOM

- [#3](https://github.com/ravibagri5/platform-bom/issues/3) `pbom release lint`
- [#25](https://github.com/ravibagri5/platform-bom/issues/25) Export a platform
  release as CycloneDX
- [#26](https://github.com/ravibagri5/platform-bom/issues/26) Bind offerings to
  implementations per environment

### M4 — Upstream intelligence

- [#4](https://github.com/ravibagri5/platform-bom/issues/4) End-of-life data
- [#5](https://github.com/ravibagri5/platform-bom/issues/5) Security advisories
  for running versions
- [#6](https://github.com/ravibagri5/platform-bom/issues/6) Compatibility rules
  between components
- [#27](https://github.com/ravibagri5/platform-bom/issues/27) Upgrade paths to
  a supported version

### M5 — GitOps and IaC discovery

- [#1](https://github.com/ravibagri5/platform-bom/issues/1) Argo CD
  Applications as evidence
- [#2](https://github.com/ravibagri5/platform-bom/issues/2) Flux HelmReleases
  and Kustomizations as evidence
- [#28](https://github.com/ravibagri5/platform-bom/issues/28) Terraform/OpenTofu
  state and Pulumi stacks as evidence
- [#29](https://github.com/ravibagri5/platform-bom/issues/29) Environments that
  are not Kubernetes clusters

### M6 — Cloud evidence

- [#30](https://github.com/ravibagri5/platform-bom/issues/30) Correlate cloud
  managed services with the offerings they implement

### M7 — Ecosystem integrations

- [#14](https://github.com/ravibagri5/platform-bom/issues/14) Backstage plugin
- [#15](https://github.com/ravibagri5/platform-bom/issues/15) Headlamp plugin
- [#16](https://github.com/ravibagri5/platform-bom/issues/16) k9s plugin
- [#17](https://github.com/ravibagri5/platform-bom/issues/17) GitHub Action for
  drift checks
- [#18](https://github.com/ravibagri5/platform-bom/issues/18) GitLab CI/CD
  component
- [#19](https://github.com/ravibagri5/platform-bom/issues/19) Argo CD UI
  extension
- [#20](https://github.com/ravibagri5/platform-bom/issues/20) Kargo

### M8 — Kubernetes-native integrations

- [#21](https://github.com/ravibagri5/platform-bom/issues/21) Crossplane
  provider or function
- [#22](https://github.com/ravibagri5/platform-bom/issues/22) Platform and
  PlatformRelease as custom resources
- [#23](https://github.com/ravibagri5/platform-bom/issues/23) Helm chart

### M9 — Developer interfaces

- [#8](https://github.com/ravibagri5/platform-bom/issues/8) Versioned REST API
  with OpenAPI
- [#9](https://github.com/ravibagri5/platform-bom/issues/9) Prometheus metrics
- [#10](https://github.com/ravibagri5/platform-bom/issues/10) kubectl plugin
  through krew
- [#11](https://github.com/ravibagri5/platform-bom/issues/11) Go client
- [#12](https://github.com/ravibagri5/platform-bom/issues/12) Webhooks or events

### M10 — MCP server

- [#13](https://github.com/ravibagri5/platform-bom/issues/13) Read-only MCP
  server over stdio and HTTP

### M11 — PBOM specification

- [#31](https://github.com/ravibagri5/platform-bom/issues/31) Draft PBOM
  specification with JSON Schema
- [#32](https://github.com/ravibagri5/platform-bom/issues/32) Publish platform
  releases as OCI artifacts

## Not planned

- **Writing to what pbom discovers.** `pbom` observes; it does not deploy,
  upgrade or reconcile. That is what your GitOps tooling is for.
- **A general Kubernetes dashboard.** Pods, logs and events are well served by
  existing tools, including the ones pbom plugs into.
- **A generic cloud inventory.** Cloud evidence is limited to resources that
  implement platform components or offerings.
