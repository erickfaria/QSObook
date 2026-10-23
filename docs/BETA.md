# Install and update the QSObook beta

This beta is intended for personal testing, including a 64-bit Raspberry Pi running Debian. Ready-made images support **Linux ARM64 and AMD64**. No Go compiler, Node.js or Docker Hub account is needed on the Pi. Public images can be pulled anonymously from GitHub Container Registry.

Use the official [Docker Engine installation instructions](https://docs.docker.com/engine/install/debian/) and [Compose plugin guide](https://docs.docker.com/compose/install/) if these tools are missing. Check `docker version` and `docker compose version` first. Use a maintained Docker Engine release 28 or newer. This project does not install host packages, change your firewall or modify other services.

## Install on Tortuga for your private LAN

The browser address will be **http://tortuga.local:8088** or **http://YOUR_PI_IP:8088**. Choose a free port if another service already uses 8088. The application stays in one container; SQLite stays in a local Docker volume.

This mode is for a **trusted home network**. It uses the browser's native username/password prompt. HTTP does not encrypt the password or notebook traffic: do not use it on guest/public Wi-Fi, forward the port through your router or expose it to the internet. Use HTTPS/VPN before accessing it through an untrusted network. A container does not replace access control, encryption or backups.

### 1. Download a specific release

Run these commands on Tortuga. Replace the version below when installing a later beta:

```sh
mkdir qsobook-beta
cd qsobook-beta
version=v0.1.0-beta.1
for file in compose.yaml compose.lan.yaml image-digest.txt image-manifest.json SHA256SUMS; do
  curl --fail --location --output "$file" \
    "https://github.com/erickfaria/QSObook/releases/download/$version/$file" || break
done
sha256sum --check SHA256SUMS
```

**Continue only if every download succeeds and the checksum check passes.** The Compose files pin the exact published image digest, so installing an older release cannot silently install a newer beta. Checksums detect incomplete/changed downloads; provenance is described below.

Use one fixed directory and Compose project for this notebook. The project name is `qsobook`, preserving the existing `qsobook_data` volume if you already used the source-build Compose deployment. Stop that deployment first; do not run two instances on the same notebook. A standalone binary's database is not automatically imported. Never delete an existing notebook or volume to make this installation work.

### 2. Create the private access password once

```sh
mkdir -m 700 secrets
(umask 077; set -C; openssl rand -hex 24 > secrets/access-password.txt)
chmod 444 secrets/access-password.txt
cat secrets/access-password.txt
```

Save the displayed password in your password manager. The username is **`qsobook`**. Do not send the password through chat or commit it to Git. Run this only for a new installation; the command refuses to overwrite an existing file. The 0700 parent directory keeps the file private on the host; the read-only file must be readable by the container's non-root user. Compose mounts only this file as a secret, never the entire directory.

The app refuses passwords shorter than 16 bytes or missing password files. New network hostnames cannot be enabled without authentication. Passwords are not stored in the database or container image. Changing the password file and recreating the container changes access without rewriting contacts. Browser Basic authentication has no application logout button; close the browser session to clear cached credentials.

### 3. Set the LAN address and allowed names

Find Tortuga's LAN IPv4 address in your existing router/server settings. `hostname -I` on Tortuga may show several interfaces; choose the address of your home network.

Create a `.env` file in this directory, replacing **both occurrences** of the example address:

```dotenv
QSOBOOK_BIND_ADDRESS=192.168.1.50
QSOBOOK_PORT=8088
QSOBOOK_ALLOWED_HOSTS=tortuga.local:8088,192.168.1.50:8088
```

Bind to Tortuga's specific LAN address, not `0.0.0.0`. The allowed entries are exact `host:port` values, not URLs or wildcards. If you choose a different port, update it in every entry. Reserve a stable LAN address in your existing network configuration if you need it; QSObook does not configure DHCP or DNS. `tortuga.local` must already resolve on your device; use the IP otherwise.

### 4. Start the image

```sh
docker compose -f compose.lan.yaml config --quiet
docker compose -f compose.lan.yaml pull
docker compose -f compose.lan.yaml up -d
docker compose -f compose.lan.yaml logs --tail=30 qsobook
```

Open **http://tortuga.local:8088** in your computer/phone browser on the same trusted network. Enter username `qsobook` and the password saved above. Start a round, add a fictitious callsign, confirm a QSO, reload and check the notebook. Test the flow before entering records you want to keep.

The container runs without root, drops capabilities and uses a read-only root filesystem. SQLite is in the named `qsobook_data` volume. Changing the image or recreating the container keeps that volume. Map requests remain disabled until explicit opt-in.

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
cp -Rp secrets "$backup_dir/"
docker compose -f compose.lan.yaml start
```

Continue to updating only after all copies succeed. Keep this backup private and store another offline copy wherever you choose. A named volume is not a backup. CI verifies that this stopped SQLite copy passes integrity/foreign-key checks and contains confirmed contacts. There is no in-app restoration tool yet: validate a separate working copy before relying on a backup, and do not overwrite your only notebook. Keep the original backup untouched. Restoring/moving a notebook into a new volume and UID ownership handling are still manual operations.

## Update when you decide

Changing code on GitHub does not change Tortuga. A maintainer must run **Publish beta** on `main` with a new version; it validates, builds, tests and publishes a release. You then choose when to install it.

After a successful stopped backup, download the files for the **new** version into a separate temporary directory and check `SHA256SUMS`, exactly as in installation step 1. Review its release notes. Replace only `compose.lan.yaml` with that release's file, preserving your `.env`, `secrets/`, project name and volume. Then run:

```sh
docker compose -f compose.lan.yaml pull
docker compose -f compose.lan.yaml up -d
docker compose -f compose.lan.yaml logs --tail=30 qsobook
```

There is no Watchtower or automatic deployment. The optional mutable `:beta` image tag follows the latest successfully published beta, but release downloads use digest-pinned images by default. Do not assume changing the image back can reverse database migrations; keep the pre-update database copy and release files together. No automatic schema downgrade is provided.

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

This verifies a recorded build identity; it is not a vulnerability-free guarantee or an external security audit. CI uses supported Go 1.27.3 and pinned `govulncheck` v1.9.0 to reject reachable Go vulnerabilities. Embedded upstream components and operational security still require maintenance. The beta has hosted native ARM64 tests; physical Raspberry Pi validation is pending your testing.
