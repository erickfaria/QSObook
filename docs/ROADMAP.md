# Incremental roadmap

The latest published image is v0.1.0 (schema 7). Current source additionally implements optional RadioID autofill (schema 8); it requires a source build until a new release is published. This roadmap describes current capabilities and possible next steps; it does not promise dates or unimplemented features. Historical changes are preserved in [Releases](https://github.com/erickfaria/QSObook/releases) and Git tags.

## Implemented in current source

- A Go/SQLite monolith with embedded HTML, CSS and plain JavaScript. The core notebook works offline, without accounts, telemetry or external callsign services.
- One QSO workspace for listening or optional named roundtables, remembered roundtable names/editions, an active/QRT list and explicit contact confirmation.
- Local callsign autofill plus optional exact backend RadioID queries on Enter, protected manual corrections and bounded daily retries of incomplete registrations.
- Independent operator/conversation notes, historical QSO snapshots, transactional writes and replay protection.
- Operator registration/editing and a paginated notebook with callsign/name/city search, common Latin accent folding and local-calendar date ranges.
- Portuguese, English, Spanish and French; compact phone controls, white/pastel and dark/system themes, locally bundled open fonts and original SVG branding.
- Optional Leaflet/OpenStreetMap city points, an approximate home marker, local mode/band filters, geographic-only opt-in geocoding, coordinate/negative caches, provider cooldowns and offline corrections.
- Recoverable deletion, validated SQLite backup/restoration with pre-restore recovery copies, and previewed ASCII ADIF import/export.
- Installed app/source/schema information and manual update guidance. Source migrations run through schema 8; v0.1.0 retains schema 7. Migration tests protect older notebooks.
- Docker Compose with persistent local storage, optional passwords for trusted LAN/VPN access, native Linux AMD64/ARM64 checks and separate stable/beta image channels.

See the [README](../README.md) for the workflow and screenshots, [architecture](ARCHITECTURE.md) for implementation/data guarantees, [installation guide](INSTALL.md) for deployment, and [data guide](DATA.md) for recovery and interchange limits.

## Maintenance

- Review driver and transitive dependency versions, licensing and security notices before updating them.
- Remove unused assets, obsolete text and duplicate checks while retaining migration, recovery, historical-data, security and replay regressions.
- Validate changes through Go tests/vet, browser regressions, builds and native AMD64/ARM64 container checks. Keep test data isolated and map providers local/fictitious.
- Continue physical Raspberry Pi and real-phone checks, including larger notebooks and assistive-technology review. Hosted ARM64 CI complements this work.

## Possible improvements

- Conflict detection for simultaneous explicit edits; current edits use last-write-wins behavior.
- Broader Unicode normalization, ADX/international interchange and additional ADIF fields, guided by actual use.
- Accessibility and mobile refinements based on feedback.
- Easier updates or Android support after the existing data workflow is stable.

Required cloud accounts, telemetry and mandatory external services remain outside the product scope.
