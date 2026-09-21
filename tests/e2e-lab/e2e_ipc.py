"""Minimal IPC driver for the e2e lab.
Usage: e2e_ipc.py <method> [json-payload]
Env: CONNECTIVE_SOCK overrides the socket path."""
import json
import os
import socket
import sys

SOCK = os.environ.get("CONNECTIVE_SOCK",
    os.path.expanduser("~/.local/share/connective/connectived.sock"))


def call(method, payload=None):
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.settimeout(30)
    s.connect(SOCK)
    f = s.makefile("r")
    req = {"v": "1", "id": "1", "type": method}
    if payload is not None:
        req["payload"] = payload
    s.sendall((json.dumps(req) + "\n").encode())
    while True:  # skip async event frames until our response arrives
        resp = json.loads(f.readline())
        if resp.get("id") == "1":
            break
    s.close()
    if resp.get("error"):
        print(f"ERROR {method}: {resp['error']}")
        sys.exit(1)
    out = resp.get("payload")
    print(json.dumps(out, indent=2) if out is not None else "OK")
    return out


if __name__ == "__main__":
    method = sys.argv[1]
    payload = json.loads(sys.argv[2]) if len(sys.argv) > 2 else None
    call(method, payload)
