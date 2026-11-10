# Run with: brew ruby scripts/test-cask-widget-registration.rb <rendered-cask>
#
# Loads the rendered cask through Homebrew and drives its real flight-block and
# uninstall artifacts. No AgentDeck helper is executed: every command the cask
# would run is recorded by a stand-in, and the uninstall script is run only
# along the paths where no helper exists. Widget unregistration itself happens
# inside the helper's ownership lock, so what this file proves is that the cask
# hands the whole removal to the helper and keeps no path-based fallback.
require "cask/cask_loader"
require "cask/config"
require "tmpdir"
require "fileutils"
require "open3"

def assert(condition, message)
  raise message unless condition
end

# Records what the cask asks Homebrew to run, and answers like the helper would.
class RecordingCommand
  Result = Struct.new(:ok, :stdout, :stderr) do
    def success? = ok
  end

  class << self
    attr_accessor :calls, :failures

    def reset(failures = {})
      self.calls = []
      self.failures = failures
    end

    def run(executable, args: [], **_options)
      args = args.map(&:to_s)
      calls << [executable.to_s, *args]
      key = args.first(3).join(" ").sub(/\Ainstall /, "")
      key = "ditto" if executable.to_s.end_with?("ditto")
      return Result.new(false, "", failures.fetch(key)) if failures.key?(key)

      stdout = if key.start_with?("attempt prepare")
        attempt = args[args.index("--attempt") + 1]
        "attempt_id=#{attempt}\ndirectory=/account/attempts/#{attempt}\n" \
          "helper=/account/attempts/#{attempt}/helper\nmanifest=/account/attempts/#{attempt}/manifest.json\n"
      else
        ""
      end
      Result.new(true, stdout, "")
    end

    def run!(executable, **options)
      result = run(executable, **options)
      raise "command failed: #{executable} #{options[:args]&.join(" ")}" unless result.success?

      result
    end

    def named(fragment) = calls.select { |call| call.join(" ").include?(fragment) }
  end
end

# The flight DSL builds its own command object, so the stand-in is installed on
# the DSL base class rather than passed in.
module RecordedFlight
  def command = RecordingCommand

  def system_command(executable, **options) = RecordingCommand.run!(executable, **options)
end
Cask::DSL::Base.prepend(RecordedFlight)

REPAIR = "HOMEBREW_AGENTDECK_REPAIR".freeze
EXPECTED = "HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION".freeze
ORIGINAL_ID = ("ab" * 16).freeze

def load_cask(content, appdir, environment = {})
  [REPAIR, EXPECTED].each { |name| ENV.delete(name) }
  environment.each { |name, value| ENV[name] = value }
  config = Cask::Config.new(explicit: { appdir: })
  cask = Cask::CaskLoader::FromContentLoader.new(content).load(config:)
  cask.config = config
  cask
ensure
  [REPAIR, EXPECTED].each { |name| ENV.delete(name) }
end

def flight(cask, key)
  cask.artifacts.find { |a| a.is_a?(Cask::Artifact::PreflightBlock) && a.directives.key?(key) }
end

def capture_stderr
  original = $stderr
  $stderr = StringIO.new
  yield
  $stderr.string
ensure
  $stderr = original
end

content = File.read(ARGV.fetch(0))

Dir.mktmpdir("agentdeck-cask-declaration") do |temporary|
  appdir = File.join(temporary, "Applications with spaces")
  FileUtils.mkdir_p(appdir)
  final_helper = File.join(appdir, "AgentDeck.app/Contents/Helpers/agentdeck")

  cask = load_cask(content, appdir)
  token = cask.token
  staged = cask.staged_path.to_s
  staged_helper = "#{staged}/AgentDeck.app/Contents/Helpers/agentdeck"
  locator = "#{staged}/.agentdeck-attempt"

  # --- declaration ---------------------------------------------------------

  classes = cask.artifacts.map(&:class)
  assert(classes.none? { |klass| klass <= Cask::Artifact::App }, "the cask must not publish the App with an app stanza")
  assert(classes.count(Cask::Artifact::PreflightSteps) == 1, "the formula conflict check must stay a preflight step")
  assert(flight(cask, :preflight), "missing the publishing preflight block")
  assert(flight(cask, :uninstall_preflight), "missing the rollback block")
  assert(classes.index(Cask::Artifact::PreflightSteps) < classes.index(Cask::Artifact::PreflightBlock),
         "the formula check must run before publication")
  assert(classes.index(Cask::Artifact::PreflightBlock) < classes.index(Cask::Artifact::Binary),
         "publication must run before the binary links")

  uninstall = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Uninstall) }
  assert(uninstall, "missing uninstall stanza")
  # Homebrew adds an empty default `signal`; nothing else may be declared.
  assert((uninstall.directives.keys - [:quit, :script, :signal]).empty? && Array(uninstall.directives[:signal]).empty?,
         "uninstall may only quit the app and run the helper script, got #{uninstall.directives}")
  script = uninstall.directives[:script]
  assert(script[:executable] == "/bin/sh" && script[:sudo] == false, "uninstall must run /bin/sh as the current user")
  script_args = script[:args].map(&:to_s)
  assert(script_args[3..7] == [token, cask.version.to_s, final_helper, locator, staged_helper],
         "uninstall script identity arguments are #{script_args[3..7]}")
  assert(script_args.length == 10, "uninstall carries repair arguments without authorization: #{script_args[10..]}")
  shell = script_args[1]
  assert(shell.include?("install attempt cleanup"), "uninstall does not use the conditional cleanup")
  %w[rm rmdir trash unlink pluginkit sudo].each do |tool|
    assert(!shell.match?(/(^|[\s;&|(])#{tool}\s/), "uninstall script must not call #{tool}")
  end

  zap = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Zap) }
  assert((zap.directives.keys - [:trash, :signal]).empty? && Array(zap.directives[:signal]).empty?,
         "zap may only trash preference and cache paths, got #{zap.directives.keys}")
  Array(zap.directives[:trash]).each do |path|
    assert(!path.to_s.include?("AgentDeck.app") && !path.to_s.include?("Application Support/AgentDeck") &&
           File.basename(path.to_s) != ".agentdeck", "zap must not remove #{path}")
  end

  code = content.lines.reject { |line| line.lstrip.start_with?("#") }.join
  %w[writable_paths writable_base Dir.home ENV["HOME"] ENV.fetch("HOME" expand_path --takeover].each do |forbidden|
    assert(!code.include?(forbidden), "the cask must not use #{forbidden}")
  end
  assert(!code.include?("Application Support"), "the cask must not derive an AgentDeck account path")
  # The flag is produced in exactly one place, guarded by the variable. The
  # other mentions validate arguments or explain a refusal and produce nothing.
  mentions = code.lines.select { |line| line.include?("--authorized-repair") }.map(&:strip)
  assert(mentions == ['repair_arguments << "--authorized-repair" if ENV["HOMEBREW_AGENTDECK_REPAIR"] == "1"',
                      'unless repair_arguments.include?("--authorized-repair") && expected.match?(/\A[0-9a-f]{32}\z/)',
                      'raise ::Cask::CaskError, "--expected-installation needs --authorized-repair and a valid installation ID"',
                      'if [ "$#" -eq 1 ] && [ "$1" = "--authorized-repair" ]; then'],
         "--authorized-repair must come only from HOMEBREW_AGENTDECK_REPAIR, found #{mentions}")

  # --- a successful install, then a later artifact fails -------------------

  RecordingCommand.reset
  flight(cask, :preflight).install_phase
  flight(cask, :uninstall_preflight).install_phase
  prepare, ditto, register, complete = RecordingCommand.calls
  assert(RecordingCommand.calls.length == 4, "preflight ran #{RecordingCommand.calls.map { |c| c[1..3] }}")
  attempt = prepare[prepare.index("--attempt") + 1]
  assert(attempt.match?(/\A[0-9a-f]{32}\z/), "attempt ID is #{attempt}")
  identity = ["--token", token, "--version", cask.version.to_s, "--executable", final_helper]
  assert(prepare == [staged_helper, "install", "attempt", "prepare", "--attempt", attempt, *identity, "--locator", locator],
         "prepare is #{prepare}")
  staged_copy = "#{appdir}/.AgentDeck.app.#{attempt}"
  assert(ditto == ["/usr/bin/ditto", "#{staged}/AgentDeck.app", staged_copy], "copy is #{ditto}")
  assert(register == [staged_helper, "install", "register", "--channel", "homebrew_cask", "--executable", final_helper,
                      "--source", staged_copy, "--cleanup-manifest", "/account/attempts/#{attempt}/manifest.json",
                      "--require-unowned"], "register is #{register}")
  assert(complete == [staged_helper, "install", "attempt", "complete", "--attempt", attempt], "complete is #{complete}")

  RecordingCommand.reset
  flight(cask, :uninstall_preflight).uninstall_phase
  flight(cask, :preflight).uninstall_phase
  assert(RecordingCommand.calls == [[staged_helper, "install", "attempt", "cleanup", "--attempt", attempt, *identity]],
         "rollback ran #{RecordingCommand.calls}")

  # A separately evaluated cask, as in a normal uninstall, rolls nothing back.
  RecordingCommand.reset
  fresh = load_cask(content, appdir)
  flight(fresh, :uninstall_preflight).uninstall_phase
  assert(RecordingCommand.calls.empty?, "a normal uninstall ran the rollback block: #{RecordingCommand.calls}")

  # A second preflight of one evaluation never reuses the first attempt.
  RecordingCommand.reset
  flight(cask, :preflight).install_phase
  second = RecordingCommand.calls.first[RecordingCommand.calls.first.index("--attempt") + 1]
  assert(second != attempt, "a second attempt reused the attempt ID")

  # --- failures inside the publishing block --------------------------------

  refusals = {
    "receipt_missing" => "HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask #{token}",
    "receipt_invalid" => "HOMEBREW_AGENTDECK_REPAIR=1 brew install --cask #{token}",
    "owned_by_other"  => "install repair-identity --channel homebrew_cask --executable '#{final_helper}'",
  }
  refusals.each do |reason, hint|
    cask = load_cask(content, appdir)
    RecordingCommand.reset("register --channel" => "install_claim_refused: refused\n  reason: #{reason}\n")
    raised = nil
    output = capture_stderr do
      flight(cask, :preflight).install_phase
    rescue Cask::CaskError => e
      raised = e
    end
    assert(raised, "a refused register did not fail the install (#{reason})")
    assert(output.include?(hint), "refusal #{reason} printed no repair command:\n#{output}")
    assert(output.include?("brew uninstall agentdeck"), "owned_by_other printed no migration") if reason == "owned_by_other"
    cleanup = RecordingCommand.calls.last
    assert(cleanup[1..4] == %w[install attempt cleanup --attempt], "#{reason}: no conditional cleanup after register")
    RecordingCommand.reset
    flight(cask, :uninstall_preflight).uninstall_phase
    assert(RecordingCommand.calls.empty?, "#{reason}: the rollback block repeated the block's own cleanup")
  end

  {
    "attempt prepare"  => 1, # nothing was asked to publish, so nothing to clean
    "ditto"            => 2,
    "attempt complete" => 5,
  }.each do |step, expected_calls|
    cask = load_cask(content, appdir)
    RecordingCommand.reset(step => "injected")
    begin
      flight(cask, :preflight).install_phase
      raise "preflight ignored a failing #{step}"
    rescue RuntimeError => e
      raise unless e.message.start_with?("command failed")
    end
    assert(RecordingCommand.calls.length == expected_calls, "#{step}: ran #{RecordingCommand.calls.map { |c| c[1..3] }}")
    cleaned = RecordingCommand.named("attempt cleanup")
    assert(cleaned.length == (step == "attempt complete" ? 1 : 0), "#{step}: cleanup calls #{cleaned}")
  end

  # --- repair authorization ------------------------------------------------

  {
    { REPAIR => "1" }                          => ["--authorized-repair"],
    { REPAIR => "1", EXPECTED => ORIGINAL_ID } => ["--authorized-repair", "--expected-installation", ORIGINAL_ID],
    { REPAIR => "yes" }                        => [],
  }.each do |environment, repair|
    cask = load_cask(content, appdir, environment)
    args = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Uninstall) }.directives[:script][:args].map(&:to_s)
    assert(args[10..] == repair, "#{environment}: uninstall repair arguments are #{args[10..]}")
    RecordingCommand.reset("attempt complete" => "injected")
    begin
      flight(cask, :preflight).install_phase
    rescue RuntimeError
      nil
    end
    register = RecordingCommand.named("install register").fetch(0)
    assert(register.last(repair.length + 1) == ["--require-unowned", *repair], "#{environment}: register is #{register}")
    RecordingCommand.named("attempt cleanup").each do |call|
      assert((call & ["--authorized-repair", "--expected-installation"]).empty?, "a rescue cleanup carried repair authority")
    end
    cask = load_cask(content, appdir, environment)
    RecordingCommand.reset
    flight(cask, :preflight).install_phase
    RecordingCommand.reset
    flight(cask, :uninstall_preflight).uninstall_phase
    rollback = RecordingCommand.calls.fetch(0)
    assert((rollback & ["--authorized-repair", "--expected-installation"]).empty?, "a rollback carried repair authority")
  end

  [
    { EXPECTED => ORIGINAL_ID },
    { REPAIR => "1", EXPECTED => "" },
    { REPAIR => "1", EXPECTED => "not-an-id" },
    { REPAIR => "1", EXPECTED => ORIGINAL_ID[0...-1] },
  ].each do |environment|
    cask = load_cask(content, appdir, environment)
    RecordingCommand.reset
    refused = false
    begin
      flight(cask, :preflight).install_phase
    rescue Cask::CaskError => error
      refused = error.message.include?("--expected-installation needs --authorized-repair")
    end
    assert(refused, "malformed binding was not refused: #{environment}")
    assert(RecordingCommand.calls.empty?, "malformed binding staged or published an installation")
  end

  # --- the uninstall script when no helper can run --------------------------

  run_script = lambda do |*repair|
    Open3.capture3("/bin/sh", *script_args.first(10), *repair)
  end
  _, stderr, status = run_script.call
  assert(status.exitstatus == 1 && stderr.include?("Nothing was removed") &&
         stderr.include?("HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask #{token}"),
         "an unverifiable uninstall must fail with the repair command: #{status.exitstatus} #{stderr}")
  _, stderr, status = run_script.call("--authorized-repair")
  assert(status.exitstatus.zero? && stderr.include?("skipped unverified") && stderr.include?("still installed"),
         "an authorized unverifiable uninstall must be tolerated and say so: #{status.exitstatus} #{stderr}")
  _, _, status = run_script.call("--authorized-repair", "--expected-installation", ORIGINAL_ID)
  assert(status.exitstatus == 1, "an identity-bound uninstall with no helper must fail")
  assert(!File.exist?(File.join(appdir, "AgentDeck.app")) && Dir.children(appdir).empty?, "the test wrote into appdir")
end
puts "Cask declaration and dispatch regression passed"
