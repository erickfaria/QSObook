# QSObook test beta 5

This beta unifies the live QSO workflow and adds data recovery and portability while preserving Go, SQLite and the compact browser interface. Linux AMD64 and ARM64 images are published after native tests and real container integration checks.

## Included

- QSO mode in the main navigation, replacing duplicate entry panels. Start an ordinary listening session or opt into a named roundtable with an optional edition; the last name/edition is remembered locally.
- One entry form and live participant list. Adding a station never confirms a QSO; use the unchecked confirmation box or the row action. Permanent and conversation notes remain separate. QRT dims/moves departed stations below active ones, with a return action.
- White light mode with restrained pastel accents, dark/system themes, compact phone controls, current screenshots and a starter English Wiki.

- Recoverable deletion of QSOs and stations, with confirmation and a Trash screen. Historical snapshots, links and replay keys remain intact; no permanent purge.
- Consistent online SQLite backup downloads and validated full-notebook restoration. Review the preview before confirming; a durable pre-restore recovery snapshot is saved first, replacement is transactional and old forms expire.
- Fifty-row pagination, callsign/name/city search with common Latin accent folding and local-calendar date ranges.
- ADIF ASCII ADI export/import with preview, explicit unsupported-field disclosure, conservative duplicate handling and preservation of current registrations. Unicode/custom QSObook values survive in `APP_QSOBOOK_DATA`; other programs may ignore this extension. ADX and non-ASCII ADI are not supported.
- Offline coordinate corrections and explicit retries for cached/no-match cities under the existing map opt-in, geographic-only requests and cooldowns. Imports never geocode automatically.
- Installed version/source/schema display, `-version`, English documentation and current desktop/mobile screenshots. Four interface languages remain supported.

## Upgrade carefully

**Back up before updating.** Schema 7 adds nullable deletion timestamps/indexes plus optional round editions and remembered roundtable names. The migration preserves existing data; beta 4 cannot open the upgraded notebook. Keep a pre-upgrade schema-5 snapshot for rollback. A raw database copied while running may be incomplete without its journals; use a consistent snapshot or the stopped-directory backup procedure.

The restore UI accepts exact schema-5/6/7 snapshots up to 64 MiB, replacing the complete notebook only after confirmation. ADIF imports are limited to 16 MiB/10,000 records and do not replace a full backup. Recovery copies stay on the server until managed manually; download an external offline copy. See [data and recovery procedures](https://github.com/erickfaria/QSObook/blob/main/docs/DATA.md).

Use the downloaded checksum-verified, digest-pinned Compose files and preserve `.env`, the project name and `qsobook_data`. Password-free trusted LAN/VPN deployment remains available in `compose.lan.yaml`; optional shared-password access uses `compose.auth.yaml`. Host/Origin/CSRF/CSP protections remain in place. Do not expose the unauthenticated configuration to the internet. Never use `down -v` to update.

This remains a testing beta. Native hosted ARM64/container checks do not replace physical Raspberry Pi testing. Downloading a new GitHub commit does not deploy to your Pi. Follow the [installation/update guide](https://github.com/erickfaria/QSObook/blob/main/docs/BETA.md).
