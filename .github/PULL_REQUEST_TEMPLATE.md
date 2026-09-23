<!--
Thanks for contributing. A short description of why this change is needed is
worth more than a long description of what it does; the diff already says what
it does.

Title: use a Conventional Commit, for example
  feat(catalog): recognise Envoy Gateway
It becomes the squashed commit message.
-->

## What does this change?

<!-- One or two sentences. -->

## Why?

<!--
What problem does this solve? If it fixes an issue, link it:
Fixes #123
-->

## How did you test it?

<!--
For discovery or catalog changes: which distribution, and which versions of the
components involved? "Unit tests only" is a fine answer for changes that do not
touch cluster behaviour.
-->

## Checklist

- [ ] The title is a Conventional Commit
- [ ] Commits are signed off (`git commit --signoff`)
- [ ] Tests cover the behaviour I changed
- [ ] `make check` passes locally
- [ ] The UI still builds (`make ui`), if I touched `web/`
- [ ] CHANGELOG.md has an entry under Unreleased, for user-visible changes

## Breaking changes

<!--
Renaming a field in pbom.yaml, a release file or a Component definition, or
changing a CLI flag, breaks users' files and scripts. If this does that, say so
here and explain the migration. Otherwise write "None".
-->

None
