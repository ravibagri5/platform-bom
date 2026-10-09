# Getting started with Platform BOM

Install PBOM in an existing Kubernetes cluster and open its inventory before
writing any platform YAML. Helm deploys the container, configuration, private
Service and read-only discovery permissions. No CRDs or GitOps controller are
required. PBOM observes components; it does not install or update them.

## 1. Check prerequisites

- Helm 3.8+ (OCI support) or Helm 4, and `kubectl` on your workstation.
- A kubeconfig for the cluster, including any credential plugin your provider requires.
- Permission to create a namespace and cluster-scoped discovery RBAC. If your
  account lacks this, ask a cluster administrator to install PBOM.
- Cluster access to `ghcr.io` to pull the PBOM image. Outbound GitHub access is
  needed only for upstream version checks.
- Git only if installing the chart from a source checkout.

Go, Node.js, npm, kind and Docker on your workstation are **not** required for
this installation.

Check the context carefully; this is the cluster PBOM will discover:

```sh
helm version
kubectl version --client
kubectl config current-context
kubectl get nodes
kubectl auth can-i create namespaces
kubectl auth can-i create clusterroles.rbac.authorization.k8s.io
kubectl auth can-i create clusterrolebindings.rbac.authorization.k8s.io
```

The default chart grants `get`/`list` on nodes, workloads, Crossplane packages
and Secrets. Helm stores release metadata in Secrets; Kubernetes RBAC cannot
limit this permission by label. Add `--set rbac.helmSecrets=false` to the
install command if cluster-wide Secret reading is unacceptable. Other discovery
continues, with a warning for skipped Helm discovery.

## 2. Install

### From this checkout (available now)

```sh
git clone https://github.com/ravibagri5/platform-bom.git
cd platform-bom
helm upgrade --install pbom ./charts/platform-bom \
  --namespace platform-bom --create-namespace --wait --timeout 3m
```

If you already cloned the repository, run only the Helm command from its root.
It uses the versioned image in the chart; no local image build is needed.

### From a published chart (after the next chart release)

The release workflow publishes versioned OCI charts to
`ghcr.io/ravibagri5/charts/platform-bom`. This is a new distribution path;
existing binary/image releases do not retroactively contain a chart. After a
chart release is published, set `PBOM_VERSION` to that release's version
without the leading `v`, then run:

```sh
helm upgrade --install pbom oci://ghcr.io/ravibagri5/charts/platform-bom \
  --version "$PBOM_VERSION" \
  --namespace platform-bom --create-namespace --wait --timeout 3m
```

The chart and default image use the same release version. No `helm repo add`,
source checkout or values file is needed.

## 3. Open the inventory

```sh
kubectl -n platform-bom rollout status deployment/pbom-platform-bom --timeout=180s
kubectl -n platform-bom get pods
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Keep port-forward running and open <http://127.0.0.1:8080>. Inspect Components
and Environments for discovered versions and evidence. PBOM refreshes every
10 minutes by default. It is not an exhaustive Kubernetes resource browser:
components recognised by the catalog are named and compared with upstream,
and other Helm releases and Crossplane packages are listed as unclassified.
Offerings are empty and there is no release baseline until you choose to
declare them; see [Next steps](#next-steps-from-inventory-to-a-managed-platform).
Missing Crossplane CRDs are normal when Crossplane is not installed.

The Service is ClusterIP. PBOM has no built-in authentication; do not expose
it publicly without an authenticating proxy.

## 4. Operate it

For a published chart, upgrade by changing `PBOM_VERSION` to a reviewed release
and re-running the OCI install command. For a checkout, update the chart source
and re-run the local install command. Keep any `--set` overrides or values file
in subsequent commands so your chosen configuration is preserved.

Inspect the running configuration and logs:

```sh
helm -n platform-bom list
helm -n platform-bom get values pbom --all
kubectl -n platform-bom logs deployment/pbom-platform-bom --tail=100
```

To remove PBOM, including the RBAC created by this release:

```sh
helm uninstall pbom --namespace platform-bom
```

The namespace is left in place. Removing PBOM does not change discovered
components or their Helm releases.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Install denied / `Forbidden` | Ask an administrator to grant installation permissions. Discovery errors may indicate missing reader permissions; check pod logs. |
| `ImagePullBackOff` | Use `kubectl -n platform-bom describe pod` to check the tag, GHCR access and pull credentials. |
| OCI chart not found | The chart may not have been published yet; use the checkout command. Confirm the chart package is public and the version exists. |
| No inventory | Check logs, the cluster context and reader RBAC. An empty cluster may contain few recognized components. |
| Helm versions missing | Check whether `rbac.helmSecrets` was disabled, and whether the cluster uses Helm Secret storage. |
| Upstream versions missing | Check outbound GitHub access or anonymous rate limits; observed cluster inventory does not require a GitHub token. |
| Port-forward cannot connect | Confirm rollout readiness and Service name. With another Helm release name, use the commands printed by Helm. |

An optional GitHub token can be supplied through an existing Secret managed
outside Git. Set `githubTokenSecret.name` (and `githubTokenSecret.key` if it is
not `token`) on install or upgrade. Never put the token in Helm values or Git.

## Next steps: from inventory to a managed platform

The install shows what runs. Each step below adds meaning to it and is
optional; do them in order, when you need them. Keep your settings in one
values file, for example `pbom-values.yaml`, and apply every change with:

```sh
helm upgrade --install pbom ./charts/platform-bom -n platform-bom -f pbom-values.yaml
```

(Use the OCI reference and `--version` instead of `./charts/platform-bom` for a
published chart.) The pod restarts automatically when configuration changes.

PBOM reads four kinds of document. With the chart, three are Helm values and
none are Kubernetes resources or CRDs:

| Kind | What it says | Who writes it | With the chart |
| --- | --- | --- | --- |
| `Platform` | What the platform is: name, owners, offerings, environments and each environment's target release | You | `platform` value; the default discovers this cluster |
| `Component` | How to recognise one tool and where its upstream releases are | Built-in catalog of common tools, plus yours | `components` value |
| `PlatformRelease` | A versioned baseline: the component versions and offerings you support together | You, or `pbom release create` | `releases` value |
| `Inventory` | What one environment was found running, with evidence | `pbom discover -o yaml` | Not needed for this cluster; only for clusters PBOM cannot reach |

The full schema of each is in [usage.md](usage.md#configuration).

### Name the platform and describe its offerings

Offerings are what application teams get, such as "Kubernetes workloads" or
"PostgreSQL", linked to the components that provide them. Use component names
from the Components page:

```yaml
platform:
  metadata:
    name: team-platform
    displayName: Team Platform
  spec:
    tagline: What our platform provides and where it runs.
    offerings:
      - name: gitops
        displayName: GitOps delivery
        category: delivery
        status: ga
        components: [argocd]
    environments:
      - name: prod
        displayName: Production
        inCluster: true
```

Helm merges maps with the chart defaults but replaces lists as a whole, so
`environments` and `offerings` must always be complete. Keep an environment
with `inCluster: true`, and repeat its full entry when you change it later.

### Teach PBOM about unclassified tools

Helm releases and Crossplane packages the built-in catalog does not recognise
are listed as unclassified, with a version but no upstream comparison. Add a
`Component` for each one you care about. The entry name becomes the file name:

```yaml
components:
  internal-gateway.yaml:
    apiVersion: pbom.dev/v1alpha1
    kind: Component
    metadata:
      name: internal-gateway
    spec:
      displayName: Internal Gateway
      category: networking
      discovery:
        helmCharts: ["internal-gateway"]
      upstream:
        github: acme/internal-gateway   # optional; enables update checks
```

A definition with the same `metadata.name` as a built-in one replaces it. To
keep definitions as files, pass them with
`--set-file 'components.internal\.yaml=components/internal.yaml'`; a file may
hold several documents separated by `---`. Definitions useful to others belong
in the built-in catalog; see [CONTRIBUTING.md](../CONTRIBUTING.md).

### Record your first release baseline

A release says "this is the platform we support". Cut one from what the
cluster runs today, from a workstation with the [`pbom` CLI](deployment.md)
and a kubeconfig context for the cluster. Use a small local platform file, for
example `baseline/pbom.yaml`, that names the environment the same way:

```yaml
apiVersion: pbom.dev/v1alpha1
kind: Platform
metadata:
  name: team-platform
spec:
  environments:
    - name: prod
      kubeContext: prod-cluster   # your kubeconfig context
```

```sh
pbom -c baseline/pbom.yaml release create 1.0.0 --from-env prod --summary "First baseline"
```

This writes `baseline/releases/1.0.0.yaml` with exact versions. Review it,
loosen versions you do not want to pin (`"1.36"` accepts any 1.36 patch),
remove components that are not part of the platform, then add it and set the
target release:

```yaml
platform:
  spec:
    environments:
      - name: prod
        inCluster: true
        targetRelease: "1.0.0"
```

```sh
helm upgrade --install pbom ./charts/platform-bom -n platform-bom -f pbom-values.yaml \
  --set-file 'releases.1\.0\.0\.yaml=baseline/releases/1.0.0.yaml'
```

Or paste the document under `releases: {1.0.0.yaml: ...}` in the values file.
The Environments page now shows whether the cluster matches its release, and
Releases compares baselines as you add more.

### Turn on upstream checks with a token

Upstream versions come from GitHub. Anonymous requests are rate limited, so
create a token Secret outside Git and reference it:

```sh
kubectl -n platform-bom create secret generic pbom-github --from-literal=token="$GITHUB_TOKEN"
```

```yaml
githubTokenSecret:
  name: pbom-github
```

### Share it and manage it with Git

- Expose the UI behind authentication with the optional
  [Ingress](../charts/platform-bom/README.md#optional-ingress-and-tls).
- Commit `pbom-values.yaml` and let Argo CD apply it with
  [optional GitOps](gitops.md).
- Add more clusters with [multi-cluster discovery](deployment.md#discovering-other-clusters-from-inside-one),
  or use a [binary or container](deployment.md) where installing in the
  cluster is not appropriate.
