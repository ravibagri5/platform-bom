# Governance

This document describes how `platform-bom` is run. It is modelled on the
governance of CNCF projects such as
[Crossplane](https://github.com/crossplane/crossplane/blob/main/GOVERNANCE.md),
so that the project is easy to hand over to a foundation if it grows into one.

## Principles

- **Open.** Design discussion happens in public issues and pull requests. If a
  decision was made somewhere else, it gets written down here.
- **Read-only discovery.** `pbom` observes clusters and never changes them. The
  platform definition and its releases live in files the user controls. A
  change that would write to a cluster is out of scope, not a future feature.
- **The platform is the primary object.** Clusters, Helm, images and Crossplane
  are evidence. Features are framed around what the platform offers and which
  release it is on, not around Kubernetes objects.
- **Explainable, not magic.** Recommendations say why. A number with no reason
  attached is not a recommendation.
- **Vendor neutral.** No component, cloud or distribution gets special treatment
  in the code. Knowledge about specific tools lives in catalog definitions that
  anyone can add to or override.
- **Small dependency footprint.** Every dependency is a supply chain risk for
  everyone who runs the binary.

## Roles

### Contributor

Anyone who files an issue, comments on one, or opens a pull request. No
onboarding required.

### Reviewer

A contributor with a track record of good reviews. Reviewers are listed in
[MAINTAINERS.md](MAINTAINERS.md) and their approval counts toward merging, but
they cannot merge themselves.

Becoming a reviewer: have several non-trivial pull requests merged, then ask a
maintainer, or be nominated by one. Maintainers decide by lazy consensus.

### Maintainer

A reviewer who additionally has write access and is responsible for the health
of the project: triage, releases and security response. Maintainers are listed
in [MAINTAINERS.md](MAINTAINERS.md).

Becoming a maintainer: sustained contribution over at least three months,
demonstrated good judgement in reviews, nominated by an existing maintainer,
and approved by lazy consensus of the existing maintainers over seven days.

Maintainers who have been inactive for six months move to emeritus. This is
not a judgement; people's circumstances change. Emeritus maintainers can return
by asking.

## Decision making

Most decisions are made by **lazy consensus**: a proposal is made in an issue
or pull request, and if no maintainer objects within a reasonable period it is
accepted. In practice most changes are merged after one approving review.

Changes that need more than lazy consensus:

- Adding a dependency.
- A breaking change to the resource model (`Platform`, `PlatformRelease`,
  `Component`, `Inventory`) or to the CLI.
- Anything that would make `pbom` write to a cluster.
- Changes to this document, to `MAINTAINERS.md`, or to the licence.

For these, a maintainer opens an issue describing the change, and it needs
approval from a majority of maintainers with no outstanding objections. If
consensus cannot be reached, the maintainers vote; a simple majority decides
and a tie means the proposal does not proceed.

## Design proposals

Substantial changes, such as a new evidence source or a new resource kind,
start with a short document in `design/`: the problem, the proposal, the
alternatives considered, and what is explicitly out of scope. A proposal is
accepted when it is merged.

## Code of Conduct

Code of Conduct violations are handled by the maintainers in the first
instance, escalating to the CNCF Code of Conduct Committee as described in
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Licensing

All contributions are made under the Apache License 2.0 and must be signed off
under the [Developer Certificate of Origin](https://developercertificate.org/).
There is no CLA.
