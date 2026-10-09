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
10 minutes by default. Only recognized catalog components are shown, not an
exhaustive Kubernetes resource browser. Offerings are empty and there is no
release baseline until you choose to declare them. Missing Crossplane CRDs
are normal when Crossplane is not installed.

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

## Optional next steps

- Customize platform names, offerings and release baselines using the chart's
  `platform` and `releases` values. See the [GitOps runbook](gitops.md) for an example.
- Manage the same chart declaratively with Argo CD using [optional GitOps](gitops.md).
- Use a [binary or container](deployment.md) if installing in the cluster is not appropriate.
- For multiple clusters, see [multi-cluster discovery](deployment.md#discovering-other-clusters-from-inside-one).
