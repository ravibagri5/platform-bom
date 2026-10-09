# Optional: manage PBOM with GitOps

Start with the [getting-started runbook](getting-started.md) to see inventory
without writing YAML. GitOps is an optional way to manage the installation
and your declared platform over time, not a prerequisite for discovery.

## Use the Helm chart with Argo CD

Prerequisites: an existing Argo CD installation, Git, `kubectl`, a repository
Argo CD can read, and permission for Argo CD to create the chart's cluster-wide
reader RBAC. You do not need Helm, Go or Node.js on your workstation because
Argo CD renders the chart. Use your organization's existing Argo CD project;
the example uses `default`.

1. Fork or import this repository and commit the chart to your fork. For
   production, pin `targetRevision` to a reviewed commit or tag instead of `main`.
2. Put the following Application in your GitOps bootstrap repository,
   replacing `YOUR_ORG`, and let your existing controller sync it. For a
   first Application, apply the file once with `kubectl apply -f`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-bom
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/YOUR_ORG/platform-bom.git
    targetRevision: main
    path: charts/platform-bom
    helm:
      releaseName: pbom
  destination:
    server: https://kubernetes.default.svc
    namespace: platform-bom
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

Verify the Application is synced and open the inventory:

```sh
kubectl -n argocd get application platform-bom
kubectl -n platform-bom rollout status deployment/pbom-platform-bom --timeout=180s
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Open <http://127.0.0.1:8080>. The same single-cluster defaults work without a
values file. Do not let Helm CLI and Argo CD manage the same installation at
the same time. For a trial installation made with Helm, uninstall it before
letting Argo CD deploy it; this briefly interrupts the UI but does not affect
the observed platform.

### Add a baseline only when you are ready

After discovery works, optionally add `platform-values.yaml` alongside
`Chart.yaml` in your fork. Set the following under `spec.source.helm` in the
Application, retaining `releaseName: pbom`. The path is relative to the chart:

```yaml
valueFiles:
  - platform-values.yaml
```

For example, put the following in that values file, replacing the Kubernetes
version with the minor shown in your inventory:

```yaml
platform:
  metadata:
    name: team-platform
    displayName: Team Platform
  spec:
    offerings:
      - name: workloads
        displayName: Kubernetes workloads
        category: runtime
        status: ga
        components: [kubernetes]
    environments:
      - name: this-cluster
        displayName: This cluster
        inCluster: true
        targetRelease: "1.0.0"
releases:
  1.0.0.yaml:
    apiVersion: pbom.dev/v1alpha1
    kind: PlatformRelease
    metadata:
      name: "1.0.0"
    spec:
      version: "1.0.0"
      summary: First approved platform baseline.
      components:
        kubernetes: "1.36"
```

Helm merges these values with the chart defaults, retaining the Platform
schema and `/releases` mount. Commit and push your changes. Argo CD syncs the
ConfigMaps, and the chart's configuration checksum rolls the pod. Releases
and drift now appear in the UI. To omit Helm Secret access, also set
`rbac.helmSecrets: false` in the values file.

## Alternative: use the Kustomize base

PBOM observes your platform; Argo CD (or your existing GitOps controller)
deploys PBOM itself. This walkthrough installs one PBOM instance in the same
cluster it discovers. It needs a Kubernetes cluster, `kubectl`, Git, and access to the published
`ghcr.io/ravibagri5/platform-bom:0.1.0` image. Use an Argo CD repository
credential if your fork is private. No Helm or Go installation is needed.

### 1. Put your platform in Git

Fork [platform-bom](https://github.com/ravibagri5/platform-bom) in GitHub (or
import it into your Git server), then clone **your fork**:

```sh
git clone https://github.com/YOUR_ORG/platform-bom.git
cd platform-bom
```

Replace `deploy/pbom.yaml` with the following minimal configuration.
The in-cluster service account discovers this cluster; `/releases` is the
separately mounted release ConfigMap, not a directory under `/config`:

```yaml
apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: team-platform
  displayName: Team Platform
spec:
  tagline: What our platform provides and where it runs.
  releasesDir: /releases
  offerings:
    - name: workloads
      displayName: Kubernetes workloads
      category: runtime
      status: ga
      components: [kubernetes]
  environments:
    - name: cluster
      displayName: Cluster
      inCluster: true
```

Keep the shipped `deploy/kustomization.yaml`, RBAC, Deployment, Service and
Namespace. The Kustomize base packages your platform file as a ConfigMap and
deploys the published image. Check the rendered resources, commit, and push:

```sh
kubectl kustomize deploy >/dev/null
git add deploy/pbom.yaml
git commit -m "Configure PBOM for our cluster"
git push origin main
```

There is deliberately no `targetRelease` yet: on first install, PBOM will
discover running components without claiming a baseline you have not cut.
The default `pbom-releases` ConfigMap is empty until you add a release.

### 2. Let Argo CD sync it

If your cluster does not already have Argo CD, install it once. Pin and review
an approved version for production; the following version is a working
starting point for a local cluster:

```sh
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.1.0/manifests/install.yaml
kubectl -n argocd rollout status deployment/argocd-repo-server --timeout=300s
```

In a cluster with Argo CD installed in `argocd`, apply this Application after
replacing `YOUR_ORG` with your GitHub organization or username. Save this YAML
as `bootstrap/platform-bom.yaml` in your GitOps bootstrap repository:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-bom
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/YOUR_ORG/platform-bom.git
    targetRevision: main
    path: deploy
  destination:
    server: https://kubernetes.default.svc
    namespace: platform-bom
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

Commit and push that file in your bootstrap repository, then let its existing
GitOps controller sync it. If this is your first Application, run
`kubectl apply -f bootstrap/platform-bom.yaml` once from that repository;
future changes to PBOM's `deploy/` directory are reconciled from Git. Argo CD
needs permission to create the
cluster-scoped reader Role and Binding in the base. If your Argo CD project
restricts cluster-scoped resources, allow them or have an administrator
install those resources first.

Verify the result and open the UI locally:

```sh
kubectl -n argocd get application platform-bom
kubectl -n platform-bom rollout status deployment/pbom --timeout=180s
kubectl -n platform-bom port-forward svc/pbom 8080:80
```

Visit <http://127.0.0.1:8080>. The Service is deliberately private. PBOM
has no built-in UI authentication; use an authenticating proxy before sharing
it. Its read-only ClusterRole lists Helm Secrets cluster-wide to discover
chart metadata; [review the RBAC tradeoff](deployment.md#required-rbac) and
remove that rule if you do not want Helm discovery. To fetch more upstream
releases, create the optional `pbom-github` Secret outside Git through your
secret manager; never commit a GitHub token.

### 3. Record a platform release

Once you see what is running, create `deploy/releases/1.0.0.yaml` in your
fork. Pin only versions you intentionally support; for example, replace
`1.36` with the Kubernetes minor shown on your own Environments page:

```yaml
apiVersion: pbom.dev/v1alpha1
kind: PlatformRelease
metadata:
  name: "1.0.0"
spec:
  version: "1.0.0"
  summary: First approved platform baseline.
  components:
    kubernetes: "1.36"
```

Add `releases/1.0.0.yaml` under the `files:` list of the `pbom-releases`
generator in `deploy/kustomization.yaml` (the path is relative to `deploy/`),
and set `targetRelease: "1.0.0"` on the `cluster` environment in
`deploy/pbom.yaml`. Commit and push both changes:

```sh
kubectl kustomize deploy >/dev/null
git add deploy/pbom.yaml deploy/releases/1.0.0.yaml deploy/kustomization.yaml
git commit -m "Declare platform release 1.0.0"
git push origin main
```

Argo CD syncs the new ConfigMaps and restarts PBOM with the declared target.
The UI now shows alignment, drift, and upstream versions. For other clusters,
use a service-account kubeconfig or exported inventories as described in
[multi-cluster discovery](deployment.md#discovering-other-clusters-from-inside-one).
