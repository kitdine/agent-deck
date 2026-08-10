# frozen_string_literal: true

require "digest"
require "fileutils"
require "json"

SCHEMA = "agentdeck/release-preflight/v1"
SHA_PATTERN = /\A[0-9a-f]{40}\z/
EVIDENCE_PATTERN = /\Aurn:ce:agent-deck:evidence:[A-Za-z0-9._:-]+\z/

def usage!
  abort <<~USAGE
    usage:
      release-preflight-manifest.rb create <output> <repository> <target-sha> <run-id> <real-state-evidence-id> <checksums>
      release-preflight-manifest.rb verify <artifact-dir> <repository> <target-sha> <run-id>
  USAGE
end

def require_value(condition, message)
  raise message unless condition
end

def expected_artifacts(target_sha)
  version = "preflight-#{target_sha}"
  [
    "agentdeck_#{version}_darwin_amd64.tar.gz",
    "agentdeck_#{version}_darwin_arm64.tar.gz"
  ]
end

def parse_checksums(path, target_sha)
  expected = expected_artifacts(target_sha)
  entries = File.readlines(path, chomp: true).reject(&:empty?).map do |line|
    match = line.match(/\A([0-9a-f]{64})  ([A-Za-z0-9._-]+)\z/)
    require_value(match, "invalid checksum line: #{line.inspect}")
    { "name" => match[2], "sha256" => match[1] }
  end
  require_value(entries.map { |entry| entry.fetch("name") }.sort == expected,
                "preflight checksums must name exactly the arm64 and amd64 archives")
  entries.sort_by { |entry| entry.fetch("name") }
end

def validate_identity!(repository, target_sha, run_id, evidence_id = nil)
  require_value(repository.match?(/\A[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\z/),
                "invalid repository identity")
  require_value(target_sha.match?(SHA_PATTERN), "target SHA must be 40 lowercase hex characters")
  require_value(run_id.match?(/\A[1-9][0-9]*\z/), "workflow run ID must be a positive integer")
  return if evidence_id.nil?

  require_value(evidence_id.match?(EVIDENCE_PATTERN),
                "real-state evidence ID must be an AgentDeck completion-evidence URN")
end

def create_manifest(arguments)
  usage! unless arguments.length == 6
  output, repository, target_sha, run_id, evidence_id, checksums_path = arguments
  validate_identity!(repository, target_sha, run_id, evidence_id)
  entries = parse_checksums(checksums_path, target_sha)

  manifest = {
    "schema" => SCHEMA,
    "repository" => repository,
    "target_sha" => target_sha,
    "workflow_run_id" => run_id.to_i,
    "real_state_evidence_id" => evidence_id,
    "l4" => {
      "command" => "make release-verify",
      "result" => "pass"
    },
    "candidate_artifacts" => {
      "version" => "preflight-#{target_sha}",
      "files" => entries
    }
  }

  FileUtils.mkdir_p(File.dirname(output))
  File.write(output, JSON.pretty_generate(manifest) + "\n")
end

def verify_manifest(arguments)
  usage! unless arguments.length == 4
  artifact_dir, repository, target_sha, run_id = arguments
  validate_identity!(repository, target_sha, run_id)

  manifest_path = File.join(artifact_dir, "release-preflight.json")
  manifest = JSON.parse(File.read(manifest_path))
  require_value(manifest.fetch("schema") == SCHEMA, "unsupported preflight manifest schema")
  require_value(manifest.fetch("repository") == repository, "preflight repository mismatch")
  require_value(manifest.fetch("target_sha") == target_sha, "preflight target SHA mismatch")
  require_value(manifest.fetch("workflow_run_id") == run_id.to_i, "preflight run ID mismatch")
  validate_identity!(repository, target_sha, run_id, manifest.fetch("real_state_evidence_id"))
  require_value(manifest.dig("l4", "command") == "make release-verify", "preflight L4 command mismatch")
  require_value(manifest.dig("l4", "result") == "pass", "preflight L4 did not pass")

  artifacts = manifest.fetch("candidate_artifacts")
  require_value(artifacts.fetch("version") == "preflight-#{target_sha}",
                "preflight artifact version mismatch")
  entries = artifacts.fetch("files")
  require_value(entries.is_a?(Array), "preflight artifact files must be an array")
  expected = expected_artifacts(target_sha)
  require_value(entries.map { |entry| entry.fetch("name") }.sort == expected,
                "preflight manifest must name exactly the arm64 and amd64 archives")

  checksum_path = File.join(artifact_dir, "agentdeck_preflight-#{target_sha}_checksums.txt")
  require_value(parse_checksums(checksum_path, target_sha) == entries.sort_by { |entry| entry.fetch("name") },
                "preflight manifest and checksum file disagree")
  entries.each do |entry|
    archive = File.join(artifact_dir, entry.fetch("name"))
    require_value(File.file?(archive), "missing preflight artifact #{entry.fetch('name')}")
    require_value(Digest::SHA256.file(archive).hexdigest == entry.fetch("sha256"),
                  "preflight artifact checksum mismatch for #{entry.fetch('name')}")
  end

  expected_files = expected + [File.basename(checksum_path), File.basename(manifest_path)]
  actual_files = Dir.children(artifact_dir).sort
  require_value(actual_files == expected_files.sort, "preflight artifact bundle contains unexpected files")
  puts "verified release preflight run #{run_id} for #{target_sha} with #{manifest.fetch('real_state_evidence_id')}"
end

command = ARGV.shift
case command
when "create"
  create_manifest(ARGV)
when "verify"
  verify_manifest(ARGV)
else
  usage!
end
