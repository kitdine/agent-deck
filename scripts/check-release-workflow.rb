# frozen_string_literal: true

require "yaml"

abort "usage: check-release-workflow.rb <workflow>" unless ARGV.length == 1

workflow = YAML.safe_load(File.read(ARGV.fetch(0)), aliases: true)
jobs = workflow.fetch("jobs")
release = jobs.fetch("release")
homebrew = jobs.fetch("homebrew")
desktop = jobs.fetch("desktop")
cask = jobs.fetch("cask")

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
  "(!contains(github.ref_name, '-') || contains(github.ref_name, '-rc.'))) || " \
  "github.event_name == 'workflow_dispatch') }}"
raise "Homebrew job condition must allow only successful stable/RC pushes or manual dispatch" unless condition == expected_condition
raise "Homebrew job must depend on release" unless homebrew.fetch("needs") == "release"

select_formula = step(homebrew, "Select Homebrew formula")
raise "formula selection step must expose outputs" unless select_formula["id"] == "formula"
select_run = select_formula.fetch("run")
%w[formula_name=agentdeck formula_name=agentdeck-rc GITHUB_OUTPUT].each do |expected|
  require_text(select_run, expected, "formula selection")
end

render_run = step(homebrew, "Render Homebrew formula").fetch("run")
require_text(render_run, "scripts/render-homebrew-formula.sh", "formula rendering")
require_text(render_run, "steps.formula.outputs.name", "formula rendering")
verify_run = step(homebrew, "Verify Homebrew install and completions").fetch("run")
%w[formula_name brew\ install brew\ test bash\ --noprofile zsh\ -f fish\ --no-config].each do |expected|
  require_text(verify_run, expected, "Homebrew verification")
end

tap_checkout = step(homebrew, "Check out Homebrew tap")
raise "tap checkout must use HOMEBREW_TAP_TOKEN" unless tap_checkout.dig("with", "token") == "${{ secrets.HOMEBREW_TAP_TOKEN }}"
raise "tap checkout must fetch full history for safe branch reuse" unless tap_checkout.dig("with", "fetch-depth") == 0
update_run = step(homebrew, "Open formula update pull request").fetch("run")
require_text(update_run, "scripts/update-homebrew-tap-pr.sh", "tap PR update")
require_text(update_run, "steps.formula.outputs.name", "tap PR update")

# The desktop channel: built from the same commit, signed with a run-scoped
# credential, notarized, and uploaded beside the CLI archives of the same tag.
raise "desktop job must have contents: write" unless desktop.dig("permissions", "contents") == "write"
raise "desktop job must depend on release" unless desktop.fetch("needs") == "release"
raise "desktop job needs the macOS 26 runner" unless desktop.fetch("runs-on") == "macos-26"
desktop_checkout = step(desktop, "Check out repository")
raise "desktop checkout must not persist credentials" unless desktop_checkout.dig("with", "persist-credentials") == false

build_run = step(desktop, "Build the universal desktop candidate").fetch("run")
%w[make\ build-macos-release VERSION= COMMIT= APP_VERSION= APP_BUILD_NUMBER=].each do |expected|
  require_text(build_run, expected, "desktop candidate build")
end

certificate_step = step(desktop, "Import the Developer ID certificate")
certificate_run = certificate_step.fetch("run")
require_text(certificate_run, "security create-keychain", "certificate import")
require_text(certificate_run, "RUNNER_TEMP", "certificate import")
reject_text(certificate_run, "login.keychain", "certificate import")
raise "certificate import must read the certificate from secrets" unless
  certificate_step.dig("env", "MACOS_CERTIFICATE") == "${{ secrets.MACOS_CERTIFICATE }}"

notary_run = step(desktop, "Store the notarization credential").fetch("run")
require_text(notary_run, "xcrun notarytool store-credentials agentdeck-release", "notarization credential")
# Storing the profile into a run-scoped keychain and submitting without naming
# it is the failure this pairing check exists to make impossible: notarytool
# would read the login keychain and reject a credential this job just wrote.
signing_keychain = "$RUNNER_TEMP/agentdeck-signing.keychain-db"
require_text(notary_run, "--keychain \"#{signing_keychain}\"", "notarization credential")

package_step = step(desktop, "Sign, notarize, staple, and assess the desktop artifacts")
package_env = package_step.fetch("env")
raise "packaging must use the Developer ID identity from secrets" unless
  package_env.fetch("AGENTDECK_SIGN_IDENTITY") == "${{ secrets.MACOS_SIGN_IDENTITY }}"
raise "packaging must require a passing Gatekeeper assessment" unless
  package_env.fetch("AGENTDECK_REQUIRE_GATEKEEPER") == "1"
raise "a release must never skip notarization" if package_env.key?("AGENTDECK_SKIP_NOTARIZATION")
require_text(package_step.fetch("run"), "make package-macos-app", "desktop packaging")
packaging_keychain = package_step.dig("env", "AGENTDECK_NOTARY_KEYCHAIN").to_s
unless packaging_keychain.end_with?("/agentdeck-signing.keychain-db")
  raise "desktop packaging must submit against the same run-scoped keychain the credential was stored in"
end

upload_run = step(desktop, "Upload desktop assets").fetch("run")
%w[_universal.dmg _universal.zip _checksums.txt].each do |expected|
  require_text(upload_run, expected, "desktop asset upload")
end

keychain_step = step(desktop, "Remove the signing keychain")
raise "the signing keychain must be removed even when the job fails" unless keychain_step["if"] == "always()"

# The cask channel writes Casks/, never Formula/, and only after the DMG exists.
raise "cask job must have contents: read" unless cask.dig("permissions", "contents") == "read"
raise "cask job must depend on release and desktop" unless cask.fetch("needs") == %w[release desktop]
select_cask = step(cask, "Select Homebrew cask")
raise "cask selection step must expose outputs" unless select_cask["id"] == "cask"
%w[cask_token=agentdeck-app cask_token=agentdeck-app-rc GITHUB_OUTPUT].each do |expected|
  require_text(select_cask.fetch("run"), expected, "cask selection")
end
render_cask_run = step(cask, "Render Homebrew cask").fetch("run")
require_text(render_cask_run, "scripts/render-homebrew-cask.sh", "cask rendering")
require_text(render_cask_run, "steps.cask.outputs.token", "cask rendering")
verify_cask_run = step(cask, "Verify cask install, completions, and uninstall").fetch("run")
["brew install --cask", "brew uninstall --cask", "spctl --assess", "AgentDeckWidget.appex"].each do |expected|
  require_text(verify_cask_run, expected, "cask verification")
end
cask_pr_run = step(cask, "Open cask update pull request").fetch("run")
require_text(cask_pr_run, "scripts/update-homebrew-tap-pr.sh", "cask tap PR")
require_text(cask_pr_run, "\"$RELEASE_TAG\" cask", "cask tap PR")
