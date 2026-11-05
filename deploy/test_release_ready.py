"""Readback must survive the gap between systemctl restart and socket bind."""
import http.server
import importlib.util
import socket
import threading
import time
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class Version(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"server_build":"abc"}')

    def log_message(self, *args):
        pass


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class WaitReachable(unittest.TestCase):
    def test_server_binding_late_is_awaited(self):
        port = free_port()
        servers = []

        def start_late():
            time.sleep(1.5)
            server = http.server.HTTPServer(("127.0.0.1", port), Version)
            servers.append(server)
            server.serve_forever()

        threading.Thread(target=start_late, daemon=True).start()
        try:
            status, body = release.wait_reachable(f"http://127.0.0.1:{port}", "/v1/version", 10)
        finally:
            for server in servers:
                server.shutdown()
        self.assertEqual(status, 200)
        self.assertIn(b"abc", body)

    def test_never_bound_still_fails_after_deadline(self):
        port = free_port()
        start = time.monotonic()
        with self.assertRaises(release.ReleaseError):
            release.wait_reachable(f"http://127.0.0.1:{port}", "/v1/version", 1)
        self.assertLess(time.monotonic() - start, 5)


if __name__ == "__main__":
    unittest.main()
