#!/usr/bin/env python3
"""Opt-in real OpenCode V2 transport test; model and Beads findings are fixtures.

All configuration, session data and Hook state live in a temporary directory.
Only the private server started here is stopped; no real Beads store is opened.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlencode


ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    opencode = shutil.which("opencode")
    if not opencode:
        raise SystemExit("OpenCode V2 must be installed for this opt-in test")
    requests: list[dict] = []

    class Model(BaseHTTPRequestHandler):
        def log_message(self, *_args: object) -> None:
            pass

        def do_POST(self) -> None:
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append(body)
            content = "isolated fixture reply"
            if body.get("stream"):
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                for delta, finish in (({"role": "assistant", "content": content}, None), ({}, "stop")):
                    chunk = {"id": "chatcmpl-fixture", "object": "chat.completion.chunk", "created": 1,
                             "model": "fixture", "choices": [{"index": 0, "delta": delta, "finish_reason": finish}]}
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.write(b"data: [DONE]\n\n")
            else:
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps({"id": "chatcmpl-fixture", "object": "chat.completion", "created": 1,
                    "model": "fixture", "choices": [{"index": 0, "message": {"role": "assistant", "content": content},
                    "finish_reason": "stop"}], "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}}).encode())

    model = ThreadingHTTPServer(("127.0.0.1", 0), Model)
    threading.Thread(target=model.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="agentdeck-opencode-e2e-") as temporary:
            base = Path(temporary)
            workspace = base / "workspace"
            home = base / "home"
            config = home / ".config/opencode"
            workspace.mkdir()
            config.mkdir(parents=True)
            (workspace / ".agent-instructions").mkdir()
            (workspace / ".agent-instructions/beads.md").write_text("isolated Beads fixture\n")
            (workspace / "scripts/hooks").mkdir(parents=True)
            parser = base / "parser.py"
            parser.write_text("INVOCATION_PREFIXES = ()\nCOMMAND_ROUTES = {'评审': 'REVIEW'}\n"
                              "def route_prompt(prompt):\n    return 'REVIEW' if prompt.startswith('评审：') else None\n")
            # Run the real observer main and state/deduplication machinery. Only
            # repository lookup and Beads-derived findings are test seams.
            (workspace / "scripts/hooks/beads-consistency.py").write_text(
                "import importlib.util\nfrom pathlib import Path\n"
                f"spec = importlib.util.spec_from_file_location('fixture_observer', {str(ROOT / 'scripts/hooks/beads-consistency.py')!r})\n"
                "module = importlib.util.module_from_spec(spec)\nspec.loader.exec_module(module)\n"
                "module.repo_root = lambda deadline: Path.cwd()\n"
                "module.findings = lambda root, deadline, scope=None: ['E2E_FIXTURE_MISMATCH: dispatch disagrees with fixture review']\n"
                "raise SystemExit(module.main())\n"
            )
            plugin = workspace / ".opencode/plugins/beads-consistency"
            plugin.mkdir(parents=True)
            (plugin / "package.json").write_text('{"type":"module","main":"./index.js","private":true}\n')
            # Import the unchanged production adapter, not a copied implementation.
            (plugin / "index.js").write_text(
                f"import {{ setupBeadsConsistency }} from {json.dumps((ROOT / 'scripts/hooks/opencode-beads.mjs').as_uri())}\n"
                "export default { id: 'agentdeck.beads-consistency', setup: setupBeadsConsistency }\n"
            )
            (config / "opencode.json").write_text(json.dumps({
                "$schema": "https://opencode.ai/config.json", "model": "fixture/model",
                "providers": {"fixture": {"env": ["FIXTURE_MODEL_KEY"],
                    "package": "@opencode/ai/providers/openai-compatible",
                    "settings": {"baseURL": f"http://127.0.0.1:{model.server_port}/v1"},
                    "models": {"model": {"limit": {"context": 32000, "output": 1000}}}}},
                "plugins": ["*", "-claude-mem"],
                "permissions": [{"action": "*", "resource": "*", "effect": "deny"}],
                "snapshots": False,
            }))
            env = {"PATH": os.environ["PATH"], "HOME": str(home), "SHELL": "/bin/zsh",
                   "XDG_CONFIG_HOME": str(home / ".config"), "XDG_DATA_HOME": str(base / "data"),
                   "XDG_STATE_HOME": str(base / "state"), "XDG_CACHE_HOME": str(base / "cache"),
                   "OPENCODE_CONFIG_DIR": str(config), "OPENCODE_PASSWORD": "isolated-fixture-password",
                   "FIXTURE_MODEL_KEY": "not-a-real-key",
                   "AGENTDECK_WORKFLOW_HOOK": str(parser),
                   "AGENTDECK_BEADS_HOOK_STATE_DIR": str(base / "hook-state"),
                   "DO_NOT_TRACK": "1", "TMPDIR": str(base)}
            with socket.socket() as socket_probe:
                socket_probe.bind(("127.0.0.1", 0))
                port = socket_probe.getsockname()[1]
            url = f"http://127.0.0.1:{port}"
            database = subprocess.run([opencode, "debug", "paths", "db"], cwd=workspace, env=env,
                                      text=True, capture_output=True, check=True, timeout=10).stdout.strip()
            assert Path(database).is_relative_to(base), "Test database must be isolated"

            def api(method: str, path: str, payload: dict | None = None) -> dict | list | None:
                command = [opencode, "api", "--server", url, method, path]
                if payload is not None:
                    command += ["--data", json.dumps(payload, ensure_ascii=False)]
                result = subprocess.run(command, cwd=workspace, env=env, capture_output=True, text=True, timeout=20)
                if result.returncode:
                    raise RuntimeError(f"{method} {path}: {result.stdout[-1000:]} {result.stderr[-1000:]}")
                return json.loads(result.stdout) if result.stdout.strip() else None

            with (base / "server.log").open("w+") as log:
                server = subprocess.Popen([opencode, "serve", "--hostname", "127.0.0.1", "--port", str(port),
                                           "--print-logs", "--log-level", "debug"],
                                          cwd=workspace, env=env, stdout=log, stderr=log)
                try:
                    # Wait only for our new server's listener, never restart or
                    # poll the user's shared service.
                    deadline = time.monotonic() + 20
                    while True:
                        try:
                            with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                                break
                        except OSError:
                            if server.poll() is not None or time.monotonic() > deadline:
                                log.seek(0)
                                raise RuntimeError("Private server startup failed: " + log.read()[-2000:])
                            time.sleep(0.05)
                    select = "?" + urlencode({"location[directory]": str(workspace)})
                    session = api("post", "/api/session", {"title": "Isolated Hook acceptance",
                        "location": {"directory": str(workspace)}, "model": {"providerID": "fixture", "id": "model"}})
                    session_id = session["data"]["id"]
                    api("post", f"/api/session/{session_id}/prompt", {"text": "评审：fixture / tasks.md"})
                    plugins = api("get", "/api/plugin" + select)
                    if not any(p["id"] == "agentdeck.beads-consistency" and p["state"]["status"] == "active"
                               for p in plugins["data"]):
                        log.seek(0)
                        raise AssertionError(json.dumps({"plugins": plugins, "server_log": log.read()[-5000:]}))
                    deadline = time.monotonic() + 30
                    while True:
                        messages = api("get", f"/api/session/{session_id}/context")["data"]
                        synthetic = [m for m in messages if m["type"] == "synthetic"
                                     and "E2E_FIXTURE_MISMATCH" in m.get("text", "")]
                        replies = [m for m in messages if m["type"] == "assistant" and m.get("time", {}).get("completed")]
                        if synthetic and len(replies) >= 2:
                            break
                        if time.monotonic() > deadline:
                            log.seek(0)
                            raise AssertionError(json.dumps({"message_types": [m["type"] for m in messages],
                                "replies": len(replies), "synthetic": len(synthetic), "server_log_tail": log.read()[-3000:]}, ensure_ascii=False))
                        time.sleep(0.1)
                    assert len(synthetic) == 1, synthetic
                    assert len([m for m in messages if m["type"] == "user"]) == 1
                    assert any("E2E_FIXTURE_MISMATCH" in json.dumps(r.get("messages", [])) for r in requests)
                    # New unmatched user input clears scope: do not continue it.
                    api("post", f"/api/session/{session_id}/prompt", {"text": "Explain an unrelated fixture"})
                    api("post", f"/api/experimental/session/{session_id}/wait", {})
                    messages = api("get", f"/api/session/{session_id}/context")["data"]
                    assert len([m for m in messages if m["type"] == "synthetic"]) == 1
                    print(json.dumps({"status": "PASS", "runtime": "real private OpenCode V2 server",
                        "model": "loopback HTTP fixture", "beads": "fixture findings, real Python scope/dedup machinery",
                        "verified": ["plugin active", "native prompt", "successful execution event", "Python observer",
                                     "synthetic admission", "automatic second model call sees diagnostic", "one continuation",
                                     "unmatched new user request clears scope"],
                        "real_beads_access": False, "shared_service_restart": False}, ensure_ascii=False))
                finally:
                    server.terminate()
                    try:
                        server.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        server.kill()
                        server.wait()
    finally:
        model.shutdown()
        model.server_close()


if __name__ == "__main__":
    main()
