# Security Policy

## Supported versions

Security fixes land on the latest minor release. Older releases are not
patched; please upgrade.

| Version | Supported |
| --- | --- |
| Latest minor | Yes |
| Anything older | No |

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report privately using
[GitHub's private vulnerability reporting](https://github.com/ravibagri5/platform-bom/security/advisories/new),
which notifies the maintainers listed in [MAINTAINERS.md](MAINTAINERS.md)
without disclosing anything publicly.

Please include:

- A description of the issue and its impact.
- Steps to reproduce, ideally with a minimal platform file.
- The version of `pbom` (`pbom version`) and the Kubernetes distribution you
  tested against.
- Whether you believe the issue is being actively exploited.

### What to expect

| Stage | Target |
| --- | --- |
| Acknowledgement | 3 business days |
| Initial assessment | 10 business days |
| Fix or mitigation plan | 30 days for high and critical severity |

We will credit you in the advisory unless you ask us not to. Please give us a
reasonable window to ship a fix before disclosing publicly.

## Threat model

Knowing what `pbom` does and does not do will help you judge whether something
is a vulnerability.

### Discovery is read-only

`pbom` only performs `get` and `list` requests against a cluster. There is no
create, update, patch or delete call in the codebase. Any way to make `pbom`
change cluster state is a high severity bug and we want to hear about it.

### `pbom` acts as you

Discovery uses the kubeconfig and context you give it, and therefore has
exactly those permissions. It does not escalate privilege. For a stronger
guarantee, run it with a least-privilege identity;
[docs/deployment.md](docs/deployment.md#required-rbac) has a read-only ClusterRole.

### Helm release Secrets

Helm stores each release, including its values, in a Secret. To read chart
names and versions, `pbom` lists Secrets labelled `owner=helm,status=deployed`
and decodes them. It keeps only the release name, chart name, chart version and
app version, and never logs, returns or stores anything else from them.

Kubernetes RBAC cannot restrict access by label, so this needs cluster-wide
`list` on Secrets. If that is not acceptable, leave `secrets` out of the role:
Helm discovery then fails with a warning and everything else still works.

A path through which Secret data, or Helm values, reaches `pbom`'s output, its
logs, the API or an exported inventory is a vulnerability. Please report it.

### Access to the web UI and API

By default (`--auth none`), `pbom serve` has no authentication. Anyone who can
reach it can read your platform definition, inventories and release notes. It
exposes no write operations and no cluster credentials. It binds to
`127.0.0.1` by default.

Before exposing it, turn on [authentication](docs/authentication.md):

- `--auth oidc` signs users in with an OpenID Connect provider using the
  authorization code flow with PKCE, state and nonce checks, and verified ID
  tokens. Sessions are AES-GCM encrypted, `HttpOnly`, `SameSite=Lax` cookies,
  `Secure` when the redirect URL is HTTPS.
- `--auth header` trusts a user header from an authenticating proxy. Restrict
  it with `--auth-trusted-proxies` or a network policy, because any client
  that reaches `pbom` directly can set the header.

Every signed-in user can see everything; per-environment authorisation is
planned in [#40](https://github.com/ravibagri5/platform-bom/issues/40).
Reports that the UI is readable with `--auth none` will be closed with a
pointer to this section. A way to bypass sign-in in `oidc` or `header` mode is
a vulnerability.

### Untrusted content

Upstream release notes come from GitHub and are written by third parties.
Resource names and labels come from your clusters. The UI renders release notes
and your README as Markdown without raw HTML, serves a restrictive
Content-Security-Policy, and opens external links with `noopener`. A way to run
script in the UI through any of these inputs is a vulnerability.

### Files written

`pbom` writes only:

- release files, when you run `pbom release create`, and never over an
  existing one unless you pass `--force`
- a cache of upstream release data in your user cache directory

Release names are validated so that they cannot escape the releases directory.

## Dependencies

Dependencies are updated by Dependabot, and every pull request is scanned with
`govulncheck` and CodeQL in CI. If you spot a vulnerable dependency we have
missed, please open a regular issue; it is not sensitive.
