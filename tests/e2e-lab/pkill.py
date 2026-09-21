"""pkill.py <substring>: kill processes whose cmdline contains substring,
never matching self or ancestors. Prints killed PIDs.
SAFETY: only ever pass patterns under our own lab paths.
Env: PKILL_SIG (default SIGTERM)."""
import os
import signal as sigmod
import sys

me = os.getpid()
ancestors = set()
try:
    with open(f"/proc/{me}/status") as f:
        for line in f:
            if line.startswith("PPid:"):
                ppid = int(line.split()[1])
                while ppid > 1:
                    ancestors.add(ppid)
                    try:
                        with open(f"/proc/{ppid}/status") as f2:
                            for l2 in f2:
                                if l2.startswith("PPid:"):
                                    ppid = int(l2.split()[1])
                                    break
                    except Exception:
                        break
                break
except Exception:
    pass

pat = sys.argv[1]
signo = getattr(sigmod, os.environ.get("PKILL_SIG", "SIGTERM"))
killed = []
for pid in os.listdir("/proc"):
    if not pid.isdigit():
        continue
    p = int(pid)
    if p == me or p in ancestors:
        continue
    try:
        with open(f"/proc/{p}/cmdline", "rb") as f:
            cmd = f.read().replace(b"\x00", b" ").decode(errors="replace")
    except Exception:
        continue
    if pat in cmd and "pkill.py" not in cmd:
        try:
            os.kill(p, signo)
            killed.append(p)
        except Exception as e:
            print(f"pid {p}: {e}")
print("killed:", killed)
