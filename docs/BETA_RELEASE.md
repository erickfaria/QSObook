A testing beta of QSObook, a free local amateur radio notebook.

- A unified Records screen: register an operator and optionally confirm a QSO without starting a roundtable. Confirmation starts unchecked; country has no default.
- Local country dropdowns in four UI languages, common modes and wavelength bands, custom values and an optional roundtable name.
- Transactional form replay protection, automatic timestamps, independent notes and preserved historical snapshots.
- A smaller monthly/total/city summary; optional cached map, approximate home marker, mode colors and local mode/band filters.
- Opt-in city geocoding when saving a location or confirmed contact. User-triggered geographic queries only, caching, bounded requests and rate limits; provider failure never discards a saved contact. No startup batches or external autocomplete.
- More compact rounded mobile forms; original SVG branding; bundled SIL OFL Atkinson Hyperlegible fonts and Unicode-licensed country data. No new runtime dependencies or remote fonts.
- Existing roundtables, participants, QRT and history remain accessible. Passwords remain optional for private LAN/VPN use; Host/Origin/CSRF/frame protections are unchanged.

**Back up before upgrading:** this release migrates SQLite schema 4 to 5. Existing records, IDs, links and cache are retained. Band starts blank for older contacts; home city is empty and geocoding-on-save is disabled until selected. An older beta cannot open the upgraded notebook. Keep the stopped pre-upgrade backup and earlier release files for rollback; do not delete or recreate the data volume.

Download `compose.lan.yaml` for LAN/VPN access without a password, `compose.auth.yaml` for optional shared-password access, or `compose.yaml` for loopback-only access. Images support Linux ARM64 and AMD64, and release Compose assets pin their exact digest. Read the [installation, backup and update guide](https://github.com/erickfaria/QSObook/blob/main/docs/BETA.md). Reload open browser pages after updating.

Maps require opt-in and internet for tiles/geocoding; core recording and lookup remain local/offline. Follow the [Nominatim policy](https://operations.osmfoundation.org/policies/nominatim/) and manage aggregate usage across instances. Unmapped older cities need an explicit location action; no-match results are cached. Independent concurrent edits still use last-write-wins. No individual accounts, automatic deployment, in-app backup/restore or ADIF export yet. Without a password, network/VPN rules must restrict access to the port. This beta is not an external security audit.
