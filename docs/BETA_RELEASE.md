A testing beta of QSObook, a free local amateur radio notebook.

This release fixes native browser form submissions being rejected with “Origem não permitida” when starting a round, saving a contact or changing map/language preferences. The previous `no-referrer` policy caused browsers to send `Origin: null`; `same-origin` preserves legitimate form Origins while keeping external referrers private. Null/external Origins, invalid CSRF tokens and cross-site requests remain rejected. No database or dependency changes. Reload open pages after updating.

- QSO rounds with independent participation, QRT status and explicit contact confirmation.
- Safe local callsign lookup, reusable operators, independent notes and preserved contact snapshots.
- Portuguese, English, Spanish and French, light/dark themes and a responsive phone layout.
- Optional OpenStreetMap city map, disabled until explicitly enabled.
- Ready-to-run Linux ARM64 and AMD64 images at `ghcr.io/erickfaria/qsobook`.
- Private-LAN/VPN access with an optional shared password and explicit allowed hostnames.

Download `compose.lan.yaml` for private LAN/VPN access without a password, `compose.auth.yaml` to require a shared password, or `compose.yaml` for loopback-only use. Published Compose assets pin the image digest. Installation, password setup, cold backups and manual updates are explained in the [beta installation guide](https://github.com/erickfaria/QSObook/blob/main/docs/BETA.md).

Images are built and tested natively on both architectures after Go tests, vet, vulnerability checks and JavaScript regressions. Container tests cover authentication, HTTP security, duplicate confirmation, stopped-database backup integrity and persistence after recreation. The publication includes GitHub provenance attestations. This is not a security audit or a guarantee of defect-free software.

Known limits: private/trusted networks only; HTTP does not encrypt passwords or contacts. Use HTTPS/VPN for untrusted networks or remote access. No individual user accounts, automatic updates, in-app backup/restore, ADIF export or supported stable release line yet. Keep offline backups before upgrades. Physical Raspberry Pi testing is the next step; hosted ARM64 tests do not replace it. Without a password, anyone who can reach the published port can read/change the notebook. Network/VPN rules must restrict access. Upgrading from beta.1 preserves the data volume and existing password file; use `compose.auth.yaml` to keep authentication. No database schema changes.
