# Security policy

## Status and support

QSObook has an initial functional implementation for personal local use. There is no supported release line or guaranteed response time yet. There is no authentication or isolation between local users. Do not expose it directly to the internet. The listen address is restricted to loopback; do not bypass that restriction with a proxy before discussing authentication, TLS and access control.

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
- Form bodies limited to 32 KiB, bounded headers and HTTP timeouts, and errors without internal details.
- CSP for local resources, frame blocking, `nosniff`, `no-referrer` and no-store responses. An opted-in home page additionally permits images from the configured tile origin; tile images send only the application origin as Referer.
- A local database file with `0600` permissions and new directories with `0700`; databases, configuration and backups are never static assets.
- No telemetry or external callsign lookup. Maps are disabled by default and require opt-in. Enabled maps request the viewed tile area; explicit geocoding requests send geographic fields only, with local caching and request limits.

The CSRF token does not authenticate users. Processes and people with access to the OS account or local port can access contacts. Reload old forms after restarting the server. Use a private directory: existing directory permissions are not changed. There is no database encryption or in-app backup. Stop every database user before manual copies and test restoration separately. External license, policy and source links are only requested when opened. Maps use the disclosed providers only after opt-in. The tile provider receives the browser IP and view area; the geocoder receives the server IP and city/state/country. Disable the map and reload other already-open map tabs to stop their future tile requests. New geocoding requests are rejected immediately after disabling. No contacts or notes are sent to those providers.

Dependencies are pinned in `go.mod`/`go.sum`; this does not replace security notice review and updates. There is no external audit or configured vulnerability scanner. See [architecture](docs/ARCHITECTURE.md) for the embedded SQLite version and storage limits.
