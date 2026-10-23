# Incremental roadmap

No dates are promised. Planned features are not available until implemented and verified.

## Implemented foundation

- Go 1.24.4 module declaration, standard-library HTTP, embedded assets, local configuration and SQLite without CGO.
- Separate operator registrations, historical QSO copies and current participants, with transactional migrations and relational integrity.
- Operator registration/editing and a notebook with callsign/name search, contact details and editing.
- QSO mode with a participant table, local callsign lookup, explicit confirmation, editing and removal.
- Responsive forms, labels, keyboard focus, CSRF protection, HTTP limits and automated storage/HTTP tests.

## Implemented QSO reliability

- Adding a participant stays separate from confirming a contact.
- Existing operator fields survive blank, repeated and stale inclusion forms; replacements require explicit editing.
- Permanent notes remain separate from conversation notes and historical contacts.
- Registration and participation/manual-contact writes are transactional.
- Repeated or simultaneous confirmation does not duplicate a QSO for the same participation.
- Schema version 2 preserves existing records and prevents reuse of removed participation IDs.
- Compact entry, inline editing and working alternatives without JavaScript.
- Lookup request ordering and isolated drafts are covered by JavaScript tests without external packages.
- English documentation describing the workflows, setup and data guarantees, with real application screenshots using fictitious data.

CI runs native Linux AMD64 and ARM64 checks using Go 1.27.3 with CGO disabled, including container persistence/backup tests. Pinned govulncheck checks reachable Go vulnerabilities. Physical Raspberry Pi and phone feedback remains part of beta testing; hosted ARM64 tests do not replace it. Assistive-technology review is pending. Explicit concurrent edits retain last-write-wins behavior.

## Implemented conversation workspace and home

- Muted pastel light/dark palettes with a browser-local selector and system mode.
- An essential home screen with confirmed-contact count and recent saved rounds.
- Explicit round start/end, optional names, QRT/return state and read-only closed rounds.
- Legacy participant preservation, non-reusable IDs and rejection of stale cross-round edits/actions.
- Optional Leaflet/OpenStreetMap city points from confirmed historical contacts, grouped by location with links to city history.
- Map opt-in, locally vendored assets, geographic-only manual geocoding, coordinate/negative caches, bounded requests and provider switching.
- Tests for lifecycle, migration rollback, QRT, map privacy, caching, rate limits and provider failures; screenshots for the current UI.
- GitHub Actions for pull requests and `main`: formatting, module verification, Go/JavaScript tests, vet and native Linux AMD64/ARM64 builds.

## Implemented data tools (beta 5)

- Recoverable QSO/station deletion with explicit confirmation and independent restoration, preserving historical links and replay protection.
- Consistent online SQLite snapshots, strict isolated validation, restoration preview, durable pre-restore copies and atomic table replacement.
- Fifty-row pagination, callsign/name/city search with common Latin accent folding and local-calendar history ranges.
- ASCII ADIF contact export/import with explicit field mapping, preview, unsupported-field disclosure and transactional duplicate handling.
- Offline map coordinate correction and explicit cached/no-match retry under existing provider controls.
- Installed app/source/schema version display and manual update guidance without remote update checks.

## Maintenance and possible improvements

- Periodic review of driver and transitive dependency versions.
- Broader Unicode normalization, ADX/international interchange and additional ADIF fields, based on actual user feedback.
- Review accessibility and physical Raspberry Pi behavior with larger notebooks and real mobile browsers.

## Possible future work

Android support and easier updates may be evaluated after data handling is stable. Required cloud accounts, telemetry and mandatory external services are outside the product scope.

## Implemented unified records (beta 4)

- One entry screen with explicit confirmation, optional roundtable labels, local country dropdowns, mode/band choices and custom values.
- Database-backed replay protection for the new form; schema-5 upgrades preserve older notebooks and optional live rounds.
- Monthly counts in the configured time zone, opt-in city geocoding on save, local coordinate reuse, a home-city marker and local mode/band map filters.
- Compact rounded mobile controls, original SVG project branding and locally bundled open fonts/country data with license notices.
