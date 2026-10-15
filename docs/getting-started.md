# Getting started with Platform BOM

Install PBOM in a Kubernetes cluster with Helm, see what the cluster runs, then
describe your platform in a few YAML files. PBOM only reads the cluster; it
never installs or changes anything.

1. [Check prerequisites](#1-check-prerequisites)
2. [Install with Helm](#2-install-with-helm)
3. [Open the inventory](#3-open-the-inventory)
4. [Describe your platform in YAML](#4-describe-your-platform-in-yaml)
5. [Apply the YAML](#5-apply-the-yaml)
6. [Keep it in Git](#6-keep-it-in-git)
7. [Operate it](#7-operate-it)

[Clusters PBOM cannot reach](#optional-clusters-pbom-cannot-reach),
[Reference: what each file is for](#reference-what-each-file-is-for) and
[Troubleshooting](#troubleshooting) are at the end.

## 1. Check prerequisites

- Helm 3.14+ or Helm 4, and `kubectl`, pointing at the cluster to inventory.
- Permission to create a namespace, a ClusterRole, a ClusterRoleBinding and
  CustomResourceDefinitions.
- The cluster can pull images from `ghcr.io`.

```sh
kubectl config current-context    # this is the cluster PBOM will discover
kubectl auth can-i create clusterroles.rbac.authorization.k8s.io
kubectl auth can-i create customresourcedefinitions.apiextensions.k8s.io
```

## 2. Install with Helm

Install the published chart directly from GitHub Container Registry:

```sh
helm upgrade --install pbom oci://ghcr.io/ravibagri5/charts/platform-bom \
  --namespace platform-bom --create-namespace --wait
```

Helm installs the latest chart by default. Add `--version X.Y.Z` to pin a
specific release.

PBOM gets read-only access to nodes, workloads, Crossplane packages and Helm
release Secrets. Add `--set rbac.helmSecrets=false` if it must not read
Secrets; it then skips Helm releases. The chart also installs the four
`pbom.dev` custom resource definitions and lets PBOM read and watch those
resources in its own namespace; it never writes them.

## 3. Open the inventory

```sh
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Open <http://127.0.0.1:8080>. **Components** lists what PBOM recognised, with
versions, evidence and the latest upstream release. Helm releases of CNCF
projects outside the built-in catalog are named and categorised from the
[CNCF landscape](usage.md#cncf-landscape); others appear as *unclassified*.
The home page and **Offerings** start with a description and one offering per
category, drafted from what PBOM found, and **Releases** with a drafted first
release of what the cluster runs now, until you write your own in step 4. To
keep that release, open **Releases**, download it and apply it with
`kubectl apply -n platform-bom -f 1.0.0.yaml`; later releases work the same
way. **Updates** lists newer upstream versions and why they matter.
Latest upstream versions are fetched from GitHub after startup and can take a few
minutes to fill in, longer without a [GitHub token](#7-operate-it).

That is a working install. The next steps are optional and add meaning: what
the platform offers, which tools it is made of, and which versions you support.

## 4. Describe your platform in YAML

The [`pbom` binary](deployment.md#install-the-pbom-binary) writes the files for
you from what the cluster runs. It needs `pbom` 1.0.0 or newer
(`pbom version`). Use the same cluster as in step 2.

`--env` names the environment this cluster is, such as `prod`, `staging` or
`dev` (lowercase letters, digits and dashes). It becomes the environment's name
in `platform.yaml` and on the Environments page; choose the one that describes
this cluster before running:

```sh
pbom init pbom-config --context "$(kubectl config current-context)" --env prod --components
```

```text
pbom-config/
├── platform.yaml        # Platform: name, offerings, environments
├── releases/1.0.0.yaml  # PlatformRelease: every version running today
├── components/*.yaml    # Component drafts, one per unclassified Helm release
└── kustomization.yaml   # applies them as PBOM custom resources
```

Review and edit them before applying; every file is plain YAML:

- **`platform.yaml`**: set the name and review the **offerings** teams get.
  `pbom init` drafts a description and one offering per category of
  recognised components, such as Delivery, Networking, Security or
  Observability, each listing the components that provide it. Comments in the
  file explain every field, with commented-out examples for owners, links,
  guarantees and a planned offering. Rename, split or describe the offerings;
  component names come from the Components page or `pbom catalog`. The
  environment named by `--env` is the cluster PBOM runs in, and it targets
  release 1.0.0.
- **`releases/1.0.0.yaml`**: the baseline. Remove components that are not part
  of the platform, and loosen versions you do not want to pin: `"1.36"` accepts
  any 1.36 patch, `"1.36.2"` only that one.
- **`components/`**: one draft per Helm release the built-in catalog does not
  know. Keep the tools that belong to the platform, set their `category` and,
  for update checks, `upstream.github`; delete the rest. Leave out
  `--components` if you want none.

After adding or deleting files, regenerate the kustomization:

```sh
pbom kustomize pbom-config
```

### Later: cut the next release

When the cluster has been upgraded, snapshot it as a new release, then point
the environment at it by changing `targetRelease` in `platform.yaml`:

```sh
pbom -c pbom-config/platform.yaml release create 1.1.0 --from-env prod \
  --context "$(kubectl config current-context)" --summary "Kubernetes upgrade"
```

This writes `pbom-config/releases/1.1.0.yaml` and updates
`kustomization.yaml`.

## 5. Apply the YAML

Switch the chart to your files, then apply them. The `helm upgrade` is needed
only once, and must come first: it removes the chart's own `Platform`, which
has the same default name as yours.

```sh
helm upgrade pbom oci://ghcr.io/ravibagri5/charts/platform-bom -n platform-bom --reset-then-reuse-values --set crdResources.create=false --set platform.metadata.name=my-platform
kubectl apply -k pbom-config
kubectl -n platform-bom rollout status deploy/pbom-platform-bom
```

Set `platform.metadata.name` to the name in `pbom-config/platform.yaml` (the
`--name` given to `pbom init`, `my-platform` by default); PBOM serves that
`Platform`.

From now on, edit the files and run `kubectl apply -k pbom-config` again.
PBOM picks up the change within seconds, without a restart. Kubernetes
rejects a resource that is not a valid `Platform`, `PlatformRelease` or
`Component`; a resource that is valid but inconsistent is reported in the pod
log and the previous configuration keeps serving. `kubectl get pbom -n
platform-bom` lists all of them.

## 6. Keep it in Git

Commit `pbom-config/` to a Git repository, then point any continuous delivery
tool at that directory so it applies the files whenever they change, for
example an Argo CD Application or a Flux Kustomization with `pbom-config` as
its path. Any tool that can run `kubectl apply -k` on a directory works. The
namespace is set in `kustomization.yaml`. Platform changes then go through
pull requests, and `git log` becomes the platform's changelog.

## 7. Operate it

| Task | Command |
| --- | --- |
| Logs | `kubectl -n platform-bom logs deploy/pbom-platform-bom` |
| Metrics, alerts and dashboard | `--set metrics.enabled=true --set metrics.serviceMonitor.enabled=true` and more; see [Monitoring](../charts/platform-bom/README.md#monitoring) |
| Upgrade PBOM | Helm never upgrades CRDs, so apply them first: `helm show crds oci://ghcr.io/ravibagri5/charts/platform-bom \| kubectl apply --server-side -f -`, then re-run the `helm upgrade` from step 2 or 5. From 0.x, follow the migration in the [changelog](../CHANGELOG.md#100---2026-10-15) |
| Upstream checks without rate limits | `kubectl -n platform-bom create secret generic pbom-github --from-literal=token=<token>`, then `helm upgrade ... --reset-then-reuse-values --set githubTokenSecret.name=pbom-github` |
| Share the UI | Turn on [sign-in](authentication.md) with your identity provider, then enable the [Ingress](../charts/platform-bom/README.md#optional-ingress-and-tls) |
| More clusters | [Multi-cluster setup](multi-cluster.md): one PBOM for dev, QA and prod |
| Uninstall | `kubectl delete -k pbom-config`, then `helm uninstall pbom -n platform-bom`. Helm keeps the CRDs; `kubectl delete crd platforms.pbom.dev platformreleases.pbom.dev components.pbom.dev inventories.pbom.dev` removes them and every PBOM resource in the cluster |

All chart settings are listed in the [chart README](../charts/platform-bom/README.md).

## Optional: clusters PBOM cannot reach

Export an inventory from a machine that can reach the cluster, add it to the
kustomization, and add an `edge` environment that reads it (`environments` is
the last section of `platform.yaml`, so appending works), then apply:

```sh
pbom discover --context edge-cluster --env edge -o yaml > pbom-config/edge.inventory.yaml
pbom kustomize pbom-config
cat >> pbom-config/platform.yaml <<'EOF'
    - name: edge
      inventory: edge
EOF
kubectl apply -k pbom-config
```

The export is an `Inventory` resource named after `--env`; `inventory` refers
to it by that name. Re-export and apply it whenever that cluster changes.

## Reference: what each file is for

| Kind | Answers | Written by | Needed? |
| --- | --- | --- | --- |
| `Platform` | What is the platform, what does it offer, which environments should run which release? | You | One per platform; the chart has a default |
| `Component` | How is a tool recognised, and where are its releases published? | Built-in catalog of common tools, plus yours | Only for tools the catalog does not know |
| `PlatformRelease` | Which component versions and offerings make up platform version X? | You, or `pbom release create` | Only to track drift against a baseline |
| `Inventory` | What was an environment running, and what is the evidence? | `pbom discover -o yaml` | Only for clusters PBOM cannot reach |

All four are Kubernetes custom resources in the `pbom.dev/v1alpha1` API,
installed by the chart; PBOM watches them in its namespace and never writes
them. A display name goes in the `pbom.dev/display-name` annotation. Inventories
of clusters PBOM discovers live are kept in memory, not stored as resources.
Editing an exported `Inventory` does not classify anything; to name
an unclassified tool, write a `Component`. The full schema of each kind is in
[usage.md](usage.md#configuration).

Instead of separate files, the same documents can also be set as the chart's
`platform`, `releases` and `components` values; see the
[chart README](../charts/platform-bom/README.md).

## Troubleshooting

| Symptom | Check |
| --- | --- |
| `Forbidden` during install | Your account cannot create cluster-wide RBAC; ask a cluster administrator. |
| Pod not ready after step 5 | No `Platform` with the configured name exists in the namespace: run `kubectl apply -k pbom-config` and check `platform.metadata.name`. The pod log says what is missing. |
| `ImagePullBackOff` | `kubectl -n platform-bom describe pod` shows the image and pull error; check access to `ghcr.io`. |
| A YAML change does not show up | Check the pod log for `PBOM resources are invalid`. After adding or removing files, run `pbom kustomize pbom-config` before `kubectl apply -k`. |
| Few or no components | PBOM only names tools in its catalog; others appear as unclassified Helm releases or Crossplane packages. Add a Component. |
| Helm releases missing | `rbac.helmSecrets` may be disabled. |
| No upstream versions | The pod needs outbound access to GitHub; add a token if the anonymous rate limit is exhausted. |
| Port-forward fails | `kubectl -n platform-bom get pods`; with another Helm release name, the Service is `<release>-platform-bom`. |
