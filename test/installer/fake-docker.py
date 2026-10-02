"""Fake Docker engine for the install.sh tests: answers 200 OK on a unix socket, so `doctor` is green."""

import http.server
import os
import socketserver
import sys


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(b"OK")

    def log_message(self, *args):
        pass


class Server(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True


if __name__ == "__main__":
    path = sys.argv[1]
    if os.path.exists(path):
        os.remove(path)
    Server(path, Handler).serve_forever()
