import { execFile } from "node:child_process"
import { existsSync, realpathSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"

const MAX_SESSIONS = 256
const AUTHORIZATION_WAIT = /^WORKFLOW_AUTHORIZATION_WAIT:\s*[A-Za-z0-9][A-Za-z0-9._:/-]*(?:\s+[A-Za-z0-9_-]+)?[ \t]*$/m

// Never interpolate prompts or repository paths into a shell command.
export function runObserver(directory, event, signal) {
  return new Promise((resolve, reject) => {
    const child = execFile("python3", [join(directory, "scripts/hooks/beads-consistency.py"), "--runtime", "opencode"], {
      cwd: directory,
      timeout: 15_000,
      maxBuffer: 256 * 1024,
      signal,
      env: {
        ...process.env,
        AGENTDECK_WORKFLOW_HOOK: process.env.AGENTDECK_WORKFLOW_HOOK ?? join(homedir(), ".agentdeck/hooks/development-workflow/workflow_hook.py"),
      },
    }, (error, stdout) => {
      if (error) return reject(error)
      try {
        resolve(stdout.trim() ? JSON.parse(stdout) : null)
      } catch (error) {
        reject(error)
      }
    })
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify(event))
  })
}

/** Native V2 adapter. It observes only prompts admitted in this location. */
export async function setupBeadsConsistency(ctx, { observer = runObserver, warn = console.warn } = {}) {
  const location = ctx.location?.directory
  if (!location || !existsSync(join(location, ".agent-instructions/beads.md"))) return
  const directory = realpathSync(location)
  const sessions = new Map()
  const queues = new Map()
  const controller = new AbortController()
  const registrations = []
  const warnFailure = () => warn("[agentdeck.beads-consistency] Observer unavailable; no completion verdict was established.")
  const queue = (sessionID, action) => {
    const previous = queues.get(sessionID) ?? Promise.resolve()
    const next = previous.then(() => controller.signal.aborted ? undefined : action()).catch(warnFailure)
    queues.set(sessionID, next)
    void next.finally(() => {
      if (queues.get(sessionID) === next) queues.delete(sessionID)
    })
    return next
  }
  const cleanup = async () => {
    controller.abort()
    await Promise.allSettled(registrations.map((registration) => registration.dispose()))
    await Promise.allSettled([...queues.values()])
    sessions.clear()
  }
  try {
    registrations.push(await ctx.session.hook("prompt", async (event) => {
      if (!event.sessionID || !event.messageID || typeof event.prompt?.text !== "string") return
      if (sessions.size >= MAX_SESSIONS && !sessions.has(event.sessionID)) {
        const idle = [...sessions.keys()].find((id) => !queues.has(id))
        if (!idle) return
        sessions.delete(idle)
      }
      const turn = { promptID: event.messageID, reconciled: false, assistant: "" }
      sessions.set(event.sessionID, turn)
      await queue(event.sessionID, () => observer(directory, {
        hook_event_name: "UserPromptSubmit",
        session_id: event.sessionID,
        prompt_id: turn.promptID,
        prompt: event.prompt.text,
      }, controller.signal))
    }))
  } catch (error) {
    await cleanup()
    throw error
  }

  const receive = async (event) => {
    const id = event.data?.sessionID
    const turn = sessions.get(id)
    if (!turn) return
    if (event.location?.directory && event.location.directory !== location) return
    if (event.type === "session.deleted") {
      sessions.delete(id)
      return
    }
    if (event.type === "session.text.ended" && typeof event.data.text === "string") {
      turn.assistant = event.data.text
      return
    }
    if (event.type !== "session.execution.succeeded" || AUTHORIZATION_WAIT.test(turn.assistant)) return
    await queue(id, async () => {
      if (sessions.get(id) !== turn) return
      // A session can move without unloading this location's plugin. Never scan
      // the old checkout, and never attribute an older completed turn to a
      // queued new user prompt.
      const session = await ctx.session.get({ sessionID: id })
      if (session.location?.directory !== location || sessions.get(id) !== turn) return
      const messages = await ctx.session.context({ sessionID: id })
      const user = messages.findLast((message) => message.type === "user")
      if (user?.id !== turn.promptID || sessions.get(id) !== turn) return
      const assistant = messages.findLast((message) => message.type === "assistant")
      const text = assistant?.content?.filter((part) => part.type === "text").map((part) => part.text).join("\n") ?? turn.assistant
      if (AUTHORIZATION_WAIT.test(text)) return
      const output = turn.pending ?? await observer(directory, {
        hook_event_name: "Stop",
        session_id: id,
        prompt_id: turn.promptID,
        last_assistant_message: text,
        stop_hook_active: turn.reconciled,
      }, controller.signal)
      if (sessions.get(id) !== turn || controller.signal.aborted) return
      const reason = output?.decision === "block" ? output.reason : output?.systemMessage
      if (typeof reason !== "string" || !reason.trim()) return
      // The Python observer already deduplicated this report. Retain it until
      // admission succeeds so a transient client failure cannot lose it.
      turn.pending = output
      // This is a system diagnostic, NOT a new user command or phase grant.
      // V2 offers post-execution continuation, not a pre-completion Stop gate.
      const resume = output.decision === "block" && !turn.reconciled
      await ctx.session.synthetic({
        sessionID: id,
        text: reason,
        description: "AgentDeck Beads consistency diagnostic",
        metadata: { source: "agentdeck.beads-consistency", promptID: turn.promptID },
        delivery: "queue",
        resume,
      })
      turn.pending = undefined
      turn.reconciled = true
    })
  }

  const stream = (async () => {
    try {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        // Do not hold the event stream behind Git/Beads IO: an admitted newer
        // prompt must invalidate an old result before a synthetic is queued.
        void receive(event).catch(warnFailure)
      }
      if (!controller.signal.aborted) warnFailure()
    } catch {
      if (!controller.signal.aborted) warnFailure()
    }
  })()
  return async () => {
    await cleanup()
    await stream
  }
}
