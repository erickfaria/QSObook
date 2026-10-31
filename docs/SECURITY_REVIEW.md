# Source security review

Reviewed on **2026-10-31** for current source, including optional RadioID enrichment and schema 8. The published v0.1.0 image remains schema 7. Scope: HTTP/access controls, local lookup, registration/QSO integrity, RadioID requests/retries, backup/restoration, browser behavior and the existing container/release boundary. This is a source review with automated checks, not an independent audit or a guarantee against all vulnerabilities.

## RadioID and privacy

- Consent is off after migration. Local lookup, registration and confirmation remain usable offline. Browser requests target the local server; only the backend contacts RadioID.
- Lookup sends an exact validated callsign, fixed query options and identifying client metadata. It sends no operator name, QSO details, notes or database. A missing entry is not license/identity verification.
- The fixed HTTPS client rejects redirects, has a three-second timeout and bounds responses to 256 KiB/200 rows. Non-exact callsigns and invalid data cannot enrich another station. Multiple IDs share one registration.
- Serialized requests, persisted provider leases/cooldowns, cached outcomes and a daily batch of at most ten incomplete registered stations limit external work. The minute worker only checks local eligibility. Provider failures retain pending state and do not block manual QSO entry.
- Opt-out waits for an existing bounded attempt before acknowledging the disabled setting. No later RadioID lookup is permitted while disabled. Restoring an enabled schema-8 backup restores that consent and retry schedule.

Map consent is independent. Geographic providers receive only the disclosed city/state/country or tile-view information, never callsigns, operator names or notes. Geographic lookups do not run in a background batch. Both integrations are tested with local fictitious providers.

## Data integrity and restoration

RadioID fills only missing current fields. Explicitly typed corrections replace only values still marked external; matching revisions support changes from a fresh form. This also protects Save before a lookup response arrives. Once a field is manual, stale automatic values cannot replace it. Permanent operator notes remain separate from conversation notes.

QSOs retain independent historical copies. Before automatic enrichment changes a registration, existing closed/removed participations preserve their previously displayed details and original text. Adding a station, marking QRT or ending a session never creates a QSO. Registration/participation/confirmation writes remain transactional and replay-protected.

SQLite uploads are inspected read-only against exact known schemas 5–8 before embedded migrations run in an isolated staging copy. Known schema-8 pending-lookup triggers are part of the expected signature; foreign objects, injected triggers/views and invalid metadata are rejected. RadioID IDs, provenance and historical snapshots are validated along with existing notebook data.

Restoration preserves a validated durable recovery snapshot, replaces rows in one transaction and expires old forms. Individual RadioID attempts share the request lock so a response from the previous notebook cannot write into the restored one. Background writes can expire a transfer preview through the existing change check. ADIF imports require preview/confirmation and do not initiate provider calls themselves; incomplete imported registrations can later qualify for an opted-in daily RadioID retry.

## HTTP and deployment controls

The central security matrix covers **25 POST routes**, including streamed uploads and RadioID lookup/consent, against eight forgery cases on both loopback and explicitly allowed LAN hosts. **18 action-only routes** reject GET and HEAD. Rejected requests must not modify SQLite, set cookies or create transfer previews. Route-wide checks replace duplicate language/origin tests; unique cookie, return-destination, localization and data-preservation cases remain.

Parameterized SQL, validated fields, template escaping, exact Host handling, CSRF/Origin checks, POST-body write targets, bounded request sizes/timeouts, local-resource CSP, frame protection and no-store responses remain in place. No policy exception is needed for RadioID or screenshots. The optional shared password applies before all routes; unauthenticated LAN/VPN access remains the maintainer's explicit deployment model.

Runtime dependencies and licenses are unchanged. Assets/fonts remain local. Containers stay non-root, read-only and capability-restricted, with one persistent local volume. Main requires successful native AMD64/ARM64 CI; publication is separate, versioned and digest-pinned. A merge does not publish an image or deploy to a user's server.

## Verification and evidence

Relevant local checks include Go tests with the race detector, vet, module verification, AMD64 build and ARM64 cross-build; **20 JavaScript regressions**, syntax, formatting, Markdown/anchor checks and fictitious-provider tests. Migration/recovery tests remain necessary for supported old notebooks and are retained. CI reruns native AMD64/ARM64 builds and all three Compose profiles before integration.

The PR and [Actions checks](https://github.com/erickfaria/QSObook/actions/workflows/ci.yml) record exact revisions/results. Cross-compilation and hosted ARM64 checks do not replace testing the installed Raspberry Pi. The integration tests do not call the real RadioID API or claim provider availability.

Pinned govulncheck v1.9.0 reported zero reachable vulnerabilities and zero findings in imported packages in the current local check. It also reported the module-only [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024) in the required x/sys Windows API, outside the packages used by supported Linux builds. CI records the scan for the exact revision; no unrelated dependency upgrade is implied.

## Earlier stable corrections

The v0.1.0 review on 2026-10-24 corrected query-parameter fallback for POST write targets, normalized stored timestamp offsets for monthly totals and returned validation errors for invalid map-edit input. It consolidated unused queries/styles and overlapping tests. That published release retained schema 7; schema 8 belongs to the later RadioID source changes.

## Operational limits

Use a trusted local/private network or VPN and private host storage. HTTP and optional Basic authentication do not encrypt traffic or create per-user isolation. Backups are unencrypted. Explicit registration edits use last-write-wins; local OS-account access is trusted. A link to a trashed QSO requires restoration. ADIF is interchange, not a complete backup. Provider policies/access may change. Use one application process per notebook, retain an external backup before migrations, and verify each updated installation.
