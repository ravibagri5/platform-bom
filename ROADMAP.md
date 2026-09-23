# Roadmap

This is where the project is heading, roughly in order. It is a statement of
intent, not a commitment to dates. If something here matters to you, say so in
the linked discussion or open an issue; that is how priorities move.

The guiding question for every item: *does this help a platform team see,
version or explain the platform they run?*

Work is tracked as issues under a milestone per release and on the
[platform-bom Roadmap](https://github.com/users/ravibagri5/projects) project
board. See [docs/community.md](docs/community.md#planning).

## Now: v0.1 — foundations

The first release. Everything below is implemented.

- The resource model: `Platform`, `PlatformRelease`, `Component`, `Inventory`
- Read-only discovery from kube contexts: cluster version and distribution,
  workload images, Helm releases, API groups and Crossplane packages
- A builtin catalog of common CNCF and cloud native components, extensible with
  `componentsDir`
- Automatic placement of uncatalogued Crossplane Configurations, Functions and
  Providers, and of Helm releases
- Releases cut from a live environment, release diffs, drift against each
  environment's target release
- Upstream release tracking from GitHub with explainable recommendations
- CLI, JSON API and an embedded web UI

## Next: v0.2 — the Git side of GitOps

Today `pbom` compares what is running with the release you declared. Most
platforms also have a third opinion: what Git says should be deployed.

- **Argo CD evidence source.** Read `Application` resources for the target
  revision, sync status and images, so the matrix can show *promised 3.5.3, Git
  3.5.3, running 3.5.4, OutOfSync*.
- **Flux evidence source.** The same for `HelmRelease` and `Kustomization`.
- **Release lint.** `pbom release lint` checks a release file against the
  catalog before it is merged: unknown components, versions that do not exist
  upstream.

## Later: v0.3 — lifecycle intelligence

- **End-of-life data** from [endoflife.date](https://endoflife.date) and managed
  Kubernetes support windows (EKS, AKS, GKE), so "unsupported" means the
  provider's calendar and not only upstream's.
- **Security advisories** for the versions you run, from GitHub Security
  Advisories and OSV.
- **Compatibility rules** between components, declared in the catalog: this
  Karpenter needs that Kubernetes, this provider needs that Crossplane.
- **Upgrade paths**: the smallest set of releases that gets a component from
  where it is to a supported version.

## Later: v0.4 — history

- **Inventory history.** Keep discovery snapshots so the UI can answer *what
  changed this week?* and show when each environment moved between releases.
- **Release timeline per environment**: when did production reach 1.2.0?
- **Export** a platform release as a CycloneDX BOM.

## Exploring

Ideas we think are worth doing but have not designed yet:

- An in-cluster agent that pushes inventories to a central `pbom`, for
  clusters the central server cannot reach
- Publishing platform releases as OCI artifacts
- Non-Kubernetes evidence: cloud provider APIs, Terraform state
- A Backstage plugin that renders the Platform page

## Not planned

- **Writing to clusters.** `pbom` observes; it does not deploy, upgrade or
  reconcile. That is what your GitOps tooling is for.
- **A general Kubernetes dashboard.** Pods, logs and events are well served by
  existing tools.
