# Platform BOM Helm chart

See the [getting-started runbook](../../docs/getting-started.md) for installation
and the [optional GitOps guide](../../docs/gitops.md) for Argo CD.

```sh
helm upgrade --install pbom ./charts/platform-bom \
  --namespace platform-bom --create-namespace --wait --timeout 3m
kubectl -n platform-bom port-forward svc/pbom-platform-bom 8080:80
```

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
| `platform` | Single in-cluster environment | PBOM `Platform` document, not a Kubernetes CRD; `componentsDir` is set to the mounted `components` |
| `releases` | `{}` | Map of file names to `PlatformRelease` documents or raw file content (`--set-file`) |
| `components` | `{}` | Map of file names to `Component` documents or raw file content; override or extend the built-in catalog |
| `resources` | 50m CPU / 128Mi requested, 512Mi memory limit | Pod resources |

The UI has no authentication; keep the Service private or use an
authenticating proxy. If your administrator pre-provisions discovery access,
set `rbac.create=false`, `serviceAccount.create=false` and
`serviceAccount.name` to the existing account. Disabling RBAC creation alone
does not grant discovery permissions.

File names in `releases` and `components` must end in `.yaml` or `.yml`. For
what each document kind means and what to configure after installing, see
[next steps](../../docs/getting-started.md#next-steps-from-inventory-to-a-managed-platform).

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

## Release publishing

The tag-triggered release workflow packages a chart with its `version` and
`appVersion` set to the release tag without `v`, attaches the archive to the
GitHub release, and pushes it to
`oci://ghcr.io/ravibagri5/charts/platform-bom` after image publication succeeds.
The first GHCR chart package must be made public in its package settings for
anonymous installation. Do not advertise an OCI version until its chart and
image are available. The source chart currently uses image `0.1.0`.
