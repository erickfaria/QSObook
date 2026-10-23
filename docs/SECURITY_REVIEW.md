# Security review — 2026-10-23

Reviewed application source: `79f254a479705b15fda499219243949114c307d3` (beta 3).

## Result and changes

This source review and the checks below found no newly confirmed exploitable application vulnerability. No production code, database schema, runtime dependencies, access settings or GitHub protection settings were changed.

Three security regression tests were added in `security_test.go`:

- Every current write route rejects external/null Origins and missing/invalid CSRF tokens without writing to SQLite or setting preference cookies. Action-only routes reject GET and HEAD.
- SQL-like names and stored HTML remain data: registration, participation and contact writes preserve the input; rendered pages escape markup and searches do not modify tables.
- Private database, configuration, credential and source paths remain unavailable through the HTTP/static routes, including after path-cleaning redirects.

`CONTRIBUTING.md` contained obsolete classic branch-protection instructions that contradicted the active rulesets and CODEOWNERS file. Those instructions were removed, and the documented languages and CI compiler guidance were corrected. `ROADMAP.md` incorrectly said no vulnerability scanner was configured; it now describes the existing govulncheck and native CI jobs.

## Verified controls

- SQL parameters and escaped templates; no user-controlled SQL statements or trusted HTML conversion.
- Explicit Host allowlist, unchanged Origin/CSRF checks, frame blocking, CSP, body/header/time limits and no-store responses.
- Optional password applies before every application route; invalid configured password files fail startup. Password-free access remains an intentional LAN/VPN deployment option.
- Transactions and preserved registration/contact snapshots; repeated participant confirmation is tested separately from participation.
- Maps remain opt-in. Geographic-only geocoding uses fixed deployment-configured providers, bounded requests, redirect rejection, local caches and cooldowns; tests use local fake providers.
- Container build context excludes databases, backups, secrets, `.env`, caches and Git. Runtime remains non-root with a read-only root filesystem and a persistent data volume.
- Active GitHub rulesets require PRs and both native checks against an up-to-date base. Integrity rules have no bypass actors. The separate maintainer review exception is described in `CONTRIBUTING.md`.
- Workflow actions are pinned, PR tokens have read-only contents permission, checkout credentials are not persisted and publishing uses separate manually dispatched jobs.
- Checked tracked runtime filenames and private-key/GitHub-token/AWS-key patterns: no matches. This was a limited current-tree check, not a full-history secret-scanner audit.

## Local validation

Using Go 1.27.3 on Linux AMD64:

- `CGO_ENABLED=0 go test -mod=readonly -count=1 -cover ./...`: passed; statement coverage 74.1%.
- `CGO_ENABLED=1 go test -mod=readonly -race -count=1 ./...`: passed. CGO was enabled only for the race detector; production builds remain CGO-free.
- `go vet ./...`, module verification and AMD64/ARM64 builds: passed.
- JavaScript regressions: 15 passed, no npm packages required.
- Pinned workflow lint and whitespace checks: passed.
- `govulncheck v1.9.0`: no reachable vulnerability or imported-package finding. It reports [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024) at module level in `golang.org/x/sys/windows.NewNTUnicodeString`; this package is not used by the Linux application. Dependencies were not changed for an unused Windows-only path.

The PR's GitHub Actions checks run the tests and container smoke tests natively on Linux AMD64 and ARM64. Local cross-compilation alone is not reported as physical Raspberry Pi validation.

## Remaining limits

This is a source review with automated regressions, not an external penetration test or an assessment of a user's host, router, VPN rules or other containers. Govulncheck does not cover every embedded upstream component or unknown vulnerability. Physical deployment testing remains part of the beta.

Without a password, everyone who can reach the published port can read and modify the notebook. Network/VPN access rules must restrict that port. Plain HTTP does not encrypt notebook traffic outside the VPN. Individual accounts, database encryption, automatic updates and in-app backup/restore remain unavailable. Explicit concurrent edits still use last-write-wins behavior, and manual new-contact submissions do not have server-side replay deduplication. Preserve offline backups before upgrades and avoid exposing the service directly to the internet.
