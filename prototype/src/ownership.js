// installed-state-ownership candidate character specimens.
// Text follows the shipped renderers: renderCommandErrorText-style error blocks on
// stderr, renderDoctorText's "status/mode/warnings/errors" header and
// "name: status (code)" check lines, and writeDoctorResult's partial trailer.
// Doctor specimens abbreviate unrelated existing checks. JSON never carries paths.
// Proves: copy, line order, field names, exit codes and stdout/stderr placement.
// Does not prove: the guard, the ownership lock, receipts or any runtime behavior.

const AT = "2026-11-05T12:00:00Z";
const envelope = (command, data, extra = {}) =>
  JSON.stringify({ schema_version: 1, command, generated_at: AT, data, warnings: [], partial: false, ...extra }, null, 2);
const errorEnvelope = (command, code, message, details) =>
  JSON.stringify({ schema_version: 1, command, generated_at: AT, data: null, warnings: [], partial: false, error: { code, message, details } }, null, 2);

const NOT_INSTALLED_LINES = (reason) => [
  `installation: warning (not_installed)`,
  `  reason: ${reason}`,
  `  state: isolated ~/.agentdeck-dev`,
  `  next: Expected for development builds. If this should be your installation, reinstall it with Homebrew, make install, or make install-macos-app.`,
];
const doctorHeader = (status, warnings, errors) => [`status: ${status}`, "mode: quick", `warnings: ${warnings}`, `errors: ${errors}`];
const doctorData = (status, checks, warnings, errors) => ({
  mode: "quick", status, healthy: status === "healthy", checks, problems: warnings + errors, warnings, errors,
});

// doctor.Service.Check returns early with Partial=true when the state root is missing;
// writeDoctorResult then adds this trailer and the checks_skipped warning.
const MISSING_STATE_TRAILER = "checks_skipped: remaining diagnostics require a readable core database";
const partial = { warnings: ["checks_skipped"], partial: true };

const STATE_REFUSED_MESSAGE = "This AgentDeck is not the installed program and cannot use the real state directory.";
const CONFIG_REFUSED_MESSAGE = "This AgentDeck is not the installed program and cannot change client configuration in your home directory.";

export const OWNERSHIP = {
  notice: {
    command: "go run ./cmd/agentdeck provider list",
    exit: 0,
    proves: "Not-installed TTY note: stderr only, once, before command output; absent from JSON, pipes and --quiet.",
    stderr: "note: not the installed AgentDeck (receipt_missing); using isolated state ~/.agentdeck-dev\n",
    stdout: "No providers.\n",
    json: "(unchanged: --format json output is identical to the installed program's; the note never appears in JSON, NDJSON, pipes or --quiet)",
  },
  refused_state: {
    command: "./dist/agentdeck --state-dir ~/.agentdeck scan",
    exit: 1,
    proves: "Ordinary command refused before any access to the real state directory.",
    stderr: [
      `state_ownership_refused: ${STATE_REFUSED_MESSAGE}`,
      "  resource: state_root",
      "  reason: receipt_other_executable",
      "  next: Omit --state-dir to use the isolated state, or run the installed agentdeck.",
      "  manual prerequisite: To work with real data, make an authorized SQLite backup copy and pass that copy's directory.",
      "  diagnose: agentdeck doctor",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("scan", "state_ownership_refused", STATE_REFUSED_MESSAGE, { resource: "state_root", reason: "receipt_other_executable" }),
  },
  refused_client_config: {
    command: "go run ./cmd/agentdeck telemetry enable --write-codex-config",
    exit: 1,
    proves: "Client configuration write refused; files stay byte-identical.",
    stderr: [
      `state_ownership_refused: ${CONFIG_REFUSED_MESSAGE}`,
      "  resource: client_config",
      "  reason: receipt_missing",
      "  next: Run the installed agentdeck, or run this build with a temporary HOME for development.",
      "  diagnose: agentdeck doctor",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("telemetry.enable", "state_ownership_refused", CONFIG_REFUSED_MESSAGE, { resource: "client_config", reason: "receipt_missing" }),
  },
  revoked: {
    command: "agentdeck watch",
    exit: 1,
    proves: "A long-lived former owner stops before its next round once ownership moved.",
    stderr: [
      `state_ownership_refused: ${STATE_REFUSED_MESSAGE}`,
      "  resource: state_root",
      "  reason: receipt_other_executable",
      "  next: Another installation now owns AgentDeck state. Run that installation's agentdeck.",
      "  diagnose: agentdeck doctor",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("watch", "state_ownership_refused", STATE_REFUSED_MESSAGE, { resource: "state_root", reason: "receipt_other_executable" }),
  },
  doctor_installed: {
    command: "agentdeck doctor",
    exit: 0,
    proves: "Installed program: installation first, managed command owner beside the Hook checks. Other checks abbreviated.",
    stderr: "",
    stdout: [
      ...doctorHeader("healthy", 0, 0),
      "installation: ok",
      "  channel: homebrew_cask",
      "state: ok",
      "managed_command_owner: ok",
    ].join("\n") + "\n",
    json: envelope("doctor", doctorData("healthy", [
      { name: "installation", status: "ok", reason: "homebrew_cask" },
      { name: "state", status: "ok" },
      { name: "managed_command_owner", status: "ok" },
    ], 0, 0)),
  },
  doctor_not_installed: {
    command: "./dist/agentdeck doctor",
    exit: 0,
    proves: "Not-installed program whose isolated state does not exist yet: the existing missing-state short circuit stays a partial report (checks_skipped), now preceded by installation. managed_command_owner does not run.",
    stderr: "",
    stdout: [...doctorHeader("degraded", 2, 0), ...NOT_INSTALLED_LINES("receipt_other_executable"), "state: warning (state_missing)", MISSING_STATE_TRAILER].join("\n") + "\n",
    json: envelope("doctor", doctorData("degraded", [
      { name: "installation", status: "warning", code: "not_installed", reason: "receipt_other_executable" },
      { name: "state", status: "warning", code: "state_missing" },
    ], 2, 0), partial),
  },
  doctor_formula_unclaimed: {
    command: "$(brew --prefix)/opt/agentdeck/bin/agentdeck doctor",
    exit: 0,
    proves: "An unclaimed Homebrew formula install names its claim command as recovery_command; its isolated state is missing, so the report is partial with checks_skipped.",
    stderr: "",
    stdout: [...doctorHeader("degraded", 2, 0), ...NOT_INSTALLED_LINES("receipt_missing"), "  recovery: agentdeck install claim", "state: warning (state_missing)", MISSING_STATE_TRAILER].join("\n") + "\n",
    json: envelope("doctor", doctorData("degraded", [
      { name: "installation", status: "warning", code: "not_installed", reason: "receipt_missing", recovery_command: "agentdeck install claim" },
      { name: "state", status: "warning", code: "state_missing" },
    ], 2, 0), partial),
  },
  doctor_refused: {
    command: "./dist/agentdeck doctor --state-dir ~/.agentdeck",
    exit: 0,
    proves: "Refused state root: partial report with exactly installation and state; nothing else runs; exit 0.",
    stderr: "",
    stdout: [
      ...doctorHeader("unhealthy", 1, 1),
      ...NOT_INSTALLED_LINES("receipt_other_executable"),
      "state: error (state_ownership_refused)",
      "  reason: receipt_other_executable",
      "  next: Omit --state-dir to use the isolated state, or run the installed agentdeck.",
      "checks_skipped: the requested state directory was refused; no other diagnostics ran",
    ].join("\n") + "\n",
    json: envelope("doctor", doctorData("unhealthy", [
      { name: "installation", status: "warning", code: "not_installed", reason: "receipt_other_executable" },
      { name: "state", status: "error", code: "state_ownership_refused", reason: "receipt_other_executable" },
    ], 1, 1), { warnings: ["checks_skipped"], partial: true }),
  },
  doctor_command_owner: {
    command: "agentdeck doctor",
    exit: 0,
    proves: "After a takeover, absolute-path Hooks of the old owner are named with a re-registration command. Other checks abbreviated.",
    stderr: "",
    stdout: [
      ...doctorHeader("degraded", 1, 0),
      "installation: ok",
      "  channel: make",
      "state: ok",
      "managed_command_owner: warning (managed_command_owner_mismatch)",
      "  reason: claude",
      "  next: Claude hooks point to another AgentDeck program and are ignored.",
      "  recovery: agentdeck usage hook setup --client claude",
    ].join("\n") + "\n",
    json: envelope("doctor", doctorData("degraded", [
      { name: "installation", status: "ok", reason: "make" },
      { name: "state", status: "ok" },
      { name: "managed_command_owner", status: "warning", code: "managed_command_owner_mismatch", reason: "claude", recovery_command: "agentdeck usage hook setup --client claude" },
    ], 1, 0)),
  },
  claim_ok: {
    command: "$(brew --prefix)/opt/agentdeck/bin/agentdeck install claim",
    exit: 0,
    proves: "Formula claim success.",
    stderr: "",
    stdout: "Claimed: this Homebrew formula installation now owns AgentDeck state.\n",
    json: envelope("install.claim", { channel: "homebrew_formula", takeover: false }),
  },
  claim_owned_by_other: {
    command: "agentdeck install claim",
    exit: 1,
    proves: "Another installation owns state; no path of that installation is shown.",
    stderr: [
      "install_claim_refused: Another AgentDeck installation owns AgentDeck state.",
      "  reason: owned_by_other",
      "  next: Keep the current installation, or rerun with --takeover after quitting it.",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("install.claim", "install_claim_refused", "Another AgentDeck installation owns AgentDeck state.", { reason: "owned_by_other" }),
  },
  claim_owner_running: {
    command: "agentdeck install claim --takeover",
    exit: 1,
    proves: "Takeover refused while any owner session holds the ownership lock.",
    stderr: [
      "install_claim_refused: The current owner is still using AgentDeck state.",
      "  reason: owner_running",
      "  next: Quit AgentDeck.app, stop any agentdeck watch or telemetry listener, pause Codex and Claude sessions, then retry.",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("install.claim", "install_claim_refused", "The current owner is still using AgentDeck state.", { reason: "owner_running" }),
  },
  claim_not_current_keg: {
    command: "/usr/local/Cellar/agentdeck/0.7.0/bin/agentdeck install claim",
    exit: 1,
    proves: "A retained or non-current keg cannot claim.",
    stderr: [
      "install_claim_refused: Only the current Homebrew formula installation can claim AgentDeck state.",
      "  reason: not_current_formula_keg",
      "  next: Run $(brew --prefix)/opt/agentdeck/bin/agentdeck install claim instead.",
    ].join("\n") + "\n",
    stdout: "",
    json: errorEnvelope("install.claim", "install_claim_refused", "Only the current Homebrew formula installation can claim AgentDeck state.", { reason: "not_current_formula_keg" }),
  },
};

export const OWNERSHIP_STATES = Object.keys(OWNERSHIP);
