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

[Reference: what each file is for](#reference-what-each-file-is-for) and
[Troubleshooting](#troubleshooting) are at the end.

## 1. Check prerequisites

- Helm 3.8+ or Helm 4, and `kubectl`, pointing at the cluster to inventory.
- Permission to create a namespace, a ClusterRole and a ClusterRoleBinding.
- The cluster can pull images from `ghcr.io`.

```sh
kubectl config current-context    # this is the cluster PBOM will discover
kubectl auth can-i create clusterroles.rbac.authorization.k8s.io
```

## 2. Install with Helm

From a clone of this repository:

```sh
git clone https://github.com/ravibagri5/platform-bom.git && cd platform-bom
helm upgrade --install pbom ./charts/platform-bom \
  --namespace platform-bom --create-namespace --wait
```

Once a chart release is published, you can install it without cloning:
`helm upgrade --install pbom oci://ghcr.io/ravibagri5/charts/platform-bom --version <version> --namespace platform-bom --create-namespace --wait`.

PBOM gets read-only access to nodes, workloads, Crossplane packages and Helm
release Secrets. Add `--set rbac.helmSecrets=false` if it must not read
Secrets; it then skips Helm releases.

## 3. Open the inventory

```sh
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Open <http://127.0.0.1:8080>. **Components** lists what PBOM recognised, with
versions, evidence and the latest upstream release. Helm releases and
Crossplane packages it does not recognise appear as *unclassified*.

That is a working install. The next steps are optional and add meaning: what
the platform offers, which tools it is made of, and which versions you support.

## 4. Describe your platform in YAML

The [`pbom` binary](deployment.md#install-the-pbom-binary) writes the files for
you from what the cluster runs. It needs `pbom` 0.3.0 or newer
(`pbom version`). Use the same cluster as in step 2:

```sh
pbom init pbom-config --context "$(kubectl config current-context)" --env prod --components
```

```text
pbom-config/
├── platform.yaml        # Platform: name, offerings, environments
├── releases/1.0.0.yaml  # PlatformRelease: every version running today
├── components/*.yaml    # Component drafts, one per unclassified Helm release
└── kustomization.yaml   # turns the files into ConfigMaps
```

Review and edit them before applying; every file is plain YAML:

- **`platform.yaml`**: set the name and list the **offerings** teams get, each
  linked to component names from the Components page or `pbom catalog`. The
  `prod` environment is the cluster PBOM runs in, and it targets release 1.0.0.
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

### Optional: clusters PBOM cannot reach

Export an inventory from a machine that can reach the cluster, add it to the
kustomization, and add an `edge` environment that reads it (`environments` is
the last section of `platform.yaml`, so appending works):

```sh
pbom discover --context edge-cluster -o yaml > pbom-config/edge.inventory.yaml
pbom kustomize pbom-config
cat >> pbom-config/platform.yaml <<'EOF'
    - name: edge
      inventoryFile: edge.inventory.yaml
EOF
```

## 5. Apply the YAML

Apply the files, then switch the chart to them. The `helm upgrade` is needed
only once:

```sh
kubectl apply -k pbom-config
helm upgrade pbom ./charts/platform-bom -n platform-bom --reuse-values --set configMaps.create=false --wait
```

From now on, edit the files and run `kubectl apply -k pbom-config` again.
PBOM reloads changed configuration within one to two minutes, without a
restart.
A file with a mistake is reported in the pod log and the previous
configuration keeps serving.

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
| Upgrade PBOM | Re-run the `helm upgrade` from step 2 or 5 with a newer chart |
| Upstream checks without rate limits | `kubectl -n platform-bom create secret generic pbom-github --from-literal=token=<token>`, then `helm upgrade ... --reuse-values --set githubTokenSecret.name=pbom-github` |
| Share the UI | Enable the [Ingress](../charts/platform-bom/README.md#optional-ingress-and-tls) behind an authenticating proxy; PBOM has no login of its own |
| More clusters | [Multi-cluster discovery](deployment.md#discovering-other-clusters-from-inside-one) or exported inventories |
| Uninstall | `helm uninstall pbom -n platform-bom` and `kubectl delete -k pbom-config` |

All chart settings are listed in the [chart README](../charts/platform-bom/README.md).

## Reference: what each file is for

| Kind | Answers | Written by | Needed? |
| --- | --- | --- | --- |
| `Platform` | What is the platform, what does it offer, which environments should run which release? | You | One per platform; the chart has a default |
| `Component` | How is a tool recognised, and where are its releases published? | Built-in catalog of common tools, plus yours | Only for tools the catalog does not know |
| `PlatformRelease` | Which component versions and offerings make up platform version X? | You, or `pbom release create` | Only to track drift against a baseline |
| `Inventory` | What was an environment running, and what is the evidence? | `pbom discover -o yaml` | Only for clusters PBOM cannot reach |

None of these are Kubernetes resources or CRDs: they are files PBOM reads from
ConfigMaps. Editing an exported `Inventory` does not classify anything; to name
an unclassified tool, write a `Component`. The full schema of each kind is in
[usage.md](usage.md#configuration).

Instead of separate files, the same documents can also be set as the chart's
`platform`, `releases` and `components` values; see the
[chart README](../charts/platform-bom/README.md).

## Troubleshooting

| Symptom | Check |
| --- | --- |
| `Forbidden` during install | Your account cannot create cluster-wide RBAC; ask a cluster administrator. |
| Pod stuck in `ContainerCreating` after step 5 | The `pbom-platform` ConfigMap is missing: run `kubectl apply -k pbom-config`. |
| `ImagePullBackOff` | `kubectl -n platform-bom describe pod` shows the image and pull error; check access to `ghcr.io`. |
| A YAML change does not show up | Check the pod log for `configuration changed but is invalid`. After adding or removing files, run `pbom kustomize pbom-config` before `kubectl apply -k`. ConfigMap updates take up to two minutes to reach the pod. |
| Few or no components | PBOM only names tools in its catalog; others appear as unclassified Helm releases or Crossplane packages. Add a Component. |
| Helm releases missing | `rbac.helmSecrets` may be disabled. |
| No upstream versions | The pod needs outbound access to GitHub; add a token if the anonymous rate limit is exhausted. |
| Port-forward fails | `kubectl -n platform-bom get pods`; with another Helm release name, the Service is `<release>-platform-bom`. |
