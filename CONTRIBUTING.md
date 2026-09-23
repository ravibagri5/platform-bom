# Contributing

Thanks for wanting to help. Platform BOM aims to be the simplest way for a
platform team to see, version and explain the platform they run, and that only
happens with contributions from people running real platforms.

## Table of contents

- [Code of Conduct](#code-of-conduct)
- [Ways to contribute](#ways-to-contribute)
- [Where to say what](#where-to-say-what)
- [Development setup](#development-setup)
- [Adding a component to the catalog](#adding-a-component-to-the-catalog)
- [Code style](#code-style)
- [Testing](#testing)
- [Commit messages and sign-off](#commit-messages-and-sign-off)
- [Pull requests](#pull-requests)
- [Releases](#releases)

## Code of Conduct

This project follows the [CNCF Code of Conduct](CODE_OF_CONDUCT.md). By
participating you are expected to uphold it.

## Ways to contribute

- **Teach the catalog a new component.** One YAML file makes `pbom` recognise a
  tool for everyone. This is the most valuable and the easiest contribution;
  see [Adding a component to the catalog](#adding-a-component-to-the-catalog).
- **Report what `pbom` gets wrong on your platform.** A wrong version, a
  component filed under the wrong capability, a Crossplane package in the wrong
  place in the tree. Platforms vary enormously and these reports are genuinely
  useful.
- **Add an evidence source.** Argo CD, Flux and cloud provider APIs are on the
  [roadmap](ROADMAP.md).
- **Improve the UI or the docs.**

Before starting anything large, open an issue so we can agree on the shape of
it first. Nobody enjoys having a pull request turned down after a weekend of
work.

## Where to say what

| You have | Go to |
| --- | --- |
| A question about how something works | [Discussions → Q&A](https://github.com/ravibagri5/platform-bom/discussions/categories/q-a) |
| An idea that is not fully formed | [Discussions → Ideas](https://github.com/ravibagri5/platform-bom/discussions/categories/ideas) |
| A platform page, workflow or screenshot to share | [Discussions → Show and tell](https://github.com/ravibagri5/platform-bom/discussions/categories/show-and-tell) |
| Something reproducibly broken | [Bug report](https://github.com/ravibagri5/platform-bom/issues/new?template=bug_report.yml) |
| A component `pbom` should recognise | [Component request](https://github.com/ravibagri5/platform-bom/issues/new?template=component_request.yml), or better, a pull request |
| A specific, scoped capability | [Feature request](https://github.com/ravibagri5/platform-bom/issues/new?template=feature_request.yml) |
| A security vulnerability | [Private report](https://github.com/ravibagri5/platform-bom/security/advisories/new), never a public issue |

How Discussions, labels, triage and the project board fit together is described
in [docs/community.md](docs/community.md).

## Development setup

You need Go 1.26 or newer and Node 24 or newer. A cluster is optional: the
example platform in [examples/acme](examples/acme) uses exported inventories.

```shell
make ui build   # build the web UI and the binary into ./bin
make test       # run the unit tests
make lint       # run golangci-lint
make check      # everything CI runs: verify, vet, lint, test
make run        # serve the example platform on http://127.0.0.1:8080
```

For UI work, run the Go server and the Vite dev server side by side. Vite
proxies `/api` to the Go server and reloads on save:

```shell
./bin/pbom serve -c examples/acme/pbom.yaml   # terminal 1
make dev-ui                                   # terminal 2, http://127.0.0.1:5173
```

For discovery work, `make kind-demo` creates a kind cluster with Argo CD,
cert-manager and Crossplane installed.

## Adding a component to the catalog

Builtin definitions live in
[internal/catalog/components](internal/catalog/components), one file per
capability. Add a document to the right file:

```yaml
---
apiVersion: pbom.dev/v1alpha1
kind: Component
metadata:
  name: external-dns
spec:
  displayName: ExternalDNS
  category: networking
  description: Synchronise Services and Ingresses with DNS providers.
  homepage: https://kubernetes-sigs.github.io/external-dns
  discovery:
    images: ["external-dns/external-dns"]
    helmCharts: ["external-dns"]
  upstream:
    github: kubernetes-sigs/external-dns
    tagPrefix: v
```

Things to get right:

- **`metadata.name` is an identifier.** It appears in users' release files, so
  pick a stable, lowercase name and never rename it.
- **Image patterns ignore registries.** Write `external-dns/external-dns`, not
  `registry.k8s.io/external-dns/external-dns`, so mirrors match too. Patterns
  are globs matched against any trailing part of the repository path.
- **Pick the source whose version is the product version.** Flux controller
  images are versioned separately from Flux, so its definition uses the Helm
  chart instead. If an image tag is not the product version, leave `images` out.
- **Check the upstream tags.** If the repository publishes several tag streams,
  for example `controller-v1.13.0` and `helm-chart-4.13.0`, set `tagPrefix` so
  only the product's tags count.
- **Nest where it belongs.** Crossplane providers and functions set
  `partOf: crossplane`.
- **Avoid broad API groups.** `argoproj.io` is shared by Argo CD, Rollouts and
  Workflows, so it cannot identify any one of them.

`catalog_test.go` loads every definition, so `make test` catches a malformed
file. Add a case to `TestMatching` for the image or package you expect to match.

## Code style

- Run `make check` before pushing. It runs the same formatting, vet, lint and
  test steps CI does. The linter is stricter than `go vet`: `prealloc` wants
  `make([]T, 0, n)` rather than `var x []T` when you append in a loop, and
  `unparam` rejects parameters nothing reads.
- Comments explain *why*, not *what*. If a line needs a comment to say what it
  does, rewrite the line.
- Wrap errors with context using `%w`: `fmt.Errorf("cannot list %s: %w", kind, err)`.
  Error strings are lowercase.
- Discovery is **read-only** and stays that way. Only `get` and `list` calls,
  never a write. See [GOVERNANCE.md](GOVERNANCE.md#principles).
- Do not add a Go or npm dependency without discussing it in an issue first.
- Keep the resource model backwards compatible within `v1alpha1` where you can,
  and call out anything that is not in the pull request.

## Testing

- Unit tests use the standard `testing` package.
- Anything that talks to a cluster or to GitHub is tested against fakes or
  `httptest`. No test needs a live cluster or network access.
- Test the pure logic. Version matching, evidence resolution, drift status and
  recommendations are ordinary functions over plain values, and those are
  where the bugs live. `TestBuildMatrix`, `TestClassifyPackage` and
  `TestResolveVersion` are the pattern to copy.
- The UI is type-checked by `npm run build`.

## Commit messages and sign-off

Write the summary line as a [Conventional
Commit](https://www.conventionalcommits.org/): a type, an optional scope and an
imperative description. Release notes are grouped by these prefixes.

```text
feat(catalog): recognise Envoy Gateway

Envoy Gateway is common on Gateway API platforms and was reported as an
unclassified Helm release.
```

Use `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `build`, `ci`
or `deps`, scoped by the package you touched (`catalog`, `discovery`,
`analysis`, `ui`, …). Append `!` for a breaking change to the resource model or
the CLI:

```text
feat(api)!: rename spec.discovery.includeUnclassified to hideUnclassified
```

Pull requests are squash-merged, so the pull request title becomes the commit
message.

All commits must be signed off under the
[Developer Certificate of Origin](https://developercertificate.org/):

```shell
git commit --signoff
```

## Pull requests

- Target `main`.
- One logical change per pull request.
- Add or update tests for behaviour you change.
- Add an entry under *Unreleased* in [CHANGELOG.md](CHANGELOG.md) for anything a
  user would notice.
- CI must pass: verify, lint, test on Linux, macOS and Windows, UI build,
  govulncheck, container image and DCO.

A maintainer will review within a few days. If nobody has, a polite ping on the
pull request is welcome.

## Releases

Maintainers cut releases by tagging `main`:

```shell
git tag -s v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

The release workflow builds the UI, runs GoReleaser, publishes signed archives,
SBOMs and multi-arch images to `ghcr.io/ravibagri5/platform-bom`, and creates a
draft GitHub release. A maintainer reviews the notes against
[CHANGELOG.md](CHANGELOG.md) and publishes it. Tags containing a hyphen, such as
`v0.2.0-rc.1`, are published as pre-releases and never move `latest`.
