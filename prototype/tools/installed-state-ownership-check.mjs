// Structural check for the installed-state-ownership CLI specimens.
// Run: node prototype/tools/installed-state-ownership-check.mjs
// Checks specimen consistency only; it does not exercise any AgentDeck runtime.
import { OWNERSHIP, OWNERSHIP_STATES } from "../src/ownership.js";

const failures = [];
const fail = (state, message) => failures.push(`${state}: ${message}`);

const expected = [
  "notice", "refused_state", "refused_client_config", "revoked",
  "doctor_installed", "doctor_not_installed", "doctor_formula_unclaimed", "doctor_refused", "doctor_command_owner",
  "claim_ok", "claim_owned_by_other", "claim_owner_running", "claim_not_current_keg",
];
for (const state of expected) if (!OWNERSHIP_STATES.includes(state)) fail(state, "missing state");
for (const state of OWNERSHIP_STATES) if (!expected.includes(state)) fail(state, "undeclared state");

for (const state of OWNERSHIP_STATES) {
  const s = OWNERSHIP[state];
  for (const key of ["command", "proves", "stdout", "stderr", "json"]) if (typeof s[key] !== "string") fail(state, `${key} must be a string`);
  if (![0, 1].includes(s.exit)) fail(state, "exit must be 0 or 1");
  for (const key of ["stdout", "stderr"]) if (s[key] && !s[key].endsWith("\n")) fail(state, `${key} must end with a newline`);
  if (/\t|\u001b/.test(s.stdout + s.stderr)) fail(state, "text must not contain tabs or escape codes");
  if (state === "notice") {
    if (!s.stderr.startsWith("note: not the installed AgentDeck (")) fail(state, "note must be the first stderr line");
    continue;
  }
  let env;
  try { env = JSON.parse(s.json); } catch (error) { fail(state, `json does not parse: ${error.message}`); continue; }
  if (/"\/|~\//.test(s.json)) fail(state, "json must not contain paths");
  for (const key of ["schema_version", "command", "generated_at", "data", "warnings", "partial"]) if (!(key in env)) fail(state, `envelope missing ${key}`);
  if (s.exit === 1) {
    if (!env.error || !env.error.code || !env.error.details || !env.error.details.reason) fail(state, "exit 1 requires error.code and error.details.reason");
    if (env.data !== null) fail(state, "error envelope data must be null");
    if (!s.stderr.startsWith(`${env.error.code}: ${env.error.message}\n`)) fail(state, "first stderr line must be '<code>: <message>'");
    if (s.stdout !== "") fail(state, "refusals print nothing on stdout");
  } else if (env.error) fail(state, "exit 0 must not carry an error");
  if (state.startsWith("doctor_")) {
    const checks = env.data.checks.map((c) => c.name);
    if (checks[0] !== "installation") fail(state, "installation must be the first check");
    if (!/^status: \S+\nmode: quick\nwarnings: \d+\nerrors: \d+\n/.test(s.stdout)) fail(state, "doctor text must start with the renderDoctorText header");
    for (const c of env.data.checks) {
      const line = c.code ? `${c.name}: ${c.status} (${c.code})` : `${c.name}: ${c.status}`;
      if (!s.stdout.split("\n").includes(line)) fail(state, `text lacks check line '${line}'`);
    }
    // A doctor report is partial exactly when it short-circuits on the state root:
    // state_missing (existing doctor.Service behaviour) or state_ownership_refused (new).
    const stateCode = (env.data.checks.find((c) => c.name === "state") || {}).code;
    const trailers = {
      state_missing: "checks_skipped: remaining diagnostics require a readable core database",
      state_ownership_refused: "checks_skipped: the requested state directory was refused; no other diagnostics ran",
    };
    const shortCircuit = stateCode in trailers;
    if (env.partial !== shortCircuit) fail(state, `partial must be ${shortCircuit} when the state check code is ${stateCode || "absent"}`);
    const skippedWarning = env.warnings.length === 1 && env.warnings[0] === "checks_skipped";
    if (shortCircuit !== skippedWarning) fail(state, "checks_skipped warning must accompany exactly the short-circuited reports");
    const lastLine = s.stdout.trimEnd().split("\n").pop();
    if (shortCircuit && lastLine !== trailers[stateCode]) fail(state, `text must end with '${trailers[stateCode]}'`);
    if (!shortCircuit && s.stdout.includes("checks_skipped:")) fail(state, "a complete report must not print checks_skipped");
    if (shortCircuit && checks.join(",") !== "installation,state") fail(state, "a short-circuited report contains exactly installation and state");
    if (state === "doctor_not_installed" && checks.includes("managed_command_owner")) fail(state, "managed_command_owner runs only for the installed program");
  }
}

if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}
console.log(`installed-state-ownership specimens: ${OWNERSHIP_STATES.length} states ok`);
