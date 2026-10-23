"""CI smoke test using a disposable Compose project and fictitious data only.

Requires Docker Engine, Compose and Python's standard library. It builds locally,
never publishes images, and removes only this test's randomly named containers
and volume. Do not point it at an existing deployment.
"""

import http.cookiejar
import json
import os
import re
import socket
import subprocess
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
compose = ["docker", "compose", "--project-name", project, "--file", "compose.yaml"]
base = f"http://127.0.0.1:{port}"
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def run(*args, capture=False):
    return subprocess.run([*compose, *args], env=env, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def get(path):
    with opener.open(base + path, timeout=5) as response:
        return response.read().decode()


def ready():
    for _ in range(30):
        try:
            return get("/")
        except (OSError, urllib.error.URLError):
            time.sleep(1)
    raise RuntimeError("Container did not become ready")


def post(path, **fields):
    page = get("/")
    fields["csrf"] = re.search(r'name="csrf" value="([^"]+)"', page)[1]
    request = urllib.request.Request(base + path, data=urllib.parse.urlencode(fields).encode(),
                                     headers={"Origin": base})
    with opener.open(request, timeout=5) as response:
        assert response.status == 200, response.status
        return response.read().decode()


try:
    run("config", "--quiet")
    run("up", "--detach", "--build")
    ready()
    container = run("ps", "--quiet", "qsobook", capture=True).strip()
    info = json.loads(subprocess.check_output(["docker", "inspect", container], text=True))[0]
    assert info["Config"]["User"] == "10001:10001"
    assert info["HostConfig"]["ReadonlyRootfs"] is True
    assert info["HostConfig"]["PortBindings"]["8080/tcp"][0]["HostIp"] == "127.0.0.1"
    assert "ALL" in info["HostConfig"]["CapDrop"]
    assert any(m["Destination"] == "/data" and m["Type"] == "volume" for m in info["Mounts"])
    post("/round/start", previous_round="0", name="Container smoke test")
    for _ in range(2):
        post("/participant", round_id="1", callsign="zz0docker", name="Example operator",
             city="Example city", country="Example country", mode="DMR", notes="Example conversation")
    assert json.loads(get("/lookup?callsign=zz0docker"))["contacts"] == 0
    for _ in range(2):
        post("/participant/record", id="1")
    data = json.loads(get("/lookup?callsign=zz0docker"))
    assert data["contacts"] == 1 and data["name"] == "Example operator", data
    post("/participant/status", id="1", active="0")
    post("/language", language="fr", return_to="/round")
    assert '<html lang="fr"' in get("/round")
    # Recreate the container, not the named volume: records must survive.
    run("stop", "--timeout", "10")
    run("up", "--detach", "--force-recreate", "--no-build")
    ready()
    assert json.loads(get("/lookup?callsign=zz0docker")) == data
    assert "QRT" in get("/round") and "Example conversation" in get("/round")
    # The bridge listener must not weaken HTTP security.
    request = urllib.request.Request(base + "/", headers={"Host": "example.com"})
    try:
        opener.open(request, timeout=5)
        raise AssertionError("External Host accepted")
    except urllib.error.HTTPError as error:
        assert error.code == 403
    request = urllib.request.Request(base + "/language", data=b"language=en&return_to=/",
                                     headers={"Origin": "https://example.com"})
    try:
        opener.open(request, timeout=5)
        raise AssertionError("External Origin accepted")
    except urllib.error.HTTPError as error:
        assert error.code == 403
    print("Container smoke passed: non-root, local port, writes, idempotency, language, persistence and HTTP security.")
finally:
    # This unique project contains only the disposable data created above.
    subprocess.run([*compose, "logs", "--tail", "30"], env=env, check=False)
    subprocess.run([*compose, "down", "--volumes", "--remove-orphans"], env=env, check=False)
