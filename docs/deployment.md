# Installing and deploying Platform BOM

- [Installation](#installation)
- [Required RBAC](#required-rbac)
- [Running in a container](#running-in-a-container)
- [Running in a cluster](#running-in-a-cluster)

## Installation

### From source

```shell
go install github.com/ravibagri5/platform-bom/cmd/pbom@latest
```

`go install` builds without the web UI, since the UI is compiled separately.
The CLI works fully; `pbom serve` answers the JSON API and explains how to
build the UI. For the complete binary, clone the repository and run
`make ui build` (Go 1.26+ and Node 24+).

### Release binaries and images

Releases are built with GoReleaser. From the first tagged release, archives for
Linux, macOS and Windows on amd64 and arm64, with SBOMs and a cosign-signed
checksum file, are published on the
[releases page](https://github.com/ravibagri5/platform-bom/releases), and
images at `ghcr.io/ravibagri5/platform-bom`. Until then, build the image
locally with `docker build -t ghcr.io/ravibagri5/platform-bom:dev .`.

## Required RBAC

Discovery only uses `get` and `list`. A least-privilege ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: platform-bom-reader
rules:
  - apiGroups: [""]
    resources: [nodes, secrets]   # secrets: Helm release metadata; drop to skip Helm
    verbs: [get, list]
  - apiGroups: [apps]
    resources: [deployments, statefulsets, daemonsets]
    verbs: [get, list]
  - apiGroups: [pkg.crossplane.io]
    resources: [providers, functions, configurations]
    verbs: [get, list]
```

Helm stores release values in Secrets, and Kubernetes RBAC cannot restrict
access by label, so reading Helm metadata requires cluster-wide `list secrets`.
`pbom` requests only Secrets labelled `owner=helm` and reads nothing but the
chart name and versions from them, but the permission is broad. Leave it out if
that is not acceptable; Helm discovery is then skipped with a warning. See
[SECURITY.md](../SECURITY.md) for the full threat model.

## Running in a container

```shell
docker run --rm -p 8080:8080 \
  -v "$PWD:/config:ro" \
  -v "$HOME/.kube/config:/home/nonroot/.kube/config:ro" \
  -e GITHUB_TOKEN \
  ghcr.io/ravibagri5/platform-bom:latest
```

The image runs as a non-root user and expects the platform file at
`/config/pbom.yaml`. Kubeconfigs that use exec plugins such as `kubelogin` or
`aws eks get-token` need those binaries, so for those, run the binary directly
or use exported inventories.

The UI has no authentication of its own. It binds to `127.0.0.1` by default;
put it behind your organisation's authenticating proxy before exposing it.

## Running in a cluster

[deploy/](../deploy) is a kustomize base that runs `pbom serve` in its own
namespace with a read-only ClusterRole. The pod runs as non-root with a
read-only root filesystem and no capabilities, and the namespace enforces the
`restricted` Pod Security Standard.

1. **Describe the platform** in [deploy/platform/pbom.yaml](../deploy/platform/pbom.yaml).
   The cluster `pbom` runs in is an environment with `inCluster: true`:

   ```yaml
   environments:
     - name: prod
       inCluster: true
       targetRelease: "1.0.0"
   ```

2. **Add releases** by copying them into `deploy/releases/` and listing them
   under the `pbom-releases` generator in
   [deploy/kustomization.yaml](../deploy/kustomization.yaml). Both files become
   ConfigMaps whose names carry a content hash, so changing either rolls the
   Deployment.

3. **Optionally add a GitHub token** for upstream release checks:

   ```shell
   kubectl create namespace platform-bom
   kubectl -n platform-bom create secret generic pbom-github --from-literal=token="$GITHUB_TOKEN"
   ```

4. **Apply and open it:**

   ```shell
   kubectl apply -k deploy
   kubectl -n platform-bom port-forward svc/pbom 8080:80
   ```

   Then browse to <http://127.0.0.1:8080>.

### Discovering other clusters from inside one

A single in-cluster `pbom` can discover other clusters through a kubeconfig.
Create a Secret holding a kubeconfig with one context per cluster, uncomment
the `kubeconfig` volume in [deploy/deployment.yaml](../deploy/deployment.yaml),
and reference it from each environment:

```shell
kubectl -n platform-bom create secret generic pbom-kubeconfig --from-file=config=./pbom-kubeconfig
```

```yaml
    - name: staging
      kubeconfig: /kubeconfig/config
      kubeContext: staging
```

The image contains no credential plugins such as `kubelogin` or
`aws eks get-token`, so the kubeconfig must carry a token, typically for a
service account bound to the same read-only ClusterRole in each target cluster.
Where that is not possible, run `pbom discover -o yaml` in a job that can reach
the cluster and use the result as an `inventoryFile`.

### Trying it on kind

Until a release is published, build the image locally and load it into kind:

```shell
docker build -t ghcr.io/ravibagri5/platform-bom:dev .
kind load docker-image ghcr.io/ravibagri5/platform-bom:dev --name <cluster>
(cd deploy && kustomize edit set image ghcr.io/ravibagri5/platform-bom:dev)
kubectl apply -k deploy
```

### Exposing it

The Service is `ClusterIP` on purpose: the UI has no authentication. To share
it, put it behind an Ingress or Gateway that authenticates users, for example
with oauth2-proxy or your identity-aware proxy.
