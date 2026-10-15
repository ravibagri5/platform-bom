# Roadmap

This is where the project is heading. It is a statement of intent, not a
commitment to dates. If something here matters to you, say so on the issue or
open a discussion; that is how priorities move.

## Vision

**PBOM becomes the system of record for the lifecycle of an internal platform
across a fleet of clusters:** what runs where, whether it is supported and
secure, what to upgrade next, who is affected, and the evidence to prove it,
without write access to your clusters.

Platform teams answer the same questions every week, from tools that each see
part of the picture:

| Job | The question | Where PBOM is heading |
| --- | --- | --- |
| **Know** | What runs where, at which version, owned by whom? | Fleet inventory with evidence (done), history, owners and consumers (next) |
| **Keep current** | What is end of life, what is compatible, in which order do we upgrade? | Lifecycle data and upgrade plans (now) |
| **Keep safe** | Which clusters run a vulnerable version, right now? | Fleet-wide advisory exposure and alerts (now) |
| **Change safely** | What changed, and what will an upgrade affect? | History, drift, upgrade pull requests (next, later) |
| **Communicate** | Which teams use the offering we are changing? | Offerings with interfaces and consumers (next) |
| **Prove** | Show the auditor the assets, versions, patch status and history | Signed snapshots, controls, exceptions, audit reports (next) |
| **Show value** | How fast do we patch, how far behind are we, who uses what? | Platform KPIs from history (then) |

Every item on this page has to answer one of these questions better than the
tools platform teams already use. The reasoning behind the priorities is on the
[Vision](https://github.com/ravibagri5/platform-bom/wiki/Vision) wiki page.

## Principles

- **Read-only towards clusters.** PBOM observes. Where it helps you act, it
  proposes changes as pull requests to Git, which you review and your GitOps
  tooling applies.
- **Bring in results, don't rebuild them.** Trivy, Kyverno, kube-bench, Argo CD
  and Flux become evidence sources, like Helm and Crossplane are today.
- **The data is the product.** A community-maintained knowledge base of
  compatibility, end of life, removed APIs and advisories per component,
  collected automatically where possible and published as signed data.
- **Works offline and air-gapped.** No mandatory SaaS. Data bundles and
  history live in your Git or OCI registry.
- **Integrate with portals, don't become one.** Backstage stays the front door
  for developers; PBOM is the system of record behind it.
- **Declared state is code.** Platforms, releases, offerings and exceptions are
  resources reviewed in pull requests.

## Where we are

`v1.2.0` covers the *Know* job for one or many clusters: discovery from
Kubernetes, Helm and Crossplane with evidence; 92 catalogued components plus
the CNCF landscape; platforms, releases, components and inventories as
Kubernetes custom resources; drift against declared releases; upstream
versions; OIDC sign-in; workload identity for AKS, EKS and GKE; Prometheus
metrics, alerts and a Grafana dashboard. See the [CHANGELOG](CHANGELOG.md).

## Priorities

### Now: keep current and keep safe

Goal: for every cluster, answer *can we upgrade?* and *are we affected?* in
seconds.

1. [#4](https://github.com/ravibagri5/platform-bom/issues/4) End-of-life data
   from endoflife.date and managed Kubernetes support windows
2. [#6](https://github.com/ravibagri5/platform-bom/issues/6) Compatibility
   between components and Kubernetes versions, collected from Helm chart
   `kubeVersion` constraints and the catalog
3. [#62](https://github.com/ravibagri5/platform-bom/issues/62) Removed APIs in
   use, from the Kubernetes deprecation schedule and the API versions each
   cluster serves
4. [#27](https://github.com/ravibagri5/platform-bom/issues/27) Upgrade plans:
   `pbom plan --to <kubernetes version>` with ordered add-on upgrades, rollout
   waves from dev to prod, and a CI gate
5. [#5](https://github.com/ravibagri5/platform-bom/issues/5) Security
   advisories from OSV and the GitHub Advisory Database: fleet-wide exposure,
   a metric and an alert
6. [#63](https://github.com/ravibagri5/platform-bom/issues/63) A lifecycle
   data bundle: signed, released separately from the binary, refreshed daily
   and mirrorable for air-gapped clusters
7. [#17](https://github.com/ravibagri5/platform-bom/issues/17) GitHub Action
   that runs drift and upgrade checks in pull requests
8. [#40](https://github.com/ravibagri5/platform-bom/issues/40) Authorization:
   who can see which environments

### Next: system of record and evidence

Goal: fleet history and evidence an auditor accepts.

1. [#24](https://github.com/ravibagri5/platform-bom/issues/24) Agent mode:
   clusters push their inventories, so a central PBOM needs no credentials to
   them
2. [#7](https://github.com/ravibagri5/platform-bom/issues/7) and
   [#32](https://github.com/ravibagri5/platform-bom/issues/32) History: signed
   inventory snapshots in Git or an OCI registry, and *what changed this week*
3. [#26](https://github.com/ravibagri5/platform-bom/issues/26) Offerings as
   contracts: type, owner, interface, lifecycle (status, sunset, successor) and
   environments, with availability and consumers derived from the cluster
4. [#64](https://github.com/ravibagri5/platform-bom/issues/64) Ownership: an
   owner (team and contact) for every component in the platform,
   inherited from its offering or set explicitly, and consuming teams taken
   from namespace labels or a Backstage catalog. Unowned components become a
   finding, and alerts, upgrade plans and notices carry the owner so they reach
   the right team
5. [#65](https://github.com/ravibagri5/platform-bom/issues/65) Capability
   coverage and health: which capabilities a platform lacks against
   a chosen profile, and evidence that the ones it has work, such as a recent
   successful backup and restore test, certificates far from expiry, and
   policies enforcing
6. [#66](https://github.com/ravibagri5/platform-bom/issues/66) Controls and
   exceptions: rules such as *no end-of-life components in prod*
   evaluated over time, and a `PlatformException` resource with an owner,
   reason and expiry
7. [#67](https://github.com/ravibagri5/platform-bom/issues/67) and
   [#25](https://github.com/ravibagri5/platform-bom/issues/25)
   `pbom audit report`: inventory, changes, deviations, exceptions,
   remediation times and coverage for a period, as a signed bundle with
   CycloneDX and optionally OSCAL
8. [#3](https://github.com/ravibagri5/platform-bom/issues/3) `pbom release lint`
9. [#8](https://github.com/ravibagri5/platform-bom/issues/8) Versioned REST API
   with OpenAPI, the contract for everything in *Later*

### Later: act and communicate

Goal: turn insight into reviewed changes and reach the teams it affects.

1. [#68](https://github.com/ravibagri5/platform-bom/issues/68) Upgrade pull
   requests: an upgrade plan becomes ordered pull requests to
   GitOps repositories, with summaries of upstream release notes, opened by
   PBOM or by Renovate
2. [#13](https://github.com/ravibagri5/platform-bom/issues/13) MCP server: an
   AI assistant that plans upgrades and answers *are we affected?* from PBOM's
   data
3. [#14](https://github.com/ravibagri5/platform-bom/issues/14) Backstage
   plugin: offerings, consumers and notices where developers already look
4. [#12](https://github.com/ravibagri5/platform-bom/issues/12) Events and
   notices to affected teams, for deprecations, sunsets and advisories
5. [#1](https://github.com/ravibagri5/platform-bom/issues/1) and
   [#2](https://github.com/ravibagri5/platform-bom/issues/2) Argo CD and Flux
   as evidence, linking changes to their Git history

### Then: show value and grow the ecosystem

Goal: make the platform team's impact visible and the knowledge base shared.

1. [#69](https://github.com/ravibagri5/platform-bom/issues/69) Platform KPIs:
   patch latency, upgrade lag, share of the fleet on supported
   versions and offering adoption, in the Grafana dashboard and the UI
2. The lifecycle knowledge base as a community resource, with contributions
   through the component catalog
3. A CNCF Sandbox application, once real users and contributors are there
4. Integrations driven by demand:
   [#10](https://github.com/ravibagri5/platform-bom/issues/10) kubectl plugin,
   [#11](https://github.com/ravibagri5/platform-bom/issues/11) Go client,
   [#15](https://github.com/ravibagri5/platform-bom/issues/15) Headlamp,
   [#16](https://github.com/ravibagri5/platform-bom/issues/16) k9s,
   [#18](https://github.com/ravibagri5/platform-bom/issues/18) GitLab CI/CD,
   [#19](https://github.com/ravibagri5/platform-bom/issues/19) Argo CD UI
   extension, [#20](https://github.com/ravibagri5/platform-bom/issues/20) Kargo

### Exploring

No commitment; we will decide with users.

- [#21](https://github.com/ravibagri5/platform-bom/issues/21) Crossplane:
  first document how compositions read `PlatformRelease` resources with
  existing functions; a dedicated function only for a use case that needs
  PBOM's analysis
- [#28](https://github.com/ravibagri5/platform-bom/issues/28) Terraform,
  OpenTofu and Pulumi,
  [#29](https://github.com/ravibagri5/platform-bom/issues/29) environments that
  are not Kubernetes clusters and
  [#30](https://github.com/ravibagri5/platform-bom/issues/30) cloud managed
  services as evidence
- [#31](https://github.com/ravibagri5/platform-bom/issues/31) A PBOM
  specification: likely aligned with CycloneDX rather than a new format

## Done

| Version | Highlights |
| --- | --- |
| `v0.1` to `v0.5` | Resource model, discovery, catalog, CLI, API, web UI, releases and drift, Helm chart ([#23](https://github.com/ravibagri5/platform-bom/issues/23)) |
| `v0.6` to `v0.9` | Multi-cluster credentials ([#43](https://github.com/ravibagri5/platform-bom/issues/43)), workload identity ([#42](https://github.com/ravibagri5/platform-bom/issues/42)), OIDC sign-in ([#39](https://github.com/ravibagri5/platform-bom/issues/39)), dark theme ([#41](https://github.com/ravibagri5/platform-bom/issues/41)) |
| `v1.0` | Platforms, releases, components and inventories as custom resources ([#22](https://github.com/ravibagri5/platform-bom/issues/22)), CNCF landscape classification, drafts on first install |
| `v1.1` | Drafted first release, release snapshots, upstream feed fallback |
| `v1.2` | Prometheus metrics, alerts, Grafana dashboard, structured logs ([#9](https://github.com/ravibagri5/platform-bom/issues/9)) |

## How planning works

- **This file sets the order.** *Now*, *Next*, *Later* and *Then* are
  priorities, not dates, and each is a GitHub milestone, plus *Exploring*.
  `priority/high` issues are what the next releases focus on.
- **Themes are labels and a board field** (`theme/*`) on the
  [platform-bom Roadmap](https://github.com/users/ravibagri5/projects/2)
  board. See [docs/community.md](docs/community.md#planning). The earlier
  theme milestones M0 to M12 are closed and keep their history.
- **Releases are cut from `main`** when enough has landed, not per milestone.

## Not planned

- **Writing to clusters.** PBOM does not deploy, upgrade or reconcile; changes
  go through Git and your GitOps tooling.
- **A general Kubernetes dashboard.** Pods, logs and events are well served by
  existing tools, including the ones PBOM plugs into.
- **A developer portal.** PBOM feeds Backstage and similar portals.
- **A compliance suite.** PBOM produces platform evidence and exports it to
  governance, risk and compliance tools; auditors decide compliance.
- **A generic cloud inventory.** Cloud evidence is limited to resources that
  implement platform components or offerings.
