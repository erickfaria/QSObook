# Backups, recovery and ADIF

These features are available from beta 5 (database schema 6). Open **Preferences → Data**. The interface is available in Portuguese, English, Spanish and French. All transfer operations are local and require an explicit action. Nothing is synchronized or uploaded to another service.

## Download a complete backup

Choose **Download backup**. The application uses SQLite's [VACUUM INTO](https://www.sqlite.org/lang_vacuum.html#vacuuminto) to create a consistent standalone `.db` snapshot, including when the live notebook uses WAL. It validates the snapshot before sending it as an attachment. The live database is not copied with ordinary file operations or stopped for this action.

A backup contains operator registrations, historical QSOs, rounds, participation links, trash, map preferences, home city and coordinate caches. It does **not** contain Docker configuration, `.env`, password files or application/browser settings outside SQLite. Preserve deployment files separately. The backup is not encrypted: it contains personal notebook data and must be stored privately. Keep an offline copy outside the Pi; a copy in its Docker volume is not protection against disk failure.

Downloaded backups and upload staging files are private temporary files alongside the configured database, outside the static file tree. They are removed after transfer/cancellation or preview replacement. Pending previews expire after 15 minutes and are cleared when revisiting Data; graceful shutdown also removes them. An abrupt process or machine crash can leave private `.qsobook-transfer-*`/`.qsobook-backup-*` directories. Do not expose the data directory as a web root. Previewing a second file replaces the first preview for this instance.

## Restore a backup

1. Choose **Restore backup**, select a consistent SQLite backup and choose **Validate and preview**. The limit is **64 MiB**. Only the exact QSObook schemas **5 and 6** are accepted; earlier/newer versions or foreign/modified schemas are rejected. A schema-5 upload is migrated in its isolated staging copy.
2. Review the number of stations, contacts, rounds and trashed items. Restoration replaces the **entire** current notebook, including map preferences and trash. It does not merge two notebooks.
3. Check the confirmation box and confirm. If the notebook changed since the preview, send the file again. Cancel discards the staged upload.
4. The app creates and validates a durable **pre-restore recovery snapshot** in `backups/` beside the live database before replacing any rows. If that step fails, restoration stops. Table replacement happens in a single SQLite transaction, with foreign-key checks; the open database file is never renamed or swapped.
5. Return to Data to download the recovery copy if needed. These copies are not automatically removed; retain an external copy and manage server disk space. To undo a completed restore, upload the downloaded recovery copy, review it and confirm another restore.

The validator checks schema definitions, SQLite integrity, foreign keys and application fields. Uploads cannot provide SQL commands, triggers, views, arbitrary server paths or new tables. Files are streamed to the private data directory after CSRF verification; a 64 MiB upload is not buffered in RAM or spilled into the container's 16 MiB `/tmp`.

After restoration, previously open forms expire, preventing stale IDs from modifying the restored notebook. Refresh other tabs. Run a single QSObook process per notebook; direct writes by other programs during maintenance are unsupported. Explicit ordinary edits still use last-write-wins behavior.

Do not upload a raw `.db` copied while SQLite is active: its WAL/journal may contain required data. Use the application backup or the stopped full-directory procedure in the [beta guide](BETA.md). Restoration is deliberately strict and bounded; retain your original backup if it is refused. There is no automatic schema downgrade. Before upgrading beta 4 to beta 5, retain a pre-upgrade schema-5 copy: beta 4 cannot open schema 6.

## Recoverable deletion

Open an operator's edit page or a contact's details and choose **Delete station** or **Delete QSO**. A separate confirmation page explains the effect. Nothing changes until confirmation.

- A deleted station is hidden from the registry and local autofill. Its callsign stays reserved; restore it from **Preferences → Trash** before recording new contacts. Historical QSOs and round participations remain intact.
- A deleted QSO is hidden from history, monthly/total counts and map aggregation. Its station is unchanged. Replay keys and participant links remain, so an old confirmation cannot recreate it.
- Restore either item from Trash. Restoring a QSO does not reactivate an archived station, and restoring a station does not restore separately deleted QSOs.

There is no permanent purge or automatic trash expiry. Trashed data remains in full backups. An old roundtable may link to a trashed QSO; restore it through Trash to open its details again.

## Search and date ranges

Records, Stations and Notebook now show **50 results per page**, with previous/next links that retain filters. Search matches callsign, name or city, ignoring case, common Latin accents and decomposed combining marks. The saved spelling is unchanged. This is not full Unicode normalization or linguistic collation for every alphabet.

Notebook has inclusive **From/To** calendar dates in the configured application time zone. Internally they become UTC boundaries. Existing city/mode/band filters from the map are retained while searching or changing pages. There is no silent 200-result cap in these screens.

## Export contacts as ADIF

Choose **Export ADIF** to download all confirmed, non-trashed QSOs as `.adi`, without a 200-row limit. ADIF exports contact snapshots, **not** permanent operator notes, live participations or trash; use a full backup to preserve the complete notebook.

The implementation uses the [ADIF 3.1.6 ADI specification](https://adif.org.uk/316/ADIF_316.htm). ADI is ASCII. Standard fields include `CALL`, UTC `QSO_DATE`/`TIME_ON`, supported `MODE`/`SUBMODE`/`BAND`, `NAME`, `QTH`, `STATE`, `COUNTRY` and `COMMENT`. DMR/C4FM/D-Star use `DIGITALVOICE` with the appropriate submode. Band wavelengths are converted to ADIF spelling, such as `70cm`.

For interoperable ASCII fields, common accents are transliterated and other non-ASCII characters use `?`. **The original UTF-8 text, timestamp precision, custom mode/band and optional round name are also preserved** as base64-encoded JSON in the application-defined `APP_QSOBOOK_DATA` field. QSObook reimports that field losslessly; another program may ignore it. Unknown/custom modes or bands have no invented standard equivalent and are omitted from the corresponding standard field. EchoLink has no inferred RF mode. Verify an export in the receiving program before relying on it; it may reject incomplete mode/band records.

## Import contacts from ADIF

Choose **Import ADIF** and preview an **ASCII ADI** file, at most **16 MiB and 10,000 records**. ADX/XML and non-ASCII ADI are explicitly rejected rather than guessed or decoded incorrectly. A normal imported record needs `CALL`, `QSO_DATE`, `TIME_ON` and `MODE` (or a submode); band is optional. Four- and six-digit UTC times are accepted. A QSObook export can recover its original data from `APP_QSOBOOK_DATA`.

The whole file is parsed and validated before any registration or QSO is written. An invalid record rejects the file; fix it and submit again. The preview shows new contacts, skipped duplicates, records blocked by archived stations, a sample of the first 20 new contacts and every unsupported field name. **Unsupported fields are discarded only after you confirm the preview.** RST, frequency, grid squares, QSL information and other fields without a QSObook counterpart are currently unsupported. Keep the source file.

Duplicates are identified conservatively by callsign, UTC second, case-insensitive mode and normalized band, including existing trashed QSOs and earlier records in the same file. Names/notes are not used to overwrite a matching contact. An original import identity also stays reserved after later QSO edits. Two legitimate contacts with the same identity will be treated as duplicates; review skipped counts. Imports add contact snapshots and fill missing registration details, preserve existing registration values/permanent notes and never geocode in a background batch. The commit is atomic and checks duplicates again. Repeated confirmation does not duplicate contacts.

## Map corrections and installed version

Expand the home screen's city list and choose **Correct location**, or follow the same link under **My location**. Save finite latitude/longitude within their legal ranges to correct the shared local city cache offline. This changes the city's map point and matching home marker, not historical location text or registrations.

**Query again** explicitly retries a stored city, including a previous no-match. It requires map opt-in and retains geographic-only requests, request serialization/cooldowns and provider limits. Failure or an empty response preserves an existing point. It never runs on page load or typing. Imported historical cities can be located individually through the same controls.

**Preferences → About** shows the installed app version, source revision and schema version. Release images embed their beta version; ordinary source builds are marked `dev`. `qsobook -version` prints build information without opening a database. About links to releases and update instructions, with no automatic network version checks. Updating GitHub does not update your Pi.
