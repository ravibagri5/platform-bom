# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The resource model (`Platform`, `PlatformRelease`, `Component`, `Inventory`)
and the CLI are the public API. While the API is `v1alpha1`, minor releases may
change it; every such change is listed under **Changed** with a migration note.

## [Unreleased]

### Added

- The resource model, under `apiVersion: pbom.dev/v1alpha1`: `Platform` with
  offerings, guarantees, owners and environments; `PlatformRelease`;
  `Component` catalog definitions; `Inventory`.
- Read-only discovery from kube contexts: cluster version and distribution
  (EKS, AKS, GKE, kind, k3s, RKE2), container images in Deployments,
  StatefulSets and DaemonSets, Helm release metadata, served API groups, and
  Crossplane Providers, Functions and Configurations.
- A builtin catalog of common components, including Kubernetes, Argo CD, Flux,
  Crossplane and its providers and functions, cert-manager, Kyverno, External
  Secrets, Vault Secrets Operator, Istio, Cilium, Karpenter, Prometheus,
  Grafana Alloy and Velero. `componentsDir` adds or overrides definitions.
- Components with no catalog definition are still reported: Crossplane
  Configurations as Platform APIs, their embedded functions nested beneath
  them, other providers and functions under Crossplane, and Helm releases under
  Other. `discovery.hideUnclassified` skips unmatched Helm releases.
- Each discovered component records its kind: cluster, controller, Helm chart,
  provider, function, configuration or API only.
- Prerelease versions such as `2.0.1-rc.3` are preserved and matched exactly.
- `pbom release create` cuts a release from a live environment; `pbom release
  diff` compares two releases.
- Drift against each environment's target release, and the newest release each
  environment fully satisfies.
- Upstream release tracking from GitHub with a disk cache, and explainable
  upgrade recommendations: patch, minor and major lag, upstream support
  windows, low-risk patch steps and version spread across environments.
- `pbom` CLI: `discover`, `matrix`, `drift --exit-code`, `updates --why`,
  `release list|show|create|diff`, `catalog`, `serve`, `version`.
- `pbom serve`: a JSON API and an embedded web UI with Platform, Components,
  Offerings, Releases and Environments views.
- An example platform using exported inventories, and a kind demo.
- `inCluster: true` on an environment discovers the cluster `pbom` runs in,
  using its service account.
- `deploy/`, a kustomize base for running `pbom serve` in a cluster with a
  read-only ClusterRole, a hardened pod and the platform and releases as
  ConfigMaps.
