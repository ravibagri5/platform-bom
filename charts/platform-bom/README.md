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
release. The chart installs four namespaced CRDs in the `pbom.dev` group
(`Platform`, `PlatformRelease`, `Component`, `Inventory`) and PBOM watches those resources
in its own namespace, applying changes without a restart. Resource names
include the Helm release name; cluster-scoped RBAC also includes the namespace.

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
| `serviceAccount.annotations` | `{}` | Service account annotations, such as a cloud [workload identity](https://github.com/ravibagri5/platform-bom/blob/main/docs/multi-cluster.md#managed-clusters-aks-eks-and-gke) |
| `podLabels` | `{}` | Extra pod labels, such as `azure.workload.identity/use: "true"` |
| `service.port` | `80` | Private ClusterIP Service port |
| `ingress.enabled` | `false` | Create an optional Ingress |
| `ingress.className` | Empty | Ingress controller class |
| `ingress.annotations` | `{}` | cert-manager, authentication and controller annotations |
| `ingress.hosts` | `[]` | Hostnames and optional paths; at least one hostname required when enabled |
| `ingress.tls` | `[]` | TLS Secret names and hostnames |
| `refreshInterval` | `10m` | Discovery refresh interval |
| `githubTokenSecret.name` | Empty | Existing Secret containing an optional GitHub token |
| `githubTokenSecret.key` | `token` | Key in that Secret |
| `landscapeURL` | CNCF `landscape.yml` on GitHub | Names and categorises Helm releases of CNCF projects outside the catalog; on download failure the built-in snapshot is used; `""` uses only the snapshot |
| `logFormat` / `logLevel` | `json` / `info` | Log format (`text` or `json`) and level (`debug`, `info`, `warn`, `error`) |
| `metrics.enabled` / `metrics.port` | `false` / `9090` | Prometheus metrics at `/metrics` on a separate port and Service port named `metrics`, outside sign-in |
| `metrics.serviceMonitor.enabled` / `.interval` / `.labels` | `false` / `1m` / `{}` | ServiceMonitor for the Prometheus Operator; set `labels` to what your Prometheus selects |
| `metrics.prometheusRule.enabled` / `.labels` | `false` / `{}` | Alert rules; see [Monitoring](#monitoring) |
| `metrics.grafanaDashboard.enabled` / `.labels` / `.annotations` | `false` / `grafana_dashboard: "1"` / `{}` | ConfigMap with the Grafana dashboard for the Grafana sidecar |
| `auth.mode` | `none` | `none`, `header` (authenticating proxy) or `oidc` (OpenID Connect sign-in); see [authentication](https://github.com/ravibagri5/platform-bom/blob/main/docs/authentication.md) |
| `auth.header.name` | `X-Forwarded-User` | Header with the user name, in `header` mode |
| `auth.header.trustedProxies` | `[]` | CIDRs the proxy connects from; other clients are refused |
| `auth.oidc.issuer` / `.clientId` / `.redirectURL` | Empty | Required in `oidc` mode; `redirectURL` ends in `/auth/callback` |
| `auth.oidc.clientSecret.name` / `.key` | Empty / `client-secret` | Existing Secret with the client secret; empty for a public client |
| `auth.oidc.scopes` | `[openid, profile, email]` | Scopes to request |
| `auth.oidc.usernameClaim` | Empty | Claim shown as the user's name |
| `auth.oidc.groupsClaim` / `.allowedGroups` | `groups` / `[]` | Only members of these groups may sign in |
| `auth.sessionSecret.name` / `.key` | Empty / `session-secret` | Existing Secret with 32+ random bytes for session cookies; without it, users sign in again after a restart |
| `auth.sessionDuration` | `12h` | How long a sign-in lasts |
| `kubeconfigSecret.name` | Empty | Existing Secret with a kubeconfig for other clusters, mounted read-only at `/kubeconfig`; see [multi-cluster](https://github.com/ravibagri5/platform-bom/blob/main/docs/multi-cluster.md) |
| `extraInitContainers` | `[]` | Init containers that run before PBOM starts |
| `extraVolumes` | `[]` | Additional pod volumes; `platform`, `releases`, `components`, `cache` and `kubeconfig` are reserved |
| `extraVolumeMounts` | `[]` | Additional mounts in the PBOM container |
| `crdResources.create` | `true` | Render `platform`, `releases` and `components` as custom resources; `false` when you apply them yourself (`kubectl apply -k`, any CD tool) |
| `platform` | Single in-cluster environment | The `Platform` resource; its `metadata.name` is the one PBOM serves. Put a display name in `metadata.displayName`; the chart turns it into the `pbom.dev/display-name` annotation |
| `releases` | `{}` | Map of file names to `PlatformRelease` resources or raw file content (`--set-file`) |
| `components` | `{}` | Map of file names to `Component` resources or raw file content; override or extend the built-in catalog |
| `resources` | 50m CPU / 128Mi requested, 512Mi memory limit | Pod resources |

By default the UI has no sign-in; keep the Service private, or set
`auth.mode` to `oidc` or `header` as described in
[authentication](https://github.com/ravibagri5/platform-bom/blob/main/docs/authentication.md).
If your administrator pre-provisions discovery access,
set `rbac.create=false`, `serviceAccount.create=false` and
`serviceAccount.name` to the existing account. Disabling RBAC creation alone
does not grant discovery permissions.

File names in `releases` and `components` must end in `.yaml` or `.yml`. To
keep each resource in its own file instead, apply them with `kubectl apply -k`
and set `crdResources.create=false`; see
[getting started](https://github.com/ravibagri5/platform-bom/blob/main/docs/getting-started.md#4-describe-your-platform-in-yaml).
Either way, PBOM picks up changes within seconds, without a restart. Change
resources the chart renders through its values only: after a `kubectl edit` or
`kubectl patch`, `helm upgrade` stops with a field conflict. An
exported inventory of a cluster PBOM cannot reach is an `Inventory` resource,
referenced from an environment with `inventory: <name>`.

Helm installs CRDs on first install only and never upgrades or deletes them.
After a chart upgrade that changes them, apply the new CRDs with
`helm show crds oci://ghcr.io/ravibagri5/charts/platform-bom | kubectl apply -f -`.

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

**TLS is not authentication.** Without `auth.mode`, anyone who reaches the
Ingress can open the UI. Use `oidc` sign-in, an authenticating proxy with
`header` mode, or an internal ingress controller.

## Monitoring

With `metrics.enabled=true`, PBOM serves Prometheus metrics on port `9090`; the metrics are
listed in [usage.md](https://github.com/ravibagri5/platform-bom/blob/main/docs/usage.md#monitoring-and-logging).
With [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack),
turn on metrics, scraping, alerts and the dashboard, using the label your Prometheus
selects (`release: <kube-prometheus-stack release name>` by default):

```sh
helm upgrade pbom oci://ghcr.io/ravibagri5/charts/platform-bom -n platform-bom --reset-then-reuse-values \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true --set metrics.serviceMonitor.labels.release=kube-prometheus-stack \
  --set metrics.prometheusRule.enabled=true --set metrics.prometheusRule.labels.release=kube-prometheus-stack \
  --set metrics.grafanaDashboard.enabled=true
```

The Grafana sidecar loads ConfigMaps labelled `grafana_dashboard: "1"` from
every namespace it watches. Without the sidecar, import
[dashboards/platform-bom.json](https://github.com/ravibagri5/platform-bom/blob/main/charts/platform-bom/dashboards/platform-bom.json)
in Grafana. Without the Prometheus Operator, scrape the `metrics` port of the
Service directly.

| Alert | Fires when | Severity |
| --- | --- | --- |
| `PlatformBOMDown` | Prometheus cannot scrape PBOM for 10 minutes | warning |
| `PlatformBOMEnvironmentOffTarget` | An environment has not run its target release for an hour | warning |
| `PlatformBOMUnsupportedVersion` | A running version has been out of upstream support for an hour | warning |
| `PlatformBOMDiscoveryFailing` | Discovery of an environment keeps failing for 30 minutes | warning |
| `PlatformBOMConfigurationInvalid` | A rejected configuration change is still pending after 10 minutes | warning |
| `PlatformBOMUpstreamRateLimited` | The GitHub API limit was reached in the last hour | info |

Logs are JSON on stderr, ready for Loki, Elasticsearch or a cloud logging
agent; `--set logFormat=text` makes them human-readable.

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
