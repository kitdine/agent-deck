# frozen_string_literal: true

require "yaml"

abort "usage: check-release-workflow.rb <workflow>" unless ARGV.length == 1

workflow = YAML.safe_load(File.read(ARGV.fetch(0)), aliases: true)
jobs = workflow.fetch("jobs")
release = jobs.fetch("release")
homebrew = jobs.fetch("homebrew")

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

raise "default workflow permissions must be contents: read" unless workflow.dig("permissions", "contents") == "read"
raise "release job must have contents: write" unless release.dig("permissions", "contents") == "write"
raise "Homebrew job must have contents: read" unless homebrew.dig("permissions", "contents") == "read"

release_checkout = step(release, "Check out repository")
raise "release checkout must not persist credentials" unless release_checkout.dig("with", "persist-credentials") == false
homebrew_checkout = step(homebrew, "Check out AgentDeck")
raise "Homebrew checkout must not persist credentials" unless homebrew_checkout.dig("with", "persist-credentials") == false

notes_run = step(release, "Extract annotated release notes").fetch("run")
require_text(notes_run, "scripts/release-notes-from-tag.sh", "release-note extraction")
publish_run = step(release, "Publish GitHub Release").fetch("run")
require_text(publish_run, "--notes-file", "release publication")
reject_text(File.read(ARGV.fetch(0)), "--notes-from-tag", "release workflow")

condition = homebrew.fetch("if").gsub(/\s+/, " ").strip
expected_condition = "${{ always() && " \
  "((github.event_name == 'push' && needs.release.result == 'success' && " \
  "!contains(github.ref_name, '-')) || github.event_name == 'workflow_dispatch') }}"
raise "Homebrew job condition must allow only successful stable pushes or manual dispatch" unless condition == expected_condition
raise "Homebrew job must depend on release" unless homebrew.fetch("needs") == "release"

render_run = step(homebrew, "Render Homebrew formula").fetch("run")
require_text(render_run, "scripts/render-homebrew-formula.sh", "formula rendering")
verify_run = step(homebrew, "Verify Homebrew install and completions").fetch("run")
%w[brew\ install brew\ test bash\ --noprofile zsh\ -f fish\ --no-config].each do |expected|
  require_text(verify_run, expected, "Homebrew verification")
end

tap_checkout = step(homebrew, "Check out Homebrew tap")
raise "tap checkout must use HOMEBREW_TAP_TOKEN" unless tap_checkout.dig("with", "token") == "${{ secrets.HOMEBREW_TAP_TOKEN }}"
update_run = step(homebrew, "Open formula update pull request").fetch("run")
require_text(update_run, "scripts/update-homebrew-tap-pr.sh", "tap PR update")
