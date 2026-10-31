# Install and update QSObook

This guide installs **v0.2.0 (schema 8)**, including optional RadioID autofill. Back up before upgrading an existing notebook: v0.1.0 cannot open schema 8. For source builds, use [DOCKER.md](DOCKER.md). A code merge does not update your server; digest-pinned Compose files remain on their selected release until you replace them.

QSObook v0.2.0 is a stable release for personal local/private-network use, including a 64-bit Raspberry Pi running Debian. Ready-made images support **Linux ARM64 and AMD64**. No Go compiler, Node.js or Docker Hub account is needed on the Pi. Public images can be pulled anonymously from GitHub Container Registry.

Use the official [Docker Engine installation instructions](https://docs.docker.com/engine/install/debian/) and [Compose plugin guide](https://docs.docker.com/compose/install/) if these tools are missing. Check `docker version` and `docker compose version` first. Use a maintained Docker Engine release 28 or newer. This project does not install host packages, change your firewall or modify other services.

## Install on Tortuga for your private LAN

The browser address will be **http://tortuga.local:18088** or **http://YOUR_PI_IP:18088**. Choose a free port if another service already uses 18088. The application stays in one container; SQLite stays in a local Docker volume.

This mode is for a **trusted private network or VPN**, such as an existing Tailscale network. Password authentication is optional. Without a password, anyone who can reach the published port can read and change the notebook; network/VPN access rules must provide access control. Do not expose this port to the internet. HTTP does not encrypt notebook traffic; use your VPN for remote access.

### 1. Download a specific release

Run these commands on Tortuga. Replace the version below when installing a later release:

```sh
mkdir qsobook
cd qsobook
version=v0.2.0
for file in compose.yaml compose.lan.yaml compose.auth.yaml image-digest.txt image-manifest.json SHA256SUMS; do
  curl --fail --location --output "$file" \
    "https://github.com/erickfaria/QSObook/releases/download/$version/$file" || break
done
sha256sum --check SHA256SUMS
```

**Continue only if every download succeeds and the checksum check passes.** The Compose files pin the exact published image digest, so installing an older release cannot silently install a newer release. Checksums detect incomplete/changed downloads; provenance is described below.

Use one fixed directory and Compose project for this notebook. The project name is `qsobook`, preserving the existing `qsobook_data` volume if you already used the source-build Compose deployment. Stop that deployment first; do not run two instances on the same notebook. A standalone binary's database is not automatically imported. Never delete an existing notebook or volume to make this installation work.

### 2. Set the LAN address and allowed names

Find Tortuga's LAN IPv4 address in your existing router/server settings. `hostname -I` on Tortuga may show several interfaces; choose the address of your home network.

Create a `.env` file in this directory, replacing **both occurrences** of the example address:

```dotenv
QSOBOOK_BIND_ADDRESS=192.168.1.50
QSOBOOK_PORT=18088
QSOBOOK_ALLOWED_HOSTS=tortuga.local:18088,192.168.1.50:18088
```

Bind to Tortuga's specific LAN address, not `0.0.0.0`. The allowed entries are exact `host:port` values, not URLs or wildcards. If you choose a different port, update it in every entry. Reserve a stable LAN address in your existing network configuration if you need it; QSObook does not configure DHCP or DNS. `tortuga.local` must already resolve on your device; use the IP otherwise.

### 3. Start the image

```sh
docker compose -f compose.lan.yaml config --quiet
docker compose -f compose.lan.yaml pull
docker compose -f compose.lan.yaml up -d
docker compose -f compose.lan.yaml logs --tail=30 qsobook
```

Open **http://tortuga.local:18088** in your computer/phone browser on the same trusted network. No password prompt is shown. Open QSO mode in the main navigation, start without a roundtable and save a fictitious station without confirmation. Confirm a contact through its row action or the explicit checkbox, with a mode/band. Reload and check the notebook. Test the flow before entering records you want to keep.

The container runs without root, drops capabilities and uses a read-only root filesystem. SQLite is in the named `qsobook_data` volume. Changing the image or recreating the container keeps that volume. Map requests remain disabled until explicit opt-in.

## Use your existing Tailscale VPN

To publish only on the Pi's existing Tailscale interface, read its address with `tailscale ip -4` and use that address in `.env`, for example:

```dotenv
QSOBOOK_BIND_ADDRESS=100.101.102.103
QSOBOOK_PORT=18088
QSOBOOK_ALLOWED_HOSTS=100.101.102.103:18088
```

Replace the example with your actual Tailscale IPv4 address. Run the same Compose start commands, then open `http://YOUR_TAILSCALE_IP:18088` from a device authorized on your VPN. A LAN-only binding does not also publish on the VPN interface. If using an existing MagicDNS name, add that exact name and port to `QSOBOOK_ALLOWED_HOSTS`; do not assume `tortuga.local` resolves over the VPN. QSObook does not install or configure Tailscale. See the [Tailscale CLI reference](https://tailscale.com/docs/reference/tailscale-cli).

## Optional password

Choose `compose.auth.yaml` instead of `compose.lan.yaml` to require a shared password. Do not run both profiles simultaneously. Both use the same project and data volume.

```sh
mkdir -m 700 secrets
(umask 077; set -C; openssl rand -hex 24 > secrets/access-password.txt)
chmod 444 secrets/access-password.txt
cat secrets/access-password.txt
```

Save the displayed password in your password manager. The username is **`qsobook`**. Do not send the password through chat or commit it to Git. Run this only for a new installation; the command refuses to overwrite an existing file. The 0700 parent directory keeps the file private on the host; the read-only file must be readable by the container's non-root user. Compose mounts only this file as a secret, never the entire directory.

The app refuses passwords shorter than 16 bytes or missing password files. Passwords are not stored in the database or container image. Changing the password file and recreating the container changes access without rewriting contacts. Browser Basic authentication has no application logout button; close the browser session to clear cached credentials.

Use `docker compose -f compose.auth.yaml pull` and `docker compose -f compose.auth.yaml up -d` after creating `.env` as above. This profile requires a valid password file; missing or invalid files cause startup to fail. Username: `qsobook`.

To switch an existing installation to no password, use the current release’s `compose.lan.yaml` and recreate the service. To require authentication, use `compose.auth.yaml` with a valid password file. Keep your `.env`, project and data volume. Unused `secrets/` files may remain in place; switching access profiles does not delete notebook records or password files.

## Stop and resume

```sh
docker compose -f compose.lan.yaml stop
docker compose -f compose.lan.yaml start
```

Do not use `down -v`, remove the data volume or prune volumes when updating: those actions delete your notebook. Avoid changing the project name or running multiple notebooks under the same project accidentally.

## Back up before updating

Stop every process that uses the notebook, then copy the entire data directory, including any SQLite journals. Use a new private directory for every backup:

```sh
mkdir -p backups
backup_dir="backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$backup_dir"
docker compose -f compose.lan.yaml stop
docker compose -f compose.lan.yaml cp qsobook:/data "$backup_dir/"
cp -p compose.lan.yaml .env "$backup_dir/"
if [ -d secrets ]; then cp -Rp secrets "$backup_dir/"; fi
docker compose -f compose.lan.yaml start
```

Continue to updating only after all copies succeed. Keep this backup private and store another offline copy wherever you choose. A named volume is not a backup. CI verifies that this stopped SQLite copy passes integrity/foreign-key checks and contains confirmed contacts. The application provides backup/restore under **Preferences → Data**. See the [data guide](DATA.md) for validated snapshots, full-notebook replacement, recovery copies and upload limits. Keep the original backup untouched. Restoring/moving a notebook into a new volume and UID ownership handling are still manual operations.

## Database compatibility

**v0.2.0 uses schema 8.** v0.1.0 and beta 5 use schema 7. The first v0.2.0 start migrates a supported older notebook transactionally, preserving stations, QSOs, participant IDs/links, QRT status, round labels, replay keys and map caches. RadioID consent starts disabled; enabling it is separate from map consent. No existing data is deleted by this migration.

Back up before updating. **v0.1.0 and all earlier versions cannot open schema 8.** Returning to them requires a separately preserved compatible pre-upgrade backup, not simply an older image. There is no automatic schema downgrade.

The v0.2.0 backup/restore UI accepts exact schema-5/6/7/8 SQLite snapshots up to 64 MiB. A schema-5/6/7 upload is upgraded in an isolated staging copy. Restoration replaces the entire notebook only after preview/confirmation and a validated pre-restore recovery snapshot. It is not a notebook merge. Schema-8 backups restore their saved RadioID consent and retry state. See [backup, Trash and ADIF procedures](DATA.md).

## Update when you decide

Changing code on GitHub does not change Tortuga. A maintainer must run **Publish release** on `main` with a new version; it validates, builds, tests and publishes a release. You then choose when to install it.

After a successful stopped backup, download the files for the **new** version into a separate temporary directory and check `SHA256SUMS`, exactly as in installation step 1. Review its release notes. Replace only `compose.lan.yaml` with that release's file, preserving your `.env`, `secrets/`, project name and volume. Then run:

```sh
docker compose -f compose.lan.yaml pull
docker compose -f compose.lan.yaml up -d
docker compose -f compose.lan.yaml logs --tail=30 qsobook
```

There is no Watchtower or automatic deployment. The mutable `:stable` and `:latest` image tags follow the most recently published stable release; `:beta` remains a separate prerelease channel. Versioned release downloads use digest-pinned images by default. Do not assume changing the image back can reverse database migrations; keep the pre-update database copy and release files together. No automatic schema downgrade is provided.

## Local-only alternative

Use the downloaded `compose.yaml` instead of the LAN file:

```sh
docker compose pull
docker compose up -d
```

It publishes only at `http://127.0.0.1:8080` on the Docker host, without password authentication. It does not accept connections addressed to Tortuga's LAN name/IP. Access it from another trusted computer through your existing SSH tunnel, as described in [DOCKER.md](DOCKER.md). Do not run both alternatives at once for the same notebook.

## Verify the published source

Each release records the image digest and source commit. Publication runs only from `main`, after both native architecture test jobs succeed. Images are tested before upload. GitHub's temporary repository token is scoped to publication jobs; PR validation has no publishing permission. Actions are pinned by commit.

The workflow creates a GitHub provenance attestation for the multi-architecture digest. With a current GitHub CLI, verify the exact image reference from `image-digest.txt`:

```sh
gh attestation verify "oci://$(cat image-digest.txt)" --repo erickfaria/QSObook
```

This verifies a recorded build identity; it is not a vulnerability-free guarantee or an external security audit. CI uses supported Go 1.27.3 and pinned `govulncheck` v1.9.0 to reject reachable Go vulnerabilities. Embedded upstream components and operational security still require maintenance. Release validation uses hosted native ARM64 tests and real containers. The maintainer has reported successful beta-5 use on a Raspberry Pi; each deployment should still verify its own records, map and backup flows.

## Building from source instead

v0.2.0 and current source both use schema 8. Save a consistent backup and retain the stopped deployment files before changing executables/images. Build and test against a separate notebook first; [DOCKER.md](DOCKER.md) describes the source workflow. Keep the original Compose project, volume and access settings when intentionally upgrading the real notebook. Future source changes may introduce additional migrations; always check their compatibility notes.

RadioID is off after migration. Enable it explicitly in Preferences if wanted. A schema-8 backup includes its consent, pending lookups and origin metadata; see [DATA.md](DATA.md). Rolling back to v0.1.0 requires the preserved schema-7 backup. Do not downgrade the only live copy, delete its volume or edit its schema version.
