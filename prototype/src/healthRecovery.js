// health-recovery's prototype contract is shared by the menu-bar and CLI
// surfaces. Keep stable resource/reason/action tokens here so the specimens
// cannot drift while still looking individually plausible.
export const HEALTH_RECOVERY_STATES = [
  "baseline",
  "stateLive",
  "scanLive",
  "legacy",
  "ownerUnknown",
  "extensionStale",
  "discoveryFailed",
  "syncIncomplete",
  "combined",
];

const healthyChecks = [
  { name: "state_permissions", status: "ok" },
  { name: "database", status: "ok" },
];

const stateLive = {
  name: "state_lock",
  status: "warning",
  code: "lock_live",
  resource: "state",
  reason: "lock_live",
  action_kind: "retry",
};

const extensionStale = {
  name: "extensions",
  status: "warning",
  code: "extension_stale_inventory",
  resource: "extension_inventory",
  reason: "extension_stale_inventory",
  action_kind: "synchronize_inventory",
  recovery_command: "agentdeck extension scan",
  count: 1,
};

function health(checks) {
  const problems = checks.filter((check) => check.status !== "ok");
  return {
    available: true,
    status: problems.some((check) => check.status === "failed") ? "unhealthy" : "warning",
    healthy: problems.length === 0,
    problems: problems.length,
    warnings: problems.filter((check) => check.status === "warning").length,
    errors: problems.filter((check) => check.status === "failed").length,
    checks: [...healthyChecks, ...checks],
  };
}

export const HEALTH_RECOVERY = {
  stateLive: health([stateLive]),
  scanLive: health([
    {
      name: "scan_lock",
      status: "warning",
      code: "lock_live",
      resource: "scan",
      reason: "lock_live",
      action_kind: "retry",
    },
  ]),
  legacy: health([
    {
      name: "state_lock",
      status: "warning",
      code: "lock_legacy",
      resource: "state",
      reason: "lock_legacy",
      action_kind: "manual_prerequisite",
      copy_kind: "manual_prerequisite",
    },
  ]),
  ownerUnknown: health([
    {
      name: "scan_lock",
      status: "warning",
      code: "lock_owner_unknown",
      resource: "scan",
      reason: "lock_owner_unknown",
      action_kind: null,
    },
  ]),
  extensionStale: health([extensionStale]),
  discoveryFailed: health([
    {
      name: "extensions",
      status: "warning",
      code: "extension_discovery_failed",
      resource: "extension_inventory",
      reason: "extension_discovery_failed",
      action_kind: "manual_prerequisite",
    },
  ]),
  syncIncomplete: health([
    {
      name: "extensions",
      status: "failed",
      code: "extension_fingerprint_update_failed",
      resource: "extension_inventory",
      reason: "extension_fingerprint_update_failed",
      action_kind: "diagnose",
      diagnostic_command: "agentdeck extension doctor",
      inventory_committed: true,
    },
  ]),
  combined: health([stateLive, extensionStale]),
};

const jsonEnvelope = (command, details, message) => JSON.stringify({
  schema_version: 1,
  command,
  data: null,
  warnings: [],
  partial: false,
  error: details ? { code: details.code, message, details: details.details } : null,
}, null, 2);

export const CLI_HEALTH_RECOVERY = {
  stateLive: {
    command: "agentdeck session rebuild",
    text: `state_busy: AgentDeck state is busy.\n  resource: state\n  next: Wait for the current state operation to finish, then retry this command.\n  diagnose: agentdeck doctor`,
    json: jsonEnvelope("session.rebuild", {
      code: "state_busy",
      details: { resource: "state", reason: "lock_live", action_kind: "retry", recovery_command: null },
    }, "AgentDeck state is busy."),
  },
  scanLive: {
    command: "agentdeck session rebuild",
    text: `state_busy: AgentDeck scan is busy.\n  resource: scan\n  next: Wait for the current scan to finish, then retry this command.\n  diagnose: agentdeck doctor`,
    json: jsonEnvelope("session.rebuild", {
      code: "state_busy",
      details: { resource: "scan", reason: "lock_live", action_kind: "retry", recovery_command: null },
    }, "AgentDeck scan is busy."),
  },
  legacy: {
    command: "agentdeck doctor",
    text: `state_lock: warning (lock_legacy)\n  next: Confirm that no AgentDeck process is using this state directory.\n  manual prerequisite: Remove state.lock only after that confirmation.`,
    json: JSON.stringify({ resource: "state", reason: "lock_legacy", action_kind: "manual_prerequisite", recovery_command: null }, null, 2),
  },
  ownerUnknown: {
    command: "agentdeck doctor",
    text: `scan_lock: warning (lock_owner_unknown)\n  next: Do not remove scan.lock. Stop the owning AgentDeck process through its normal lifecycle or wait and diagnose again.`,
    json: JSON.stringify({ resource: "scan", reason: "lock_owner_unknown", action_kind: null, recovery_command: null }, null, 2),
  },
  extensionStale: {
    command: "agentdeck extension doctor",
    text: `status: warning\ndiagnostics: none\nstale inventory (1):\n  - codex:mcp:user:computer-use\nduplicate ids: none\ndrifted ids: none\nmanagement anomalies: none\nnext: Synchronize AgentDeck's extension inventory with current native discovery.\nrecovery: agentdeck extension scan\neffect: Updates AgentDeck's derived extension inventory and extension scan fingerprint only; does not modify Codex or Claude configuration or installed extensions.`,
    json: JSON.stringify({ resource: "extension_inventory", reason: "extension_stale_inventory", action_kind: "synchronize_inventory", recovery_command: "agentdeck extension scan" }, null, 2),
  },
  discoveryFailed: {
    command: "agentdeck extension doctor",
    text: `status: warning\ndiagnostics (1):\n  - Native extension discovery failed.\nstale inventory: unavailable\nduplicate ids: unavailable\ndrifted ids: unavailable\nmanagement anomalies: unavailable\nnext: Resolve the discovery error, then run agentdeck extension doctor again.`,
    json: JSON.stringify({ resource: "extension_inventory", reason: "extension_discovery_failed", action_kind: "manual_prerequisite", recovery_command: null }, null, 2),
  },
  syncIncomplete: {
    command: "agentdeck extension scan",
    text: `extension_sync_incomplete: AgentDeck extension inventory was updated, but its scan fingerprint was not.\n  effect: Inventory changes were committed; native client configuration was unchanged.\n  next: Verify inventory health before deciding whether to retry scan.\n  diagnose: agentdeck extension doctor`,
    json: jsonEnvelope("extension.scan", {
      code: "extension_sync_incomplete",
      details: { resource: "extension_inventory", reason: "extension_fingerprint_update_failed", action_kind: "diagnose", recovery_command: null, inventory_committed: true },
    }, "AgentDeck extension inventory was updated, but its scan fingerprint was not."),
  },
  combined: {
    command: "agentdeck doctor --full",
    text: `status: warning\nmode: full\nwarnings: 2\nerrors: 0\nstate_lock: warning (lock_live)\n  next: Let the current state operation finish, then retry the failed command.\nextensions: warning (extension_stale_inventory; count=1)\n  recovery: agentdeck extension scan\n  effect: Updates AgentDeck's derived extension inventory and extension scan fingerprint only; does not modify native client configuration.`,
    json: JSON.stringify({ checks: [stateLive, extensionStale] }, null, 2),
  },
};
