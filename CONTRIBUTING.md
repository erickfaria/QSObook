# Contributing

QSObook is a personal, free software project for amateur radio operators. Documentation, bug reports, UI suggestions and code contributions are welcome. The maintainer decides scope and integration.

## Before starting

Read the [README](README.md), [architecture](docs/ARCHITECTURE.md), [roadmap](docs/ROADMAP.md) and applicable [agent instructions](AGENTS.md). The server, SQLite storage, operator registry, notebook and QSO mode already exist. Do not implement something only because it appears in the roadmap: discuss major changes, dependencies, data-model decisions and architecture first.

Preserve Go, SQLite, a lightweight frontend without frameworks, local/offline operation and Linux ARM64 compatibility. Telemetry and tracking are not accepted. Contributions must use the project AGPL v3 and respect third-party licenses. Write public documentation in English. The UI currently remains in Brazilian Portuguese.

## Issues

Use the [repository issues](https://github.com/erickfaria/QSObook/issues) for bugs and proposals; check for an existing discussion first.

- Bugs: include expected/observed behavior, reproduction steps, version or commit and relevant environment details.
- Proposals: describe the user need, workflow and smallest useful implementation.
- Use fictitious data. Do not attach personal databases, credentials or private logs.
- Follow [SECURITY.md](SECURITY.md) before disclosing vulnerability details publicly.

## Changes and pull requests

1. Fork if you do not have write access and use a descriptive branch, such as `docs/getting-started` or `fix/callsign-lookup`.
2. Make small, focused changes and commits. Preserve existing work and avoid unrelated refactoring.
3. Update affected documentation and run the appropriate checks before pushing an authorized branch.
4. Open a pull request against `main` explaining the problem, result, verification and limitations. Link related issues. Screenshots must contain fictitious data only.
5. Wait for maintainer review and integration. Significant changes require review before merging.

Use clear English commit messages describing intent. No additional contribution tooling is required. Do not bypass protections on `main`.

## Code and checks

Prefer the Go standard library, idiomatic names, `gofmt`, explicit errors, parameterized SQL and behavior-focused tests. Justify dependencies and their impact on ARM64, licensing and CGO. Do not incidentally change the module path or declared Go version.

Run `CGO_ENABLED=0 go test ./...`, `CGO_ENABLED=0 go vet ./...` and `CGO_ENABLED=0 go build -o bin/qsobook .`. Use temporary databases and fictitious records; tests must not depend on external services or touch real data. Changes affecting portability must verify Linux ARM64 and state environment limitations. See the README for repository-local caches. HTTP tests use `httptest`, without persistent services. Optional JavaScript checks use `node --test tests/*.test.cjs` and built-in modules only.

For documentation, review clarity, relative links and screenshots, then run `git diff --check`. Preserve `go.mod`, `go.sum`, SQL, static assets and required synthetic fixtures. SQLite exceptions under `testdata/` do not authorize personal data.

Do not install tools or change host services/configuration without authorization. Keep work inside the repository on a Raspberry Pi that also hosts other services.

## Manual GitHub configuration for main

This section guides the owner; it does not claim remote protections are configured. Confirm the remote default branch and protection settings manually. Keep these changes separate from application development.

For a public repository, the documented initial setup uses GitHub Free branch protection. In **Settings → General → Default branch**, confirm `main`. In **Settings → Branches → Add classic branch protection rule**, use the exact pattern `main`.

Suggested starting settings for a single maintainer:

- Enable **Require a pull request before merging**.
- Leave **Require approvals** off while no other eligible reviewer exists. This retains the PR requirement without blocking the owner from integrating their own PRs.
- Enable **Require conversation resolution before merging**.
- Enable **Do not allow bypassing the above settings**, including for the owner/administrators.
- Leave **Allow force pushes** and **Allow deletions** off.
- Add required checks only after real CI exists with those check names.

Fork contributors cannot merge while only the owner has write access. The owner reviews and merges manually. Without a required approval, this is a maintenance rule rather than a technical approval gate. Reassess the model before giving others write access.

**A PR author cannot approve their own PR.** Requiring an approval while blocking bypass prevents an owner who is the only eligible reviewer from merging their own work. Once another trusted reviewer has write access, consider requiring one approval and dismissing stale approvals. Do not make administrator bypass the normal workflow.

If third-party PRs must specifically require owner approval, evaluate a future `.github/CODEOWNERS` containing `* @erickfaria` and **Require review from Code Owners**. With only that code owner and no bypass, it also blocks the owner's PRs. Decide how those PRs will be reviewed before enabling it. No CODEOWNERS file is included here.

Personal repositories do not offer the same actor/role controls as organizations: classic **Restrict who can push to matching branches** and some bypass actor lists are organization features. The owner can still edit protection rules; their purpose is preventing accidental changes and reinforcing review.

Official references:

- [Protected branch availability, reviews, bypass and restrictions](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches).
- [Creating a branch protection rule](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/managing-a-branch-protection-rule).
- [Approvals and reviewing your own PR](https://docs.github.com/en/pull-requests/how-tos/review-pull-requests/approving-a-pull-request-with-required-reviews).
- [Code owners and required reviews](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

## Continuous integration

Pull requests to `main` and pushes to `main` run `.github/workflows/ci.yml` on native Linux AMD64 and ARM64 runners. Both jobs must pass before merging: Go formatting, module verification, Go tests/vet, JavaScript syntax/tests and the native build. Use the Go version in `go.mod`; Node.js 24 is only a test tool and requires no npm packages. Keep tests isolated and use fake local geographic providers. Do not call public map/geocoding services from CI.

## Protected main branch

Changes reach `main` through pull requests with both native CI checks passing against the current base. Direct pushes, force pushes and branch deletion are blocked. These integrity rules have no bypass actors.

The repository-wide CODEOWNER is `@erickfaria`. Contributor pull requests require the maintainer's approval; new reviewable commits dismiss stale approvals. Only the maintainer’s approval satisfies the code-owner requirement. The maintainer's account has a pull-request-only exception to the separate review ruleset, intended for merging their own pull requests without another reviewer. GitHub scopes this exception to the account, not to the PR author, so the maintainer must reserve it for their own work. This exception never skips CI or permits direct pushes to `main`.
