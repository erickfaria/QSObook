# QSObook v0.1.0

The first stable release of QSObook, following beta-5 use on the maintainer’s Raspberry Pi. It preserves the minimal Go/SQLite/browser architecture, optional maps, four interface languages and password-free trusted LAN/VPN access.

## Reviewed and corrected

- Write targets now come only from the POST body; URL parameters cannot supply a missing record ID or city. Existing CSRF, Host, Origin, CSP and frame controls remain in place.
- Monthly contact totals now normalize stored timestamp offsets before applying the configured calendar-month boundaries.
- Removed unused legacy queries and styles. Consolidated duplicate security and lookup tests while preserving migration, historical-snapshot, backup/restore and replay regressions.
- Stable releases publish to the `stable` and `latest` Docker channels. Beta releases keep a separate `beta` channel. Downloads pin exact image digests, and release versions are never reused.

## Included

One QSO workspace for listening or optional named roundtables, remembered round names/editions, explicit QSO confirmation, QRT/return, independent operator/conversation notes, local callsign autofill, responsive white/pastel and dark themes, a Notebook, opt-in cached city maps, recoverable deletion, validated SQLite backup/restoration and previewed ASCII ADIF transfer.

## Update safely

**Preserve your notebook; do not wipe or reinstall it.** Back up first, then use the checksum-verified Compose files with your existing `.env`, project and `qsobook_data` volume. Beta 5 and v0.1.0 both use schema 7, so this upgrade introduces no new migration or database format. Older betas migrate transactionally. Never use `down -v` to update.

See the [installation/update guide](https://github.com/erickfaria/QSObook/blob/main/docs/INSTALL.md), [data guide](https://github.com/erickfaria/QSObook/blob/main/docs/DATA.md) and [security review](https://github.com/erickfaria/QSObook/blob/main/docs/SECURITY_REVIEW.md).

This is for trusted local/private networks, with an optional shared password. It is not a public multi-user service. Checks are source review, automated regressions, vulnerability scanning and hosted native AMD64/ARM64/container validation, not an independent external security audit. Ordinary simultaneous edits still use the last saved values; backups are unencrypted and must be stored privately. ADIF is contact interchange, not a full backup.
