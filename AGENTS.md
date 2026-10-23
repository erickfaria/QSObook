# Instructions for AI agents

These instructions apply to the entire QSObook repository. Read existing files and inspect Git before editing. Preserve prior user changes, including staged changes.

## Scope and architecture

- Keep Go, SQLite and plain HTML/CSS/JavaScript. Prefer `net/http`, `html/template` and the standard library.
- Preserve the module `github.com/erickfaria/QSObook`. Do not change the Go version without a demonstrated need discussed with the maintainer.
- Prioritize simplicity, readability, maintenance and Linux ARM64, especially Raspberry Pi running Debian.
- Create packages/directories only for code that is needed. Avoid generic layers, heavy frameworks and speculative abstractions.
- Justify dependencies by reviewing licensing, maintenance, need, ARM64 support and CGO implications. The standard library has no SQLite driver.
- Distinguish confirmed decisions from proposals and implemented features from plans.
- Write public documentation and new commit messages in English. Do not change the UI language as a side effect of documentation work.

## Go and UI

- Use `gofmt`, idiomatic names, small packages and focused functions.
- Handle and propagate errors with context; do not silence failures or panic for expected errors.
- Use `context.Context` for cancellation or execution limits. Close resources and bound transactions.
- Avoid mutable global state and interfaces without a real consumer.
- Keep the UI minimal, responsive and accessible, with semantic HTML, labels and keyboard navigation.

## Privacy and security

- Telemetry, analytics, tracking and transmission of usage data are prohibited.
- Core functionality must work offline; serve essential assets locally without CDNs, external fonts or remote APIs.
- Keep data in local SQLite. Do not unnecessarily log contacts, notes, credentials or form contents.
- External maps/geocoding must be optional, disabled by default, with clear disclosure and conscious opt-in. Send geographic information only, never personal operator data.
- Use parameterized SQL, server validation, template escaping and input limits. Never treat user input as trusted HTML.
- Preserve CSRF protection, rejection of writes through GET, HTTP timeouts and error messages without internals.
- Never serve databases, backups or configuration as static files. Do not put real personal data in examples, tests or screenshots.
- Do not expose the server on the network or decide authentication/public access without discussing the access model.
- Preserve AGPL v3 and license notices; do not add incompatible restrictions.
- Do not weaken iframe, CSP, Host or Origin protections merely to make a test browser work.

## Tests and documentation

- Test new/changed code proportionally to risk, preferably with `testing`, `httptest` and isolated temporary databases.
- Cover relevant validation, persistence, lookup, errors and integrity. Never use the user's real database in tests.
- For Go packages, run `go test ./...` and `go vet ./...` when possible. Report blockers and do not claim checks that were not run.
- Verify build and ARM64 behavior when code/dependencies can affect them; cross-compilation alone does not resolve CGO requirements or prove hardware execution.
- For documentation-only changes, check links, screenshots, consistency and `git diff --check`; do not create artificial tests.
- Keep affected documentation reproducible and avoid promising unavailable features.

## Operational limits

- Work inside the repository. Do not inspect credentials or unrelated personal files outside the project.
- The host runs other services: do not install packages or change the OS, services, Docker, network, firewall, DNS or Tailscale.
- Do not perform destructive operations, delete user data, rewrite history or revert another person's work without explicit authorization. Ask before important undefined decisions.
- Use small, focused commits. Commit, push or merge only with user authorization; existing authorization in the conversation applies. Verify the relevant checks before an authorized push.
- Do not alter remote configuration without authorization. Use the pull request and maintainer review workflow; never bypass `main` protections.
- Finish with changed files, verification results, limitations and remaining work.

## Current implementation

- One root Go package: `main.go`, `store.go`, `web.go` and tests. Templates, assets, migrations and license notices are embedded.
- `modernc.org/sqlite` runs without CGO. Keep its required `modernc.org/libc` version aligned; changes require ARM64 and license checks.
- Preserve the distinction between current registrations (`stations`), historical contact copies (`qsos`) and roundtable participation (`participants`). Do not propagate registration edits into old QSOs.
- Use transactional migrations versioned by `PRAGMA user_version`, preserving records and rejecting future versions. Version 2 keeps participant IDs non-reusable across removals and restarts.
- Run `CGO_ENABLED=0 go test ./...`, `CGO_ENABLED=0 go vet ./...` and `CGO_ENABLED=0 go build -o bin/qsobook .`. Keep caches/temporary files in `.cache/` when required; see the README.
- Optional JavaScript regressions use `node --test tests/app.test.cjs`, without npm packages.
- Never develop/test using the default personal database or start a persistent server. Temporary preview servers must use an isolated database, stay on loopback and be stopped after use.
