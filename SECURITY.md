# Security policy

## Status and support

QSObook v0.1.0 is the first stable release for personal local/private-network use. Earlier betas are superseded. This personal project offers no guaranteed response time or external security-audit claim. Private-LAN/VPN mode optionally supports one shared access password; it does not provide per-user accounts, roles or isolation. Local-only mode remains unauthenticated. Do not expose it directly to the internet. The standalone listener is restricted to loopback. Docker uses an explicit `-container` opt-in for its internal bridge listener; the supplied Compose file publishes only on host `127.0.0.1`. Host, Origin, CSRF and frame protections remain in place. The separate LAN Compose configuration requires a specific LAN/VPN IP and exact allowed host:port entries. Passwords are optional: without one, anyone who can reach the port can access and modify the notebook. The deployment network/VPN controls access. Use `compose.auth.yaml` to require a shared password secret. These profiles are for a trusted private network/VPN only. HTTP Basic authentication does not encrypt credentials or notebook traffic: use HTTPS/VPN for untrusted networks and remote access. Do not expose either mode directly to the internet or add an unreviewed proxy. Docker hosts and other containers on the same container network are trusted local access; the optional password applies to all application routes, including static assets and lookup. All authenticated clients share the same notebook. See the [Docker guide](docs/DOCKER.md).

## Reporting a vulnerability

Do not publish personal data, credentials, real databases or exploitation details in public issues or pull requests.

In the repository [Security tab](https://github.com/erickfaria/QSObook/security), look for **Advisories → Report a vulnerability**. Use that private form **if available**. Availability depends on the owner enabling it; this document does not confirm that it is enabled.

If the private form is unavailable, the project has no other confirmed private reporting channel. Open an issue requesting a private channel only, without sensitive technical details, and wait for the maintainer before sending the full report. No email address is presumed.

Through the agreed private channel, include the affected version/commit, environment, minimal reproduction using fictitious data, expected impact and possible mitigation. Coordinate public disclosure after the maintainer has evaluated and addressed the report. This personal project offers no response-time commitment or announced bug bounty.

The owner may enable private reports manually under **Settings → Advanced Security → Private vulnerability reporting**. See the [official GitHub instructions](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository). Application changes do not modify these remote settings.

## Project requirements

Keep data local, avoid unnecessary exposure and never introduce telemetry. Fix defects with tests proportional to their risk and document upgrade actions without exposing user records.

## Implemented controls and limits

- Parameterized SQL, server-side validation and automatically escaped templates.
- Writes restricted to POST, a per-run CSRF token, Origin verification and rejection of external Host values.
- Ordinary form bodies limited to 32 KiB, bounded headers/timeouts and errors without internal details. File transfers use separate explicit size limits and streamed private staging after CSRF verification.
- CSP for local resources, frame blocking, `nosniff`, `Referrer-Policy: same-origin` and no-store responses. The referrer policy preserves the Origin of native form submissions while omitting Referer on cross-origin requests. An opted-in home page additionally permits images from the configured tile origin; tile images send only the application origin as Referer.
- A local database file with `0600` permissions and new directories with `0700`; databases, configuration and backups are never static assets.
- No telemetry or external callsign lookup. Maps are disabled by default and require opt-in. Enabled maps request the viewed tile area; explicit geocoding requests and opted-in location/confirmed-contact saves send geographic fields only, with local caching, a three-second request deadline and rate limits. Rendering, autocomplete and startup do not geocode cities.

The CSRF token does not authenticate users. Processes and people with access to the OS account or local port can access contacts. Reload old forms after restarting the server. Use a private directory: existing directory permissions are not changed. There is no database encryption. The in-app backup creates and validates a consistent SQLite snapshot; downloaded files must be stored privately. Stop every database user before ordinary manual copies. Restoration validates an isolated exact-schema upload, requires preview/confirmation and a durable recovery copy, and transactionally replaces rows while invalidating old forms. See [data recovery](docs/DATA.md). External license, policy and source links are only requested when opened. Maps use the disclosed providers only after opt-in. The tile provider receives the browser IP and view area; the geocoder receives the server IP and city/state/country. Disable the map and reload other already-open map tabs to stop their future tile requests. New geocoding requests are rejected immediately after disabling. No contacts or notes are sent to those providers.

Dependencies are pinned in `go.mod`/`go.sum`; this does not replace security notice review and updates. There is no external security audit. CI uses pinned govulncheck v1.9.0 with supported Go 1.27.3 to reject reachable Go vulnerabilities; this does not cover every upstream component or operational risk. Release images include source revision labels, exact digests and GitHub provenance attestations. See [architecture](docs/ARCHITECTURE.md) for the embedded SQLite version and storage limits.

QSO entry and the compatible Records endpoint use random per-form keys and a partial unique SQLite index to prevent replayed confirmations. Transactions check a key before any operator update. Reusing a key for a different callsign is rejected. Operator-only saves create no QSO. Legacy direct `POST /contact` clients retain their previous new-contact semantics. Upgrades from older schemas require a consistent pre-upgrade backup for rollback; older application versions refuse the newer schema. See [INSTALL.md](docs/INSTALL.md).

Restore and ADIF upload routes require the CSRF part before the file, reject unexpected fields and enforce file limits (64 MiB and 16 MiB respectively). User-supplied filenames and paths are never used as server destinations. Unknown/modified database schemas, triggers and views are rejected before migrations. Only validated recovery snapshots have download routes; none are static assets. Imports disclose discarded fields and conservative duplicate handling before confirmation, and never contact a geocoder. Trashed items remain in backups; deletion is recoverable, not secure erasure. Use one application process per notebook during maintenance.
