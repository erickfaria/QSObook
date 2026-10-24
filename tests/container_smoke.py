"""CI smoke test using a disposable Compose project and fictitious data only.

Requires Docker Engine, Compose and Python's standard library. It builds locally,
never publishes images, and removes only this test's randomly named containers
and volume. Do not point it at an existing deployment.
"""

import base64
import http.cookiejar
import json
import os
import re
import socket
import sqlite3
import subprocess
import tempfile
from pathlib import Path
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

project = "qsobook-ci-" + uuid.uuid4().hex[:12]
with socket.socket() as port_socket:
    port_socket.bind(("127.0.0.1", 0))
    port = port_socket.getsockname()[1]
env = {**os.environ, "QSOBOOK_PORT": str(port)}
compose_file = os.environ.get("QSOBOOK_COMPOSE_FILE", "compose.yaml")
compose = ["docker", "compose", "--project-name", project, "--file", compose_file]
private_files = tempfile.TemporaryDirectory(prefix=project + "-")
auth_headers = {}
if compose_file in ("compose.lan.yaml", "compose.auth.yaml"):
    env.update(QSOBOOK_BIND_ADDRESS="127.0.0.1", QSOBOOK_ALLOWED_HOSTS=f"tortuga.local:{port}")
if compose_file == "compose.auth.yaml":
    password = "synthetic-password-" + uuid.uuid4().hex
    password_file = Path(private_files.name) / "password"
    password_file.write_text(password + "\n")
    password_file.chmod(0o444)  # Private 0700 parent; readable by the non-root container.
    env.update(QSOBOOK_PASSWORD_FILE=str(password_file), QSOBOOK_BIND_ADDRESS="127.0.0.1",
               QSOBOOK_ALLOWED_HOSTS=f"tortuga.local:{port}")
    auth_headers["Authorization"] = "Basic " + base64.b64encode(("qsobook:" + password).encode()).decode()
base = f"http://127.0.0.1:{port}"
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def run(*args, capture=False):
    return subprocess.run([*compose, *args], env=env, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def get(path):
    with opener.open(urllib.request.Request(base + path, headers=auth_headers), timeout=5) as response:
        return response.read().decode()


def ready():
    for _ in range(30):
        try:
            return get("/")
        except (OSError, urllib.error.URLError):
            time.sleep(1)
    raise RuntimeError("Container did not become ready")


def post_bytes(path, **fields):
    page = get("/")
    fields["csrf"] = re.search(r'name="csrf" value="([^"]+)"', page)[1]
    request = urllib.request.Request(base + path, data=urllib.parse.urlencode(fields).encode(),
                                     headers={**auth_headers, "Origin": base})
    with opener.open(request, timeout=30) as response:
        assert response.status == 200, response.status
        return response.read()


def post(path, **fields):
    return post_bytes(path, **fields).decode()


def upload_file(path, content):
    token = re.search(r'name="csrf" value="([a-f0-9]+)"', get("/"))[1]
    boundary = "qsobook-" + uuid.uuid4().hex
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="csrf"\r\n\r\n{token}\r\n'
            f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="notebook.db"\r\n'
            'Content-Type: application/octet-stream\r\n\r\n').encode() + content + f"\r\n--{boundary}--\r\n".encode()
    request = urllib.request.Request(base + path, data=body, headers={**auth_headers, "Origin": base,
                                     "Content-Type": f"multipart/form-data; boundary={boundary}"})
    with opener.open(request, timeout=30) as response:
        assert response.status == 200, response.status
        return response.read().decode()


try:
    run("config", "--quiet")
    run("up", "--detach", "--build" if compose_file == "compose.yaml" else "--no-build")
    ready()
    if auth_headers:
        try:
            urllib.request.urlopen(base, timeout=5)
            raise AssertionError("Unauthenticated LAN request accepted")
        except urllib.error.HTTPError as error:
            assert error.code == 401
    if compose_file in ("compose.lan.yaml", "compose.auth.yaml"):
        with opener.open(urllib.request.Request(base, headers={**auth_headers, "Host": f"tortuga.local:{port}"}), timeout=5) as response:
            assert response.status == 200
            assert response.headers.get("WWW-Authenticate") is None
    container = run("ps", "--quiet", "qsobook", capture=True).strip()
    info = json.loads(subprocess.check_output(["docker", "inspect", container], text=True))[0]
    assert info["Config"]["User"] == "10001:10001"
    assert info["HostConfig"]["ReadonlyRootfs"] is True
    assert info["HostConfig"]["PortBindings"]["8080/tcp"][0]["HostIp"] == "127.0.0.1"
    assert "ALL" in info["HostConfig"]["CapDrop"]
    assert any(m["Destination"] == "/data" and m["Type"] == "volume" for m in info["Mounts"])
    post("/round/start", previous_round="0", workspace="1", in_round="1", name="Container smoke test", edition="7")
    for _ in range(2):
        post("/participant", round_id="1", callsign="zz0docker", name="Example operator",
             city="Example city", country="Example country", mode="DMR", notes="Example conversation")
    assert json.loads(get("/lookup?callsign=zz0docker"))["contacts"] == 0
    for _ in range(2):
        post("/participant/record", id="1")
    data = json.loads(get("/lookup?callsign=zz0docker"))
    assert data["contacts"] == 1 and data["name"] == "Example operator", data
    # The primary QSO workspace adds an unconfirmed participant first and
    # confirms explicitly. Replays never duplicate contacts or registrations.
    page = get("/round")
    assert 'href="/round"' in page and 'href="/records"' not in page
    key = re.search(r'name="submission_key" value="([a-f0-9]+)"', page)[1]
    post("/qso/entry", round_id="1", submission_key=key, callsign="zz1docker",
         name="Record example", country="Brasil", mode="DMR", band="70 cm",
         operator_notes="Permanent note", notes="Record conversation")
    assert json.loads(get("/lookup?callsign=zz1docker"))["contacts"] == 0
    key = re.search(r'name="submission_key" value="([a-f0-9]+)"', get("/round"))[1]
    for _ in range(2):
        post("/qso/entry", round_id="1", submission_key=key, callsign="zz1docker", confirmed="1", mode="DMR", band="70 cm")
    assert json.loads(get("/lookup?callsign=zz1docker"))["contacts"] == 1
    assert json.loads(get("/lookup?callsign=zz1docker"))["notes"] == "Permanent note"
    post("/participant/status", id="1", active="0")
    # A URL parameter cannot supply a missing write target, even with valid CSRF.
    try:
        post("/round/end?id=1")
        raise AssertionError("Query-only write target accepted")
    except urllib.error.HTTPError as error:
        assert error.code == 400
    post("/language", language="fr", return_to="/round")
    assert '<html lang="fr"' in get("/round")
    # Exercise the transfer UI with the real image and a read-only root filesystem.
    expected_version = os.environ.get("VERSION", "dev")
    assert f"<strong>{expected_version}</strong>" in get("/about")
    snapshot = post_bytes("/data/backup")
    snapshot_file = Path(private_files.name) / "live-backup.db"
    snapshot_file.write_bytes(snapshot)
    with sqlite3.connect(snapshot_file) as connection:
        assert connection.execute("PRAGMA integrity_check").fetchone() == ("ok",)
        assert connection.execute("SELECT count(*) FROM qsos").fetchone() == (2,)
    post("/trash", id="2", kind="contact", action="delete")
    assert json.loads(get("/lookup?callsign=zz1docker"))["contacts"] == 0
    post("/trash", id="2", kind="contact", action="restore")
    post("/trash", id="2", kind="station", action="delete")
    assert "ZZ1DOCKER" in get("/history")
    post("/trash", id="2", kind="station", action="restore")
    post("/station", callsign="zz2recovery", name="Recovery example")
    preview = upload_file("/data/restore", snapshot)
    key = re.search(r'name="key" value="([a-f0-9]+)"', preview)[1]
    post("/data/confirm", key=key, confirm="yes")
    try:
        get("/lookup?callsign=zz2recovery")
        raise AssertionError("Restore did not replace the registration set")
    except urllib.error.HTTPError as error:
        assert error.code == 404
    assert "pre-restore-" in get("/data")
    adi = post_bytes("/data/export")
    preview = upload_file("/data/import", adi)
    key = re.search(r'name="key" value="([a-f0-9]+)"', preview)[1]
    post("/data/confirm", key=key, confirm="yes")
    assert json.loads(get("/lookup?callsign=zz1docker"))["contacts"] == 1
    post("/map/edit", city="Example city", state="", country="Example country",
         action="save", latitude="1", longitude="2")
    # Recreate the container, not the named volume: records must survive.
    run("stop", "--timeout", "10")
    backup = Path(private_files.name) / "backup"
    backup.mkdir(mode=0o700)
    run("cp", "qsobook:/data", str(backup))
    with sqlite3.connect(f"file:{backup / 'data' / 'qsobook.db'}?mode=ro", uri=True) as connection:
        assert connection.execute("PRAGMA integrity_check").fetchone() == ("ok",)
        assert connection.execute("PRAGMA foreign_key_check").fetchall() == []
        assert connection.execute("SELECT count(*) FROM qsos").fetchone() == (2,)
        assert connection.execute("SELECT band,round_name FROM qsos WHERE callsign='ZZ1DOCKER'").fetchone() == ("70 cm", "Container smoke test #7")
        assert connection.execute("PRAGMA user_version").fetchone() == (7,)
        assert connection.execute("SELECT last_round_name,last_round_edition FROM app_settings").fetchone() == ("Container smoke test", "7")
    run("up", "--detach", "--force-recreate", "--no-build")
    ready()
    assert json.loads(get("/lookup?callsign=zz0docker")) == data
    assert json.loads(get("/lookup?callsign=zz1docker"))["contacts"] == 1
    assert "QRT" in get("/round") and "Example conversation" in get("/round")
    # The bridge listener must not weaken HTTP security.
    request = urllib.request.Request(base + "/", headers={**auth_headers, "Host": "example.com"})
    try:
        opener.open(request, timeout=5)
        raise AssertionError("External Host accepted")
    except urllib.error.HTTPError as error:
        assert error.code == 403
    request = urllib.request.Request(base + "/language", data=b"language=en&return_to=/",
                                     headers={**auth_headers, "Origin": "https://example.com"})
    try:
        opener.open(request, timeout=5)
        raise AssertionError("External Origin accepted")
    except urllib.error.HTTPError as error:
        assert error.code == 403
    print("Container smoke passed: non-root, local port, writes, idempotency, language, live/cold backups, restoration, trash, ADIF deduplication, offline map correction, version, persistence and HTTP security.")
finally:
    # This unique project contains only the disposable data created above.
    subprocess.run([*compose, "logs", "--tail", "30"], env=env, check=False)
    subprocess.run([*compose, "down", "--volumes", "--remove-orphans"], env=env, check=False)
    private_files.cleanup()
