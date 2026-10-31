# Install QSObook with Docker Compose

For a ready-made stable image and private-LAN access on Tortuga, use the [installation guide](INSTALL.md). This guide builds the image on your own computer. You do **not** need a Docker Hub account or to publish anything. The first build downloads the official Go build image and the pinned Go modules. Subsequent use works offline, except for maps explicitly enabled by you.

## What the pieces mean

- **Image:** the packaged application, built from this repository's `Dockerfile`.
- **Container:** a running instance of that image.
- **Volume:** the local storage containing your SQLite database, separate from the container.
- **Compose:** the `compose.yaml` recipe that starts the application with its port and storage settings.

There is one application container and one data volume. SQLite runs inside the Go application; no separate database server is needed.

## 1. Check the prerequisites

Use Linux AMD64 or ARM64. A Raspberry Pi needs a **64-bit** Debian installation; this guide does not target ARMv7/32-bit systems. On macOS or Windows, use Docker Desktop with Linux containers.

Check your existing installation:

```sh
docker version
docker compose version
```

If Docker is missing, follow the [official Debian installation guide](https://docs.docker.com/engine/install/debian/) or the [Docker Desktop instructions](https://docs.docker.com/desktop/). Install the Compose plugin as described in the [official Compose guide](https://docs.docker.com/compose/install/). Use a maintained Docker Engine release **28 or newer**: older versions have a documented localhost-publishing issue. Installing Docker changes host services and networking, so review these steps for your computer before running them. This repository does not install Docker, alter permissions or configure your firewall.

You do not need Go, Node.js, npm or a system SQLite installation to build and run the container.

## 2. Get the source

Once this change has been merged:

```sh
git clone https://github.com/erickfaria/QSObook.git
cd QSObook
```

If you already have a checkout, use its directory instead. Your existing standalone database is **not** automatically imported. The first Compose start creates a separate empty notebook in its own volume; your previous notebook stays at its original path.

## 3. Build and start

From the repository directory:

```sh
docker compose up -d --build
docker compose ps
docker compose logs --tail=30 qsobook
```

Wait for the startup log, then open **http://127.0.0.1:8080** in your browser on the same computer. The first build can take several minutes, particularly on a Raspberry Pi. Keep the Pi powered and allow enough disk space for the Go builder and build cache. Future starts reuse the image.

The image runs as user `10001:10001`, with a read-only root filesystem and no Linux capabilities. The writable `/data` directory is a named Docker volume. Certificates and Go's embedded time-zone data allow the existing optional map functionality to work without a Linux distribution in the final image. The runtime image contains no shell or package manager.

To use another **local** port, create a `.env` file next to `compose.yaml`:

```dotenv
QSOBOOK_PORT=8081
```

Run `docker compose up -d` again and open `http://127.0.0.1:8081`. `.env` is ignored by Git and is not included in the image.

## 4. Use your notebook

Start a QSO round, add a fictitious operator and confirm a contact. Adding a participant and marking QRT do not confirm a contact. Open **Preferences** to choose Português, English, Español or Français and apply the change. Save any form draft before changing language, since applying reloads the current page. The language stays in a local browser cookie; it does not change stored operator data. The light/dark/system theme remains a separate browser preference.

On a phone-sized screen the QSO list stacks callsign, name, city, notes and actions into compact rows. This responsive layout does **not** enable LAN access: the local port and existing HTTP security checks are retained.

For a Pi running elsewhere, keep it local and use an SSH tunnel from a trusted computer, for example:

```sh
ssh -N -L 8081:127.0.0.1:8080 your-user@your-pi
```

Then open `http://127.0.0.1:8081` on that computer. This uses your existing SSH access; QSObook does not configure SSH. For direct access from phones on a trusted home LAN, use the private-network configuration with optional password in [INSTALL.md](INSTALL.md). Choose the documented LAN/VPN profile to bind to a specific network interface.

## 5. Stop and resume

```sh
docker compose stop
docker compose start
```

To remove the container and its Compose network while keeping the notebook:

```sh
docker compose down
```

Later, `docker compose up -d` recreates the container and reuses the named data volume. The volume is normally called `qsobook_data`. Keep the same Compose project name when updating. Do not scale multiple application instances against one notebook.

**Do not use `docker compose down -v` or delete/prune the data volume. Those operations delete the notebook.** Rebuilding or replacing the image alone does not delete its volume. A volume is persistent storage, not a backup.

## 6. Make an offline backup before updating

Stop QSObook before copying SQLite. Copy the **whole data directory**, including any SQLite journals, rather than just a database file that might still be in use. The following Linux/macOS shell commands create a new private backup directory each time:

```sh
mkdir -p backups
backup_dir="backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$backup_dir"
docker compose stop
docker compose cp qsobook:/data "$backup_dir/"
docker compose start
```

Only resume after the copy succeeds. If copying fails, inspect the error and make a successful stopped copy before upgrading. Keep backups private, offline where appropriate, and outside the Docker volume. `backups/` is ignored by Git and excluded from the build context. Store another copy wherever you choose; QSObook does not upload it.

This is the manual cold-backup alternative. The application also offers consistent in-app backups and validated restoration under Preferences → Data; see the [data guide](DATA.md). Do not overwrite a live notebook when testing restoration. Keep the original backup untouched and verify a working copy separately using the standalone application's `-db` option; check the operators, history and rounds before relying on it. Moving a notebook into a different Docker volume and ownership handling remain manual; the in-app restore works within the existing deployment.

## 7. Update

After making and checking the stopped backup, from a clean checkout:

```sh
git pull --ff-only
docker compose up -d --build
docker compose logs --tail=30 qsobook
```

Schema migrations run transactionally at startup, as in the standalone app. Keep your backup: older executables may refuse newer databases. Avoid downgrading against your only copy.

## Troubleshooting

- **`docker: command not found`:** install Docker using the official instructions above.
- **Cannot connect to the daemon / permission denied:** check your Docker installation and account access. Do not make the Docker socket world-writable.
- **Port already in use:** choose another `QSOBOOK_PORT` as above.
- **Container stops immediately:** inspect `docker compose logs --tail=50 qsobook`. Check the volume and any custom command before restarting; do not remove the volume to fix a startup error.
- **Empty notebook after changing project name:** the new project uses a different volume. Stop it and return to the original project name; do not delete either volume while investigating.
- **`Host not allowed` / Origin rejected:** use the local address or the SSH tunnel described above. These protections are intentional.
- **Map unavailable:** the notebook works offline. The map requires explicit opt-in, internet access and compliance with provider policies; see the [README](../README.md#a-small-home-screen-and-a-cached-map).

## Building for another architecture

A normal `docker compose up --build` uses the host's architecture. The Dockerfile supports `linux/amd64` and `linux/arm64`, using a multi-stage build with CGO disabled. CI builds and exercises the container natively on both architectures. A build made on your laptop does not automatically produce or publish a multi-architecture registry image.

The separate **Publish release** workflow publishes tested native ARM64/AMD64 images to GHCR with a temporary repository token, versioned tags, digest-pinned Compose assets and provenance attestations. It is manually dispatched on main, rather than deploying every code change. See [INSTALL.md](INSTALL.md) for installing a ready-made image and for private-LAN/VPN access and optional password setup. This source-build path remains available.

See Docker's documentation on [multi-stage builds](https://docs.docker.com/build/building/multi-stage/), [named volumes](https://docs.docker.com/engine/storage/volumes/) and [localhost port publishing](https://docs.docker.com/engine/network/port-publishing/) for the underlying mechanisms.
