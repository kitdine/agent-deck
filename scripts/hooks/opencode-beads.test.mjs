import assert from "node:assert/strict"
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { setImmediate } from "node:timers/promises"
import { test } from "node:test"
import { runObserver, setupBeadsConsistency } from "./opencode-beads.mjs"
import plugin from "../../.opencode/plugins/beads-consistency/index.js"

function fixture(t, observer = async () => null) {
  const directory = mkdtempSync(join(tmpdir(), "agentdeck-opencode-test-"))
  mkdirSync(join(directory, ".agent-instructions"))
  writeFileSync(join(directory, ".agent-instructions/beads.md"), "isolated fixture\n")
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  const hooks = new Map()
  const calls = []
  const synthetic = []
  const warnings = []
  const messages = new Map()
  const locations = new Map()
  let disposed = false
  let deliver
  let end
  const ctx = {
    location: { directory },
    session: {
      hook: async (name, callback) => {
        hooks.set(name, callback)
        return { dispose: async () => { disposed = true } }
      },
      get: async ({ sessionID }) => ({ location: { directory: locations.get(sessionID) ?? directory } }),
      context: async ({ sessionID }) => messages.get(sessionID) ?? [],
      synthetic: async (input) => { synthetic.push(input) },
    },
    event: {
      subscribe: ({ signal }) => ({
        [Symbol.asyncIterator]() {
          return {
            next() {
              if (signal.aborted) return Promise.resolve({ done: true })
              return new Promise((resolve) => {
                deliver = (value) => resolve({ value, done: false })
                end = () => resolve({ done: true })
                signal.addEventListener("abort", end, { once: true })
              })
            },
          }
        },
      }),
    },
  }
  const start = async () => {
    const cleanup = await setupBeadsConsistency(ctx, {
      observer: async (root, event, signal) => {
        calls.push({ root, event, signal })
        return observer(root, event, signal)
      },
      warn: (text) => warnings.push(text),
    })
    t.after(cleanup)
    return cleanup
  }
  const prompt = async (id = "ses_a", messageID = "msg_1", text = "评审：current / tasks.md") => {
    messages.set(id, [{ type: "user", id: messageID, text }])
    await hooks.get("prompt")({ sessionID: id, messageID, prompt: { text } })
  }
  const send = async (type = "session.execution.succeeded", data = { sessionID: "ses_a" }, location = null) => {
    deliver({ type, data, location })
    await flush()
  }
  return { directory, ctx, hooks, calls, synthetic, warnings, messages, locations, start, prompt, send, disposed: () => disposed }
}

async function flush() {
  for (let n = 0; n < 8; n++) await setImmediate()
}

test("project entry is a native V2 definition with no V1 hooks object", () => {
  assert.equal(plugin.id, "agentdeck.beads-consistency")
  assert.equal(plugin.setup, setupBeadsConsistency)
  assert.equal(plugin.server, undefined)
})

test("inert outside this repository and does not change skills or user prompts", async () => {
  const ctx = { location: { directory: "/nonexistent-agentdeck-fixture" } }
  assert.equal(await setupBeadsConsistency(ctx), undefined)
})

test("real prompt scope and Stop JSON use isolated opencode identity", async (t) => {
  const f = fixture(t, async (_root, event) => event.hook_event_name === "Stop" ? { decision: "block", reason: "current mismatch" } : null)
  await f.start()
  await f.prompt()
  await f.send("session.text.ended", { sessionID: "ses_a", text: "done" })
  await f.send()
  assert.equal(f.calls.length, 2)
  assert.deepEqual(f.calls[0].event, {
    hook_event_name: "UserPromptSubmit", session_id: "ses_a", prompt_id: "msg_1", prompt: "评审：current / tasks.md",
  })
  assert.equal(f.calls[1].event.last_assistant_message, "done")
  assert.equal(f.calls[1].event.stop_hook_active, false)
  assert.equal(f.synthetic.length, 1)
  assert.equal(f.synthetic[0].resume, true)
  assert.equal(f.synthetic[0].delivery, "queue")
  assert.equal(f.synthetic[0].metadata.promptID, "msg_1")
  assert.equal(f.synthetic[0].text, "current mismatch")
})

test("reconciliation may report but cannot start another continuation loop", async (t) => {
  const f = fixture(t, async (_root, event) => event.hook_event_name === "Stop"
    ? (event.stop_hook_active ? { systemMessage: "still missing" } : { decision: "block", reason: "missing" }) : null)
  await f.start()
  await f.prompt()
  await f.send()
  await f.send()
  assert.deepEqual(f.synthetic.map((input) => input.resume), [true, false])
  await f.prompt("ses_a", "msg_2", "继续")
  await f.send()
  assert.equal(f.synthetic.at(-1).resume, true)
  assert.equal(f.calls.at(-1).event.prompt_id, "msg_2")
})

test("no diagnostic means no synthetic work", async (t) => {
  const f = fixture(t)
  await f.start()
  await f.prompt("ses_a", "msg_1", "ordinary read-only request")
  await f.send()
  assert.equal(f.synthetic.length, 0)
})

test("foreign sessions, locations and failed executions cannot borrow scope", async (t) => {
  const f = fixture(t)
  await f.start()
  await f.prompt()
  await f.send("session.execution.succeeded", { sessionID: "ses_b" })
  await f.send("session.execution.succeeded", { sessionID: "ses_a" }, { directory: "/another/checkout" })
  await f.send("session.execution.failed")
  f.locations.set("ses_a", "/moved/session")
  await f.send()
  assert.equal(f.calls.length, 1)
})

test("queued new user input invalidates an older finished turn", async (t) => {
  const f = fixture(t)
  await f.start()
  await f.prompt("ses_a", "msg_old")
  await f.prompt("ses_a", "msg_new")
  f.messages.set("ses_a", [{ type: "user", id: "msg_old" }])
  await f.send()
  assert.equal(f.calls.length, 2)
})

test("new prompt during an in-flight check drops the stale diagnostic", async (t) => {
  let finish
  const f = fixture(t, async (_root, event) => event.hook_event_name === "Stop"
    ? new Promise((resolve) => { finish = () => resolve({ decision: "block", reason: "stale" }) }) : null)
  await f.start()
  await f.prompt()
  await f.send()
  assert.equal(typeof finish, "function")
  const admitted = f.prompt("ses_a", "msg_new", "another request")
  finish()
  await admitted
  await flush()
  assert.equal(f.synthetic.length, 0)
  assert.equal(f.calls.at(-1).event.prompt_id, "msg_new")
})

test("authorization wait bypasses the observer entirely", async (t) => {
  const f = fixture(t)
  await f.start()
  await f.prompt()
  await f.send("session.text.ended", { sessionID: "ses_a", text: "WORKFLOW_AUTHORIZATION_WAIT: phase-token request1" })
  f.ctx.session.get = async () => { throw Error("must not read while waiting") }
  await f.send()
  assert.equal(f.calls.length, 1)
  assert.equal(f.warnings.length, 0)
})

test("session deletion and cleanup release registrations and cancel work", async (t) => {
  const f = fixture(t)
  const cleanup = await f.start()
  await f.prompt()
  await f.send("session.deleted")
  await f.send()
  assert.equal(f.calls.length, 1)
  await cleanup()
  assert.equal(f.disposed(), true)
  assert.equal(f.calls[0].signal.aborted, true)
})

test("observer failure is reported without granting completion or restarting services", async (t) => {
  const f = fixture(t, async () => { throw Error("private process details") })
  await f.start()
  await f.prompt()
  assert.equal(f.warnings.length, 1)
  assert.ok(!f.warnings[0].includes("private process details"))
  assert.equal(f.synthetic.length, 0)
})

test("failed synthetic admission retains the report instead of losing deduplicated output", async (t) => {
  const f = fixture(t, async (_root, event) => event.hook_event_name === "Stop"
    ? { decision: "block", reason: "retained" } : null)
  await f.start()
  await f.prompt()
  const admit = f.ctx.session.synthetic
  let first = true
  f.ctx.session.synthetic = async (input) => {
    if (first) { first = false; throw Error("temporarily unavailable") }
    return admit(input)
  }
  await f.send()
  assert.equal(f.warnings.length, 1)
  await f.send()
  assert.equal(f.synthetic[0].text, "retained")
  assert.equal(f.calls.filter((call) => call.event.hook_event_name === "Stop").length, 1)
})

test("subprocess preserves literal JSON and uses explicit opencode runtime", async (t) => {
  const f = fixture(t)
  mkdirSync(join(f.directory, "scripts/hooks"), { recursive: true })
  writeFileSync(join(f.directory, "scripts/hooks/beads-consistency.py"),
    "import json,sys\nassert sys.argv[1:] == ['--runtime','opencode']\nprint(json.dumps(json.load(sys.stdin)))\n")
  const input = { prompt: '$(touch never-execute); "quoted" 中文\nnew line' }
  assert.deepEqual(await runObserver(f.directory, input, new AbortController().signal), input)
})

test("subprocess malformed output rejects instead of inventing a blocker", async (t) => {
  const f = fixture(t)
  mkdirSync(join(f.directory, "scripts/hooks"), { recursive: true })
  writeFileSync(join(f.directory, "scripts/hooks/beads-consistency.py"), "print('invalid JSON')\n")
  await assert.rejects(runObserver(f.directory, {}, new AbortController().signal))
})
