# Platform BOM Helm chart

See the [getting-started guide](https://github.com/ravibagri5/platform-bom/blob/main/docs/getting-started.md) for installation
and for managing the platform's YAML files with Git.

```sh
helm upgrade --install pbom oci://ghcr.io/ravibagri5/charts/platform-bom \
  --namespace platform-bom --create-namespace --wait --timeout 3m
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

Helm installs the latest chart by default. Add `--version X.Y.Z` to pin a
specific release.

The defaults discover the current cluster without custom YAML or a declared
release. No CRDs are installed. Resource names include the Helm release name;
cluster-scoped RBAC also includes the namespace. ConfigMap changes roll the
pod using a configuration checksum.

| Value | Default | Purpose |
| --- | --- | --- |
| `image.repository` | `ghcr.io/ravibagri5/platform-bom` | Container repository |
| `image.tag` | Chart `appVersion` | Override the versioned image |
| `image.pullPolicy` | `IfNotPresent` | Container image pull policy |
| `imagePullSecrets` | `[]` | Existing image-pull Secret references |
| `rbac.create` | `true` | Create read-only ClusterRole and binding |
| `rbac.helmSecrets` | `true` | Allow cluster-wide Secret reading for Helm metadata |
| `serviceAccount.create` | `true` | Create a service account |
| `serviceAccount.name` | Generated name | Existing account required when creation is disabled |
| `service.port` | `80` | Private ClusterIP Service port |
| `ingress.enabled` | `false` | Create an optional Ingress |
| `ingress.className` | Empty | Ingress controller class |
| `ingress.annotations` | `{}` | cert-manager, authentication and controller annotations |
| `ingress.hosts` | `[]` | Hostnames and optional paths; at least one hostname required when enabled |
| `ingress.tls` | `[]` | TLS Secret names and hostnames |
| `refreshInterval` | `10m` | Discovery refresh interval |
| `githubTokenSecret.name` | Empty | Existing Secret containing an optional GitHub token |
| `githubTokenSecret.key` | `token` | Key in that Secret |
| `kubeconfigSecret.name` | Empty | Existing Secret with a kubeconfig for other clusters, mounted read-only at `/kubeconfig`; see [multi-cluster](https://github.com/ravibagri5/platform-bom/blob/main/docs/multi-cluster.md) |
| `extraInitContainers` | `[]` | Init containers that run before PBOM starts |
| `extraVolumes` | `[]` | Additional pod volumes; `platform`, `releases`, `components`, `cache` and `kubeconfig` are reserved |
| `extraVolumeMounts` | `[]` | Additional mounts in the PBOM container |
| `configMaps.create` | `true` | Render configuration from `platform`, `releases` and `components`; `false` mounts ConfigMaps you apply yourself |
| `configMaps.platform` / `.releases` / `.components` | `pbom-platform` / `pbom-releases` / `pbom-components` | Names of your ConfigMaps when `configMaps.create=false`; releases and components may be absent |
| `platform` | Single in-cluster environment | PBOM `Platform` document, not a Kubernetes CRD; releases and components are read from the mounted ConfigMaps |
| `releases` | `{}` | Map of file names to `PlatformRelease` documents or raw file content (`--set-file`) |
| `components` | `{}` | Map of file names to `Component` documents or raw file content; override or extend the built-in catalog |
| `resources` | 50m CPU / 128Mi requested, 512Mi memory limit | Pod resources |

The UI has no authentication; keep the Service private or use an
authenticating proxy. If your administrator pre-provisions discovery access,
set `rbac.create=false`, `serviceAccount.create=false` and
`serviceAccount.name` to the existing account. Disabling RBAC creation alone
does not grant discovery permissions.

File names in `releases` and `components` must end in `.yaml` or `.yml`. To
keep each document in its own file instead, apply them as ConfigMaps and set
`configMaps.create=false`; see
[getting started](https://github.com/ravibagri5/platform-bom/blob/main/docs/getting-started.md#4-describe-your-platform-in-yaml).
Either way, PBOM reloads changed configuration without a restart.

`values.schema.json` validates values on every install, upgrade, `template` and
`lint`: unknown keys (typos such as `ingres`), wrong types, invalid ingress
paths, durations, environment names and file names fail before anything is
applied. The `platform` document is checked loosely here and in full by `pbom`
at startup.

## Optional Ingress and TLS

Ingress is disabled by default. To enable it, put the following in your Helm
values file and pass that file with `-f` on install or upgrade. Replace the
hostname, ingress class and issuer with your cluster's settings:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: pbom.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: pbom-tls
      hosts:
        - pbom.example.com
```

Your cluster must already have an ingress controller, cert-manager and the
named `ClusterIssuer`. Point DNS at the controller and ensure the issuer's
certificate challenge can complete. cert-manager's ingress shim creates the
Certificate and TLS Secret in PBOM's namespace; the chart does not install
cert-manager or an issuer. For a namespaced `Issuer`, use
`cert-manager.io/issuer` instead of `cert-manager.io/cluster-issuer`. For an
existing TLS Secret, omit the cert-manager annotation and keep `ingress.tls`.

Annotations are passed through unchanged, so controller-specific settings
and authentication proxy annotations can be added. Paths default to `/` with
`Prefix` when omitted. PBOM serves at the root path; subpath deployments need
appropriate controller rewrite rules.

**TLS is not authentication.** PBOM has no built-in authentication. Use an
internal ingress controller or an authenticating proxy before exposing the UI.

## Extra volumes and init containers

`extraVolumes`, `extraVolumeMounts` and `extraInitContainers` extend the pod
without forking the chart, for example to mount a CA bundle or exported
inventories, or to add a kubeconfig credential plugin. The PBOM image is
distroless and has no plugins, so an init container copies one into a shared
`emptyDir`:

```yaml
extraInitContainers:
  - name: kubelogin
    image: registry.example.com/tools/kubelogin:1.0.0   # any image with the binary
    command: [cp, /usr/local/bin/kubelogin, /plugins/kubelogin]
    volumeMounts:
      - name: plugins
        mountPath: /plugins
extraVolumes:
  - name: plugins
    emptyDir: {}
extraVolumeMounts:
  - name: plugins
    mountPath: /plugins
    readOnly: true
```

The kubeconfig then calls the plugin by absolute path
(`exec.command: /plugins/kubelogin`) and passes its settings in `exec.env`.
The plugin must be a statically linked binary; tools that need a shell or an
interpreter, such as the AWS CLI, do not run in the image. Init containers run
with the pod's non-root security context.

## Release publishing

The tag-triggered release workflow packages a chart with its `version` and
`appVersion` set to the release tag without `v`, attaches the archive to the
GitHub release, and pushes it to
`oci://ghcr.io/ravibagri5/charts/platform-bom` after image publication succeeds.
The first GHCR chart package must be made public in its package settings for
anonymous installation. Do not advertise an OCI version until its chart and
image are available.
