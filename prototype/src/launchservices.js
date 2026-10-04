// Candidate character specimens; OS acquisition and Widget operation are unproven.
const scope = "returned_host_urls_and_targeted_widget_matches";
const entry = (path, state = "canonical", version = "0.6.5", build = "24") => ({ path, canonical: state === "canonical", state, ...(state === "missing" ? {} : { version, build }) });
const hostPath = "/Applications/AgentDeck.app";
const widgetPath = `${hostPath}/Contents/PlugIns/AgentDeckWidget.appex`;
const source = (source, entries, reason) => ({ source, complete: !reason, ...(reason ? { reason } : {}), entries });
export const LAUNCHSERVICES = Object.fromEntries(["consistent", "conflict", "stale", "unknown", "not_applicable"].map((state) => {
  const applicable = state !== "not_applicable";
  const warning = ["conflict", "stale", "unknown"].includes(state);
  const hostEntries = [entry(hostPath)];
  if (state === "consistent") hostEntries.push(entry("/tmp/Matching.app", "matching_build"));
  if (state === "conflict") hostEntries.push(entry("/tmp/旧版.app", "different_build", "0.6.0", "21"));
  if (state === "stale") hostEntries.push(entry("/tmp/Obsolete.app", "missing"));
  if (state === "unknown") hostEntries.push(entry("/tmp/unsafe\n\u001b.app", "invalid_metadata", "", ""));
  const details = { applicable, ...(applicable ? {} : { applicability_reason: "gui_host_absent" }), scope, complete: applicable && state !== "unknown", host: source("host_application_urls", applicable ? hostEntries : [], state === "unknown" ? "invalid_metadata" : undefined), widget: source("widget_pluginkit", applicable ? [entry(widgetPath)] : [], state === "unknown" ? "timeout" : undefined) };
  const check = { name: "launchservices", status: warning ? "warning" : "ok", code: `launchservices_${state}`, resource: "launchservices_registration", reason: `launchservices_${state}`, registration_details: details };
  const lines = [`status: ${warning ? "degraded" : "healthy"}`, "mode: quick", `warnings: ${warning ? 1 : 0}`, "errors: 0", "extensions: ok", `launchservices: ${check.status} (${check.code})`];
  if (!applicable) lines.push("  applicability: gui_host_absent");
  else {
    lines.push("  scope: Returned host application URLs and targeted Widget PlugInKit matches only.");
    for (const key of ["host", "widget"]) {
      const src = details[key];
      lines.push(`  ${key}: ${src.complete ? "complete" : `incomplete (${src.reason})`}`);
      for (const e of src.entries) lines.push(`  ${key} entry: ${JSON.stringify(e.path)}; state=${e.state}${e.version ? `; version=${JSON.stringify(e.version)}; build=${JSON.stringify(e.build)}` : ""}`);
    }
    if (["conflict", "stale"].includes(state)) lines.push("  next: Verify which copy is obsolete and confirm the canonical app and Widget. Quit obsolete app copies normally. Only then manually unregister or remove the verified obsolete registration using its exact path; protect the canonical app and Widget.");
    else if (state === "unknown") lines.push("  next: Inspect the canonical app and Widget and the incomplete source before choosing any obsolete copy. Run agentdeck doctor --full again.");
    else lines.push("  next: Matching copies are not a version conflict within the returned-source scope.");
    lines.push("  limit: A later diagnosis confirms observed registrations only. Check the Widget separately.");
  }
  const data = { mode: "quick", status: warning ? "degraded" : "healthy", healthy: !warning, checks: [{ name: "extensions", status: "ok" }, check], problems: warning ? 1 : 0, warnings: warning ? 1 : 0, errors: 0 };
  return [state, { text: lines.join("\n") + "\n", json: JSON.stringify({ schema_version: 1, command: "doctor", generated_at: "2026-10-04T00:00:00Z", data, warnings: [], partial: false }, null, 2) }];
}));
