# Installing and deploying Platform BOM

For the simplest real-cluster installation, use the
[Helm getting-started runbook](getting-started.md). No user-written platform
YAML is needed to see the first inventory. This page covers alternative
distribution formats and advanced configuration.

| Format | Best for | Workstation prerequisites |
| --- | --- | --- |
| Helm chart | Install the complete UI and discovery service in your cluster | Helm, `kubectl`, cluster access; Git only for an unpublished chart checkout |
| Release binary | Run the CLI or UI against clusters from your workstation | OS/CPU-matching archive, kubeconfig and credential plugins; no Go/Node.js |
| Container image | Run the complete service without compiling | Docker, config and usable cluster credentials; no Go/Node.js |
| Source build | Develop or customize PBOM | Git, Go 1.26+, Node.js 24+, npm, Make |

The Helm chart deploys the same published container image. Helm, the image
and the binary are packaging options for the same application, not different
products. Only source builds need the language toolchains.

- [Installation](#installation)
- [Required RBAC](#required-rbac)
- [Running in a container](#running-in-a-container)
- [Running in a cluster](#running-in-a-cluster)

## Installation

### Install the `pbom` binary

The binary is the CLI and the web UI in one file; there is no installer and
nothing else to download. You need it to discover clusters from your
workstation, cut releases with `pbom release create`, or check drift in CI.
Running PBOM inside a cluster with the Helm chart does not need it.

**macOS and Linux.** Pick a version from the
[releases page](https://github.com/ravibagri5/platform-bom/releases), without
the leading `v`, then download, check and install it:

```sh
VERSION=0.2.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')                 # darwin or linux
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac
ARCHIVE="platform-bom_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/ravibagri5/platform-bom/releases/download/v$VERSION"

curl -fsSLO "$BASE/$ARCHIVE"
curl -fsSLO "$BASE/checksums.txt"
grep " $ARCHIVE\$" checksums.txt | shasum -a 256 -c -       # must print: OK

tar -xzf "$ARCHIVE" pbom
sudo install -m 0755 pbom /usr/local/bin/pbom                # or any directory on your PATH
pbom version
```

Without `sudo`, install into a directory you own, such as
`mkdir -p ~/.local/bin && install -m 0755 pbom ~/.local/bin/`, and make sure it
is on your `PATH`. On macOS, an archive downloaded with a browser instead of
`curl` is quarantined; run `xattr -d com.apple.quarantine pbom` once before
starting it.

**Windows (amd64).** Download `platform-bom_<version>_windows_amd64.zip` and
`checksums.txt` from the releases page, compare
`Get-FileHash .\platform-bom_<version>_windows_amd64.zip` with the line in
`checksums.txt`, extract `pbom.exe` into a folder on your `PATH`, and run
`pbom version` in a new terminal.

**Verify the signature (optional).** `checksums.txt` is signed in the release
workflow with [cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
keyless signing. To prove the checksums came from this repository's release
workflow, download `checksums.txt.sig` and `checksums.txt.pem` next to it and
run:

```sh
cosign verify-blob checksums.txt \
  --signature checksums.txt.sig --certificate checksums.txt.pem \
  --certificate-identity "https://github.com/ravibagri5/platform-bom/.github/workflows/release.yaml@refs/tags/v$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

**Upgrade or remove.** Repeat the steps with a newer `VERSION` to upgrade.
Remove the binary to uninstall; the only other file it writes is the upstream
release cache in `~/Library/Caches/pbom` on macOS or `~/.cache/pbom` on Linux.

### Before you start using the binary

- **Cluster access is your kubeconfig.** `pbom` uses the same contexts and
  credentials as `kubectl`, including plugins such as `kubelogin` or
  `aws eks get-token`, which must be on your `PATH`. It only reads: see
  [Required RBAC](#required-rbac).
- **No cluster needed to try it.** From a clone of this repository,
  `pbom serve -c examples/acme/pbom.yaml` opens the example platform at
  <http://127.0.0.1:8080>.
- **First look at a real cluster:** `kubectl config get-contexts`, then
  `pbom discover --context <context>`. No platform file is needed for this.
- **A platform file comes next.** For more than one-off discovery, describe
  your environments in a `pbom.yaml`; see
  [describing your platform](usage.md#describing-your-platform). Commands read
  `./pbom.yaml` by default, or the file given with `-c` or `$PBOM_CONFIG`.
- **Upstream checks call GitHub.** Export `GITHUB_TOKEN` to avoid the anonymous
  rate limit, or pass `--no-upstream` to work offline.
- **The UI binds to `127.0.0.1`.** `pbom serve` has no authentication; keep it
  local, or put it behind an authenticating proxy if you change `--addr`.

### From source

```shell
go install github.com/ravibagri5/platform-bom/cmd/pbom@latest
```

`go install` builds without the web UI, since the UI is compiled separately.
The CLI works fully; `pbom serve` answers the JSON API and explains how to
build the UI. For the complete binary, clone the repository and run
`make ui build` (Go 1.26+ and Node 24+).

### Container images

Each release also publishes multi-arch images at
`ghcr.io/ravibagri5/platform-bom`: `:<version>` for every release, `:latest`
for the newest release and `:rc` for the newest release candidate. Archives for
every platform come with SBOMs (`*.sbom.json`) on the releases page.

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

1. **Describe the platform** in [deploy/pbom.yaml](../deploy/pbom.yaml).
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

The base uses the published image, so it works on kind as is:

```shell
kind create cluster --name pbom
kubectl apply -k deploy
```

For three independent, single-node local environments without demo add-ons:

```shell
make kind-clusters
kubectl --context kind-pbom-dev get nodes
kubectl --context kind-pbom-qa get nodes
kubectl --context kind-pbom-prod get nodes
```

Re-running `make kind-clusters` keeps existing clusters. To remove them, run
`kind delete cluster --name pbom-dev` (and likewise for `pbom-qa` and
`pbom-prod`). These are empty Kubernetes clusters; no platform components are
installed by this command.

To populate them with low-resource, version-drift fixtures for PBOM discovery,
see the [three-cluster simulation](../examples/kind/simulation/README.md).

To test local changes, build the image and load it into kind instead:

```shell
docker build -t ghcr.io/ravibagri5/platform-bom:dev .
kind load docker-image ghcr.io/ravibagri5/platform-bom:dev --name pbom
(cd deploy && kustomize edit set image ghcr.io/ravibagri5/platform-bom:dev)
kubectl apply -k deploy
```

### Exposing it

The Service is `ClusterIP` on purpose: the UI has no authentication. To share
it, put it behind an Ingress or Gateway that authenticates users, for example
with oauth2-proxy or your identity-aware proxy.
