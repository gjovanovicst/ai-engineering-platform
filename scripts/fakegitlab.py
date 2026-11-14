"""Minimal GitLab API stand-in for the e2e pipeline (MR create + merge)."""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

STATE = {"next_iid": 1, "mrs": {}}

class Handler(BaseHTTPRequestHandler):
    def _reply(self, status, payload):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(payload).encode())

    def _body(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            return json.loads(raw or b"{}")
        except json.JSONDecodeError:
            return {}

    def do_POST(self):
        body = self._body()
        iid = STATE["next_iid"]
        STATE["next_iid"] += 1
        STATE["mrs"][iid] = {"iid": iid, "state": "opened", "source_branch": body.get("source_branch", "")}
        self._reply(201, {"iid": iid, "web_url": f"http://fakegitlab/mr/{iid}", "state": "opened"})

    def do_PUT(self):
        body = self._body()
        iid = int(body.get("iid", 0) or 0)
        if iid in STATE["mrs"]:
            STATE["mrs"][iid]["state"] = "merged"
        self._reply(200, {"iid": iid, "state": "merged"})

    def do_GET(self):
        self._reply(200, {"mrs": list(STATE["mrs"].values())})

    def log_message(self, *args):
        pass

HTTPServer(("0.0.0.0", 5000), Handler).serve_forever()
