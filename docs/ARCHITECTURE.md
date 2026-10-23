# Implemented architecture

## Organization

A Go monolith: browser → `net/http` → `html/template` / SQLite operations → a local file. There is one `main` package, without an ORM, SPA or web framework. `main.go` composes configuration, storage and the HTTP server; `web.go` handles HTTP; `store.go` contains persistence and validation. Templates, static assets, migrations, time-zone data and license notices are embedded in the executable. Callsign lookup requests stay local.

## SQLite driver and dependencies

`modernc.org/sqlite v1.36.3` is a BSD-3-Clause `database/sql` driver, a Go port of SQLite 3.49.0 without CGO, with documented Linux ARM64 support. It requires Go 1.23 and preserves the project declaration of Go 1.24.4. The standard library does not provide a SQLite driver. This choice avoids a C compiler and a system SQLite installation at the cost of a larger module tree and first compilation.

The [pinned version documentation](https://pkg.go.dev/modernc.org/sqlite@v1.36.3) includes platform support, licensing, changelog and upstream links. The pinned version is for reproducibility, not a claim that it is the latest release. Updates require reviewing Go compatibility, security notices, tests and migrations before distribution. No vulnerability scanner or external audit is configured.

Runtime transitive dependencies are listed in `go.mod`: `modernc.org/libc`, `mathutil`, `memory`, `golang.org/x/sys`, `x/exp`, `github.com/google/uuid`, `dustin/go-humanize`, `mattn/go-isatty`, `ncruces/go-strftime` and `remyoudompheng/bigfft`. Their permissive BSD/MIT notices are retained in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt), embedded and linked from the footer alongside the project AGPL. Original SQLite is public domain. `go.sum` also records dependencies used by upstream tests/tools. Keep `modernc.org/libc` aligned with the driver; do not update it independently.

Native Linux ARM64 builds and tests are supported without CGO. Current validation runs on Linux AMD64 with an ARM64 cross-build; execution of this revision on Raspberry Pi remains pending.

## Data model

- `stations`: current operator registration, with a normalized unique callsign, name, city, state, country and permanent notes. A callsign identifies a station; no license verification or external lookup is performed.
- `qsos`: confirmed contacts linked to a station, with a UTC timestamp, mode, conversation notes and a historical copy of operator details. Registration edits do not change earlier contacts. Editing one contact changes only its copy; callsign and timestamp stay fixed.
- `participants`: the single current roundtable, with one participation per station, its own mode and conversation notes, and an optional confirmed QSO reference. It persists across restarts and is shared by browser tabs.

New participant, manual QSO and registration forms only complete missing fields for existing callsigns. Saved nonempty values take priority over submitted data, including a stale lookup result. Replacing values requires an explicit edit by ID. Empty fields in a participation edit preserve the registration. Conversation flows never write permanent operator notes. The registration editor permits intentionally clearing fields. Explicit concurrent edits still use the last saved values; no version conflict detection is implemented.

Adding someone does not confirm a contact. A repeated addition preserves the existing participation mode, notes and QSO reference; an explicit edit can change the mode or clear conversation notes. Participation writes and registration updates share one transaction. Editing a removed participation does not recreate it. Removing someone retains the operator and confirmed contacts. A later contact with that operator requires removing and adding them again.

Confirmation uses a transaction to check the QSO reference, insert the historical copy and conversation notes, and link the QSO atomically. Repeated and simultaneous confirmations cannot duplicate a contact for that participation. Later registration or participation edits do not change the snapshot. Manual QSO creation also updates the operator and inserts its contact in one transaction; every manual submission intentionally represents a new contact.

## Migrations and storage

SQLite uses one connection per process, `foreign_keys=ON`, `busy_timeout=5000` and its default rollback journal. No automatic data cleanup is performed. Migrations run in a transaction and use `PRAGMA user_version`; unknown future or invalid versions are rejected.

- `001_initial.sql` creates the original station, QSO and participant tables.
- `002_participant_identity.sql` retains all existing participant rows, IDs and QSO links while rebuilding that table with `INTEGER PRIMARY KEY AUTOINCREMENT`. Deleted participation IDs are not reused, including after restarts. Old record/remove forms return not found instead of affecting a new participation.

The complete migration commits only on success. Failure rolls back both data and schema changes. Migration tests cover retained operators, historical contacts, notes, links, ID sequence persistence, SQLite integrity, foreign keys and failure rollback. There is no automatic downgrade: copy the stopped database before upgrading and restart the updated application so old forms receive a new CSRF token.

The main database file is created with `0600` permissions and new directories with `0700`. Existing directory permissions are not changed. Main-file symlinks and non-regular files are rejected; use a trusted private directory. There is no encryption at rest. The default location is the user data directory; `-db` overrides it. Databases are never served as static files.

## Time, queries and validation

Timestamps come from the host clock and are stored in UTC with fixed-width nanosecond precision, allowing consistent textual sorting. `time/tzdata` embeds IANA time-zone data. Display uses `-timezone`, defaulting to `America/Sao_Paulo`. Manual timestamp editing is not available.

Callsigns accept 1–32 ASCII characters: letters, numbers, `/`, `.` and `-`, starting with a letter or number. Names and location fields allow 120 characters each; notes allow 4,000. Mode is required free text up to 40 characters, with suggested values in a `datalist`. Parameterized searches use `instr`, so SQL wildcard characters have no special meaning. Name searches use SQLite ASCII case folding and distinguish accents. Listings return at most 200 results per query; pagination remains a future improvement.

## HTTP and UI

The server accepts loopback IPs only, defaulting to `127.0.0.1:8080`. External Host headers are rejected to reduce DNS rebinding risk. Writes use POST, a cryptographically random per-run CSRF token, constant-time comparison and Origin verification when present. The token does not authenticate users; old forms need reloading after a restart. Form bodies are limited to 32 KiB, with HTTP header and timeout limits. Mutable values come from the POST body.

`html/template` escapes values. CSP restricts resources to the local origin and blocks frames, inline scripts and external form submission. Responses set `nosniff`, `no-referrer`, frame protection and `Cache-Control: no-store`. Internal errors and form contents are not logged or shown to users. Standard server connection errors may appear in operational logs without application records. No security exception was introduced for screenshots or the integrated browser.

The main UI is a table with semantic HTML, labels and visible keyboard focus. The quick form and inline editors share compact fields; country and conversation notes use `details`. Mode and country are secondary cell information. Permanent and conversation notes are labeled separately. Edit links retain a separate-page fallback without JavaScript. Inclusion validation retains the submitted form and the participant table.

CSS confines horizontal scrolling to the table. JavaScript debounces local `/lookup` requests, aborts older requests and checks a version counter after both the response and JSON parsing. This ignores stale responses and errors, including A → B → A changes. Edited fields are preserved and empty lookup values never clear fields. Drafts stay in memory per normalized callsign, without browser storage; blank callsigns do not revive an earlier unassigned draft. Lookup never copies permanent notes into a conversation.

JavaScript prevents repeated submission while a form is pending and restores controls on back navigation. SQLite provides the confirmation integrity guarantee; the UI guard is supplementary. Without JavaScript, registration, editing, queries and confirmation still work with manual entry. `tests/app.test.cjs` uses only built-in Node.js modules and a simulated DOM to test request ordering and form behavior.

Authentication, user isolation and LAN access are not implemented. The host clock, account security and private storage are deployment responsibilities. TLS, proxying or network exposure require a separate access-model decision. Maps, geocoding and external services are not implemented.
