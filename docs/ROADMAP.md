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
- English documentation describing the workflows, setup and data guarantees.

Current checks run on Linux AMD64 with Go 1.27.3 and CGO disabled, including an ARM64 cross-build. The initial implementation was validated natively on ARM64. This revision still needs execution on Raspberry Pi, physical-phone testing and assistive-technology review. Explicit concurrent edits retain last-write-wins behavior. No vulnerability scanner is configured.

## Data preservation and portability — planned

- Export/import with documented formats and explicit duplicate handling.
- Consistent SQLite backup and validated restoration into a separate database.
- History pagination and improved Unicode search.
- Periodic review of driver and transitive dependency versions.

## Interoperability — planned

- Evaluate ADIF with explicit field conversion and no silent loss.

## Optional integrations — planned

- OpenStreetMap maps/geocoding only after informed opt-in, disabled by default, with provider, attribution and transmitted geographic data disclosed.
- No personal operator data sent to geographic providers.
- Missing internet or provider failure must never block the local notebook.

## Possible future work

Android support and easier updates may be evaluated after data handling is stable. Required cloud accounts, telemetry and mandatory external services are outside the product scope.
