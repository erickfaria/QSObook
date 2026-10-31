# Security review — 2026-10-24

Scope: preparation of stable **v0.1.0**, based on beta-5 source `1fe52c33901f486740e44c745a8a853247292eea`. Reviewed the HTTP/access boundary, registration/QSO transactions, local lookup, backup/restoration and ADIF paths, optional geographic requests, embedded assets, container profiles and release workflow. This is a source review with automated checks, not an independent external audit or a guarantee against all vulnerabilities.

## Confirmed corrections

- Some POST handlers accepted a missing record identifier or map city from URL parameters through `FormValue`. Identifiers and editable cities now come only from the POST body for writes. GET links retain query parameters. The regression reproduced unintended writes before the fix and now verifies rejection without database changes, plus legitimate body-targeted updates. CSRF was still required before this fix; this finding did not establish an unauthenticated write bypass.
- Monthly totals compared raw timestamp strings. Compatible older backups can contain offsets or omit fractional seconds, producing incorrect calendar-month counts. The dashboard now shares the UTC-second conversion used by history/ADIF. Tests reproduced errors for negative and positive offsets, then passed after correction.
- Invalid map-edit locations now consistently return a validation response rather than an internal-error response.

No database schema, runtime dependency, authentication model, Host/Origin policy, CSP/frame policy or GitHub branch-protection change was required. Stable v0.1.0 retains schema 7 from beta 5.

## Cleanup and retained coverage

Removed unused city-history and legacy list/contact queries and unused dashboard/table styles. Test fixture helpers now use the production paginated readers and contact transaction. Consolidated duplicated route-security checks and equivalent lookup-note tests; retained upgrade/rollback tests for old notebooks, because older data still needs safe migration.

The central security matrix covers all 23 POST routes, including streamed restore/import uploads, with eight request-forgery cases for both loopback and explicitly allowed LAN hosts: foreign/null/HTTPS/wrong-port Origins, invalid/missing CSRF, and cross-site fetch metadata with or without an Origin. Sixteen action-only routes reject GET and HEAD. Rejected requests must not alter SQLite, set preference cookies or create transfer previews. The 2026-10-31 maintenance cleanup consolidated the older browser-origin checks into this matrix without changing the application security policies.

## Verified controls

- Parameterized SQL, normalized/validated callsigns, bounded fields and automatically escaped HTML. Lookup handles stale responses and keeps permanent/listening notes independent.
- Explicit allowed host:port entries, unchanged CSRF/Origin checks, POST-only writes, bounded body/header/time limits, frame blocking, local-resource CSP and no-store responses.
- Optional shared-password access precedes every route. Password-free trusted LAN/VPN access remains the maintainer's explicit deployment choice; all connected users share the notebook.
- Transactional participation/confirmation, replay keys and historical snapshots; adding a station and marking QRT do not confirm contacts.
- Streamed private upload staging after the first CSRF part, exact-schema read-only validation, explicit transfer preview, durable validated recovery snapshots, transactional replacement and expiration of old forms. Uploaded SQL/schema extensions are rejected.
- Maps/geocoding remain opt-in with geographic fields only, fixed deployment-configured providers, bounded requests, redirect rejection, local caches and persistent cooldowns. Tests use fake local providers.
- Embedded local assets/fonts; no telemetry, external callsign service or new dependency. Docker runtime remains non-root, read-only, capability-restricted and backed by a named local volume.
- Read-only PR workflow token, pinned actions, separate manual publication, native AMD64/ARM64 and exact-image checks, digest-pinned assets and provenance. Stable/latest channels are separate from beta; published versions are not reused.

## Validation record

Local Go 1.27.3/Linux AMD64 checks: Go tests and race detector, vet, module verification, AMD64 build and ARM64 cross-build. JavaScript: 18 regressions with built-in Node modules and syntax checks. Workflow lint, formatting, Markdown links and whitespace checks are included. Go statement coverage after cleanup: **76.6%** (76.1% before this review). Native hosted AMD64/ARM64/container checks are required before merge/publication; exact release-image checks run again before pushing.

Pinned govulncheck v1.9.0 reported **zero reachable vulnerabilities and zero findings in imported packages**. It also reports [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024) in the required `golang.org/x/sys` module: the affected `windows.NewNTUnicodeString` API is not imported or reachable in the supported Linux builds. This module-only advisory remains documented; no unrelated dependency change was made.

A limited tracked-tree filename/pattern check covers runtime database/secret files and recognizable private-key/GitHub/AWS token patterns. It is not a full-history or external secret-scanner audit. Final execution results and native jobs are recorded in the release PR and Actions run; this document does not replace those records.

## Operational limits

Private LAN/VPN access and private host storage remain necessary. HTTP is not encrypted, and the optional shared password is not a multi-user permission system. Backups contain unencrypted personal data and must be stored privately. Explicit concurrent registration edits retain last-write-wins behavior. Local file access by the same OS account is trusted. A participant link to a trashed QSO requires restoration before opening that QSO. ADIF is interchange, not a complete backup. Successful beta-5 use on the maintainer's Raspberry Pi is reported by the maintainer; automated ARM64 checks use hosted runners, and each installation still needs its own smoke test after updating.
