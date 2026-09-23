# Community

How this project is organised on GitHub: where conversations happen, how issues
are labelled and planned, and the one-time repository settings that cannot live
in a file.

## Discussions

[Discussions](https://github.com/ravibagri5/platform-bom/discussions) are for
working out whether something is worth doing. Issues are for things we have
agreed to do, and for defects. A maintainer converts a discussion into an issue
when it is ready, so nothing is lost by starting in the right place.

| Category | Format | Use it for |
| --- | --- | --- |
| **Announcements** | Announcement | Releases, breaking changes, roadmap updates. Maintainers post; anyone comments. |
| **Q&A** | Question and answer | "How do I...", "Why does it...". Answers get marked, so the category becomes a knowledge base. |
| **Ideas** | Open-ended | Anything not yet formed enough to be an issue. Upvotes here feed the roadmap. |
| **Show and tell** | Open-ended | Platform pages, release workflows, catalogs. The most useful signal we get about what platforms look like. |
| **Catalog** | Open-ended | Which components to recognise and how, before a definition is written. |
| **Contributors** | Open-ended | Design discussion between contributors, release coordination, triage questions. |

Templates for Ideas, Q&A and Show and tell live in
[.github/DISCUSSION_TEMPLATE](../.github/DISCUSSION_TEMPLATE).

What does **not** belong in Discussions:

- Security vulnerabilities. Use
  [private reporting](https://github.com/ravibagri5/platform-bom/security/advisories/new).
- Reproducible bugs. Open a
  [bug report](https://github.com/ravibagri5/platform-bom/issues/new?template=bug_report.yml).

## Issue triage

Every new issue arrives with `needs triage`. A maintainer removes it once the
issue has:

1. a **type** label: `bug`, `enhancement`, `proposal`, `documentation`, …;
2. an **area** label: `area/discovery`, `area/ui`, …;
3. a **priority** label;
4. a **theme** label, if it maps to a roadmap theme;
5. a **milestone**, if it is accepted for a specific release;
6. a place on the **project board**, with Theme, Area, Priority and Size set.

Other conventions:

- `needs information` is applied when we are waiting on the reporter. Issues
  with it are closed after 30 days of silence, and reopening one is welcome.
- `good first issue` is only applied once the issue says what to change and
  where. Most component requests qualify.
- `help wanted` means no maintainer is working on it and a pull request will be
  reviewed promptly.
- `breaking change` means the resource model or the CLI changes in a way that
  breaks users' files or scripts. These are called out first in release notes.

The label catalogue is [.github/labels.yml](../.github/labels.yml) and is
applied by the labels workflow. Change labels there in a pull request, not in
the UI: labels created by hand are removed on the next sync.

## Planning

| Where | What it holds |
| --- | --- |
| [ROADMAP.md](../ROADMAP.md) | The direction and the next priorities, linked to issues. Changed by pull request. |
| Milestones | One per roadmap theme, `M0 — Project foundation` to `M11 — PBOM specification`, matching the README. Releases are cut from `main` and are not tied to a milestone. |
| **platform-bom Roadmap** project | The board. Every accepted issue is on it, grouped by milestone and theme. |

The project board is owned by the maintainer's GitHub account and linked to
this repository, the same way the crossplane-mcp-server board is. Its fields:

| Field | Values |
| --- | --- |
| Status | Todo, In Progress, Done, Blocked |
| Theme | Discover, Version, Advise, History, Present, Integrate |
| Area | api, catalog, discovery, upstream, analysis, release, server, cli, ui, deploy, build, ci, integrations |
| Priority | Critical, High, Medium, Low |
| Size | XS, S, M, L |

[hack/github-setup.sh](../hack/github-setup.sh) creates the milestones and the
board and applies the settings below. It is safe to re-run.

## Repository settings

These cannot be committed, so they are recorded here. Maintainers with admin
access apply them; `hack/github-setup.sh` does most of it.

**General**

- Default branch: `main`.
- Merge button: squash only.
- Automatically delete head branches after merge: on.
- Discussions: on, with the categories above.
- Issues: on, with blank issues disabled.
- Private vulnerability reporting: on.

**Branch protection for `main`**

- Require a pull request, with one approving review from a code owner.
- Require these status checks: *Verify*, *Lint*, *Test (ubuntu-latest)*,
  *Test (macos-latest)*, *Test (windows-latest)*, *Web UI*, *Build*,
  *Vulnerability scan*, *Container image*, *Check sign-off*.
- Require branches to be up to date before merging.
- Require linear history.
- Do not allow force pushes or deletions.

## Becoming a maintainer

See [GOVERNANCE.md](../GOVERNANCE.md). In short: sustained, good quality
contributions and review, then nomination by an existing maintainer.
