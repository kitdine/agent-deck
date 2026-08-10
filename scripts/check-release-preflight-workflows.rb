# frozen_string_literal: true

require "yaml"

abort "usage: check-release-preflight-workflows.rb <preflight-workflow> <release-workflow>" unless ARGV.length == 2

preflight_path, release_path = ARGV
preflight = YAML.safe_load(File.read(preflight_path), aliases: true)
release = YAML.safe_load(File.read(release_path), aliases: true)

def workflow_trigger(workflow)
  workflow["on"] || workflow[true]
end

def step(job, name)
  job.fetch("steps").find { |candidate| candidate["name"] == name } ||
    raise("missing workflow step: #{name}")
end

def require_text(text, expected, context)
  raise("#{context} must contain #{expected.inspect}") unless text.include?(expected)
end

def reject_text(text, rejected, context)
  raise("#{context} must not contain #{rejected.inspect}") if text.include?(rejected)
end

trigger = workflow_trigger(preflight)
raise "preflight must be manual workflow_dispatch only" unless trigger.is_a?(Hash) && trigger.keys == ["workflow_dispatch"]

inputs = trigger.fetch("workflow_dispatch").fetch("inputs")
%w[target_sha real_state_evidence_id].each do |name|
  input = inputs.fetch(name)
  raise "preflight input #{name} must be required" unless input["required"] == true
  raise "preflight input #{name} must be a string" unless input["type"] == "string"
end

raise "preflight default permissions must be contents: read" unless preflight.dig("permissions", "contents") == "read"
raise "preflight must not receive write permissions" if File.read(preflight_path).match?(/\bwrite\b/)

job = preflight.fetch("jobs").fetch("preflight")
raise "preflight must run on macos-15" unless job.fetch("runs-on") == "macos-15"

checkout = step(job, "Check out exact target")
raise "preflight checkout must use target_sha" unless checkout.dig("with", "ref") == "${{ inputs.target_sha }}"
raise "preflight checkout must not persist credentials" unless checkout.dig("with", "persist-credentials") == false

identity_run = step(job, "Require exact target SHA").fetch("run")
%w[EVENT_SHA TARGET_SHA git\ rev-parse\ HEAD].each do |expected|
  require_text(identity_run, expected.gsub("\\ ", " "), "preflight target identity")
end

l4_run = step(job, "Run technical preflight").fetch("run")
require_text(l4_run, "make release-verify", "technical preflight")

artifact_run = step(job, "Build and verify candidate artifacts").fetch("run")
require_text(artifact_run, "make release-artifact-verify", "candidate artifact verification")
require_text(artifact_run, "preflight-$TARGET_SHA", "candidate artifact verification")

manifest_run = step(job, "Write preflight evidence manifest").fetch("run")
require_text(manifest_run, "scripts/release-preflight-manifest.rb create", "preflight manifest")
%w[TARGET_SHA GITHUB_RUN_ID REAL_STATE_EVIDENCE_ID].each do |expected|
  require_text(manifest_run, expected, "preflight manifest")
end

upload = step(job, "Upload preflight evidence")
raise "preflight artifacts must use upload-artifact v4" unless upload.fetch("uses") == "actions/upload-artifact@v4"
raise "preflight artifact name must bind target_sha" unless upload.dig("with", "name") == "release-preflight-${{ inputs.target_sha }}"
raise "missing preflight artifacts must fail" unless upload.dig("with", "if-no-files-found") == "error"

raise "release default permissions must allow Actions reads" unless release.dig("permissions", "actions") == "read"
release_job = release.fetch("jobs").fetch("release")
raise "release job must allow Actions reads" unless release_job.dig("permissions", "actions") == "read"

gate_run = step(release_job, "Require successful release preflight").fetch("run")
[
  "release-preflight.yml",
  "head_sha=$GITHUB_SHA",
  "gh run download",
  "release-preflight-$GITHUB_SHA",
  "scripts/release-preflight-manifest.rb verify"
].each { |expected| require_text(gate_run, expected, "release preflight gate") }

final_run = step(release_job, "Verify version-specific release artifacts").fetch("run")
require_text(final_run, "make release-artifact-verify", "version-specific release verification")
reject_text(final_run, "release-verify", "version-specific release verification")
reject_text(File.read(release_path), "name: Verify release\n", "release workflow")
