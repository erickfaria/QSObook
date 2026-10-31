# Release notes

## QSObook v0.2.0

This stable release adds optional RadioID autofill to the existing QSO workspace. Ready-made Linux ARM64/AMD64 images and digest-pinned Compose downloads use **schema 8**. RadioID remains disabled by default; no external account is required.

- Optional RadioID autofill, off by default: Enter first reads SQLite, then performs an exact backend lookup only when necessary. The UI stays editable and reports progress/outcomes in all four languages.
- Missing fields are persisted locally. Manual values, edits made during lookup, and saves before the response arrives take priority. Multiple returned DMR IDs share one station; historical QSOs and closed/removed participations are preserved.
- Persistent retries for incomplete registered stations: at most ten per daily batch, spaced requests, provider cooldowns, seven-day no-match caching and offline manual entry.
- Schema 8 adds consent, retry/origin metadata and participant snapshots; validated backups accept exact schemas 5–8. **Back up first: v0.1.0 cannot open schema 8.**
- Removed obsolete screenshots/beta text, repeated style rules and duplicate security checks. Current screenshots, licenses, migrations and unique integrity/recovery regressions remain.
- Updated English usage, architecture, security, installation, recovery and contribution guides. No new runtime dependency, authentication model or Host/Origin/CSP policy.

### Upgrade from v0.1.0

**Back up before upgrading. Preserve the existing project, `.env`, password files and `qsobook_data` volume; never use `down -v`.** Download the v0.2.0 release files into a separate directory, verify `SHA256SUMS`, then replace only your chosen Compose profile and recreate the service. See the [update commands](https://github.com/erickfaria/QSObook/blob/main/docs/INSTALL.md#update-when-you-decide).

The first start migrates the notebook transactionally from schema 7 to 8, preserving registrations, QSOs and participation history. RadioID starts off. To use it, enable **RadioID autofill** under Preferences and press Enter in the QSO callsign field. Manual entry and local lookup continue to work offline. Enabling RadioID permits exact callsign requests and bounded daily retries; no notes or QSO details are sent.

**v0.1.0 cannot open a schema-8 notebook.** Rollback requires the separately preserved schema-7 backup and old deployment files. Never edit the schema version or point an older image at the only live copy. Schema-8 backups also restore their saved RadioID consent; review that option after restoration.

Validation includes Go tests with the race detector, vet, JavaScript regressions, vulnerability scanning and native Linux AMD64/ARM64 container checks. These checks do not replace a backup or deployment-specific manual testing. The supported access model remains a trusted LAN/VPN, with an optional shared password; this is not a public multi-user service.

## QSObook v0.1.0

The first stable release of QSObook, following beta-5 use on the maintainer’s Raspberry Pi. It preserves the minimal Go/SQLite/browser architecture, optional maps, four interface languages and password-free trusted LAN/VPN access.

### Reviewed and corrected

- Write targets now come only from the POST body; URL parameters cannot supply a missing record ID or city. Existing CSRF, Host, Origin, CSP and frame controls remain in place.
- Monthly contact totals now normalize stored timestamp offsets before applying the configured calendar-month boundaries.
- Removed unused legacy queries and styles. Consolidated duplicate security and lookup tests while preserving migration, historical-snapshot, backup/restore and replay regressions.
- Stable releases publish to the `stable` and `latest` Docker channels. Beta releases keep a separate `beta` channel. Downloads pin exact image digests, and release versions are never reused.

### Included

One QSO workspace for listening or optional named roundtables, remembered round names/editions, explicit QSO confirmation, QRT/return, independent operator/conversation notes, local callsign autofill, responsive white/pastel and dark themes, a Notebook, opt-in cached city maps, recoverable deletion, validated SQLite backup/restoration and previewed ASCII ADIF transfer.

### Update safely

**Preserve your notebook; do not wipe or reinstall it.** Back up first, then use the checksum-verified Compose files with your existing `.env`, project and `qsobook_data` volume. Beta 5 and v0.1.0 both use schema 7, so this upgrade introduces no new migration or database format. Older betas migrate transactionally. Never use `down -v` to update.

See the [installation/update guide](https://github.com/erickfaria/QSObook/blob/main/docs/INSTALL.md), [data guide](https://github.com/erickfaria/QSObook/blob/main/docs/DATA.md) and [security review](https://github.com/erickfaria/QSObook/blob/main/docs/SECURITY_REVIEW.md).

This is for trusted local/private networks, with an optional shared password. It is not a public multi-user service. Checks are source review, automated regressions, vulnerability scanning and hosted native AMD64/ARM64/container validation, not an independent external security audit. Ordinary simultaneous edits still use the last saved values; backups are unencrypted and must be stored privately. ADIF is contact interchange, not a full backup.
