"""Lab subscription server: serves sub.txt (/sub) and subns.txt (/subns)
with subscription-userinfo. Lab only, localhost/veth only."""
import http.server
import os

LAB = os.environ.get("CONNECTIVE_LAB", os.path.join(
    os.path.expanduser("~"), ".local", "share", "connective", "lab"))
PORT = int(os.environ.get("CONNECTIVE_SUBPORT", "18080"))


class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/sub":
            fname = os.path.join(LAB, "sub.txt")
        elif self.path == "/subns":
            fname = os.path.join(LAB, "subns.txt")
        else:
            self.send_response(404)
            self.end_headers()
            return
        with open(fname, "rb") as f:
            body = f.read()
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("subscription-userinfo",
                         "upload=1048576; download=2097152; "
                         "total=10737418240; expire=1780000000")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


http.server.HTTPServer(("0.0.0.0", PORT), H).serve_forever()
