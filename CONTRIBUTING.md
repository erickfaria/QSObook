# Contributing

QSObook is a personal, free software project for amateur radio operators. Documentation, bug reports, UI suggestions and code contributions are welcome. The maintainer decides scope and integration.

## Before starting

Read the [README](README.md), [architecture](docs/ARCHITECTURE.md), [roadmap](docs/ROADMAP.md) and applicable [agent instructions](AGENTS.md). The server, SQLite storage, operator registry, notebook, QSO mode and optional RadioID integration already exist. Do not implement something only because it appears in the roadmap: discuss major changes, dependencies, data-model decisions and architecture first.

Preserve Go, SQLite, a lightweight frontend without frameworks, local/offline operation and Linux ARM64 compatibility. Telemetry and tracking are not accepted. Contributions must use the project AGPL v3 and respect third-party licenses. Write public documentation in English. The UI supports Brazilian Portuguese, English, Spanish and French.

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

## Continuous integration

Pull requests to `main` and pushes to `main` run `.github/workflows/ci.yml` on native Linux AMD64 and ARM64 runners. Both jobs must pass before merging: Go formatting, module verification, Go tests/vet, JavaScript syntax/tests and the native build. Use the supported Go compiler pinned in the workflow; `go.mod` records the minimum supported language/toolchain version, not the CI compiler pin. Node.js 24 is only a test tool and requires no npm packages. Keep tests isolated and use fake local geographic providers. Do not call public map, geocoding or RadioID services from tests/CI. RadioID tests inject a local HTTP provider; retain consent, cache, manual-correction, pending-retry and historical-snapshot checks.

## Protected main branch

Changes reach `main` through pull requests with both native CI checks passing against the current base. Direct pushes, force pushes and branch deletion are blocked. These integrity rules have no bypass actors.

The repository-wide CODEOWNER is `@erickfaria`. Contributor pull requests require the maintainer's approval; new reviewable commits dismiss stale approvals. Only the maintainer’s approval satisfies the code-owner requirement. The maintainer's account has a pull-request-only exception to the separate review ruleset, intended for merging their own pull requests without another reviewer. GitHub scopes this exception to the account, not to the PR author, so the maintainer must reserve it for their own work. This exception never skips CI or permits direct pushes to `main`.

The integrity and review rules are separate repository rulesets. Do not replace them with the older classic-rule setup or remove CODEOWNERS. Recheck the actual GitHub settings before documenting future changes; the owner can change repository configuration independently of these files.

Official references: [rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets), [code owners](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners) and [required reviews](https://docs.github.com/en/pull-requests/how-tos/review-pull-requests/approving-a-pull-request-with-required-reviews).

## Documentation and releases

Write documentation for the final implemented behavior and distinguish current source from published images. The published v0.1.0 image uses schema 7; current source uses schema 8. Keep README, installation/source-build instructions, data compatibility, security, release notes and the Wiki aligned. Validate relative links, anchors and screenshot references before removing files. Older migration tests protect supported notebook upgrades and are not obsolete by age. Keep third-party files/licenses unchanged unless intentionally updating that dependency.

Record unreleased features in `docs/RELEASE_NOTES.md`. Merging to main runs CI; it does not publish an image or deploy to a user's machine. Publication uses the separate manual release workflow after selecting a new immutable version and reviewing its notes/upgrade instructions. Never overwrite a published release or use volume deletion as an update procedure.
