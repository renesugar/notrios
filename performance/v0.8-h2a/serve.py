#!/usr/bin/env python3
"""Serve the probe bundle under the app's exact production CSP.

Any relaxation here would invalidate the result, so the header is copied
verbatim from internal/httpapi/server.go webAppCSP.
"""
import http.server, os, sys

CSP = ("default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "
       "img-src 'self' data: blob: https: http:; font-src 'self' data:; "
       "connect-src 'self'; object-src 'none'; frame-src 'none'; "
       "base-uri 'self'; form-action 'none'")

class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Content-Security-Policy", CSP)
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Cache-Control", "no-store")
        super().end_headers()
    def log_message(self, *args):
        pass

os.chdir(sys.argv[1])
http.server.HTTPServer(("127.0.0.1", int(sys.argv[2])), Handler).serve_forever()
