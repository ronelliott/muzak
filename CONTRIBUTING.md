# Contributing to muzak

Thanks for your interest in contributing. Bug fixes are the highest priority; new features and refactors are welcome too, with the notes below. Review may take some time — please be patient.

## Bugs

Open a bug report issue with clear reproduction steps, before/after behavior, and the relevant platform and audio details (see the bug report template). Regression tests are strongly preferred when the fix is testable. The more platform and audio-stack context you include, the faster a fix can land.

## Features

Before opening a feature PR, please open a Discussion first to align on scope and design. Feature PRs that arrive without prior discussion are likely to be closed or substantially revised.

## Refactoring

A refactoring PR must include a short explanation of why the change improves the project — readability, performance, test coverage, or maintainability. Mechanical reformatting or churn without a clear reason will be declined.

## Pull Request Hygiene

Before marking a PR ready for review:

1. Run the full local validation (see § Validation).
2. Ensure CI passes on your branch.
3. Keep the PR focused on a single concern.

## Validation

Validate locally before pushing. CI covers the full OS matrix, but catching issues early saves time.

**macOS:**

```sh
go vet ./...
go test ./...
```

**Linux** (requires ALSA headers):

```sh
sudo apt-get install -y libasound2-dev
go vet ./...
go test ./...
```

**Windows** (CGO disabled — no system audio headers needed):

```sh
set CGO_ENABLED=0
go vet ./...
go test ./...
```

## Conventional Commits

This project uses [Conventional Commits](https://www.conventionalcommits.org/) because release automation parses commit history to determine version bumps and generate changelogs. Use one of: `feat`, `fix`, `docs`, `chore`, `test`, `refactor`, `build`, `ci`, `perf`, `style`, `revert`. A `feat` commit bumps the minor version; a `fix` bumps the patch; a `BREAKING CHANGE` footer bumps the major version.

## License

By submitting a contribution you confirm that you own or have the rights to the contribution and that it may be provided under the MIT license in `LICENSE`.
