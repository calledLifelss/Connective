"""Minimal DNS A-query client (stdlib only): dnsq.py <server-ip> <name>.
Proves a DNS answer travelled a specific path (e.g. through the TUN)."""
import random
import socket
import struct
import sys


def query(server, name, timeout=5):
    txid = random.randrange(65536)
    qname = b"".join(bytes([len(p)]) + p.encode() for p in name.split(".")) + b"\x00"
    pkt = struct.pack(">HHHHHH", txid, 0x0100, 1, 0, 0, 0) + qname + struct.pack(">HH", 1, 1)
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.settimeout(timeout)
    s.sendto(pkt, (server, 53))
    data, _ = s.recvfrom(512)
    rtxid, flags, qd, an, _, _ = struct.unpack(">HHHHHH", data[:12])
    if rtxid != txid:
        raise RuntimeError("transaction id mismatch")
    if flags & 0x000F:
        raise RuntimeError(f"dns rcode {flags & 0x000F}")
    off = 12 + len(qname) + 4
    ips = []
    for _ in range(an):
        if data[off] & 0xC0 == 0xC0:
            off += 2
        else:
            while data[off]:
                off += 1 + data[off]
            off += 1
        rtype, rclass, ttl, rdlen = struct.unpack(">HHIH", data[off:off + 10])
        off += 10
        if rtype == 1 and rdlen == 4:
            ips.append(socket.inet_ntoa(data[off:off + 4]))
        off += rdlen
    return ips


if __name__ == "__main__":
    print(query(sys.argv[1], sys.argv[2]))
