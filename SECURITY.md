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
guarantee, run it with a least-privilege identity; the
[README](README.md#required-rbac) has a read-only ClusterRole.

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

### The web UI and API are unauthenticated

`pbom serve` has no authentication or authorisation of its own. Anyone who can
reach it can read your platform definition, inventories and release notes. It
exposes no write operations and no cluster credentials.

It binds to `127.0.0.1` by default. If you bind it to another address, put it
behind a proxy that authenticates users. Reports that the UI is unauthenticated
will be closed with a pointer to this section.

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
