# Contributing to plugxfer

`plugxfer` converts plugins between Claude Code and Codex while reporting every
compatibility decision.

## Prerequisites

- [Go](https://go.dev/) (version from `go.mod`)
- GNU Make

## Build, test and lint

```bash
# Format
gofmt -w .

# Verify formatting
make fmt-check

# Vet
make vet

# Test
make test

# Build
make build

# Run the full local CI suite
make check
```

## Documentation

Keep the PRD, rule behavior, CLI help, and user documentation current in the
same pull request as the behavior they describe. Converter changes must state
whether they copy, transform, drop, or require a mapping for the affected
feature.

## Before you open a PR

- Run `make check`.
- Add focused tests for every reader, detector, strategy, or writer change.
- Update golden fixtures when conversion output intentionally changes.
- Confirm that no compatibility loss disappeared from the report silently.

## Branching and commits

- Branch off `main`; never commit directly to `main` after repository bootstrap.
- Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes.
- Sign commits where possible.
- Keep each pull request focused.

## Pull requests

- Open the pull request against `main`.
- Every pull request runs CI.
- A pull request can be merged once CI is green and all review threads are resolved.

## Releases

The release process will be defined before the first tagged version. Until
then, `main` is the only supported development channel and no stability
guarantee is made.
