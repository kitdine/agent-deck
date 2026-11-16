# Run with: brew ruby scripts/test-cask-widget-registration.rb <rendered-cask>
#
# Loads the rendered cask through Homebrew and checks the artifacts Homebrew
# itself builds from it: that the App is an app artifact Homebrew publishes,
# that registration is a postflight step and no classic flight block remains,
# and that the uninstall stanza only quits the App and runs the widget script.
# Nothing the cask declares is executed here. The registration wrapper and the
# uninstall script are exercised against stand-ins by
# scripts/test-cask-migration.sh, and against real Homebrew in CI by
# scripts/test-cask-real-brew.sh.
require "cask/cask_loader"
require "cask/config"
require "tmpdir"
require "fileutils"

def assert(condition, message)
  raise message unless condition
end

content = File.read(ARGV.fetch(0))

Dir.mktmpdir("agentdeck-cask-declaration") do |temporary|
  appdir = File.join(temporary, "Applications with spaces")
  FileUtils.mkdir_p(appdir)

  config = Cask::Config.new(explicit: { appdir: })
  cask = Cask::CaskLoader::FromContentLoader.new(content).load(config:)
  cask.config = config

  classes = cask.artifacts.map(&:class)
  apps = cask.artifacts.grep(Cask::Artifact::App)
  assert(apps.length == 1 && apps.first.target.to_s == File.join(appdir, "AgentDeck.app"),
         "Homebrew must publish exactly AgentDeck.app into appdir, got #{apps.map { |a| a.target.to_s }}")
  assert(classes.count(Cask::Artifact::PreflightSteps) == 1, "the formula conflict check must stay a preflight step")
  assert(classes.count(Cask::Artifact::PostflightSteps) == 1, "registration must be one postflight step")
  # The classic Ruby blocks are deprecated and are how the withdrawn protocol
  # published the App itself.
  flight_blocks = cask.artifacts.select { |a| a.is_a?(Cask::Artifact::AbstractFlightBlock) }
  assert(flight_blocks.empty?, "the cask must not declare a classic flight block: #{flight_blocks.map(&:class)}")
  assert(classes.none? { |klass| klass.name.include?("UninstallPreflightSteps") || klass.name.include?("UninstallPostflightSteps") },
         "uninstall must not run helper steps: the receipt is left to go stale")

  uninstall = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Uninstall) }
  assert(uninstall, "missing uninstall stanza")
  # Homebrew adds an empty default `signal`; nothing else may be declared.
  assert((uninstall.directives.keys - [:quit, :script, :signal]).empty? && Array(uninstall.directives[:signal]).empty?,
         "uninstall may only quit the app and run the widget script, got #{uninstall.directives}")
  assert(Array(uninstall.directives[:quit]) == ["com.kitdine.agentdeck"], "uninstall must quit the App first")
  script = uninstall.directives[:script]
  assert(script[:executable] == "/bin/sh" && script[:sudo] == false, "uninstall must run /bin/sh as the current user")
  script_args = script[:args].map(&:to_s)
  assert(script_args.length == 5 && script_args[0] == "-c" && script_args[2] == "agentdeck-widget-uninstall",
         "uninstall script arguments are #{script_args.values_at(0, 2, 3, 4)}")
  assert(script_args[3] == File.join(appdir, "AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex"),
         "uninstall must name this appdir's extension, got #{script_args[3]}")
  assert(script_args[4] == "com.kitdine.agentdeck.widget", "uninstall must query the release widget identifier")
  shell = script_args[1]
  assert(shell.scan("/usr/bin/pluginkit").length == 2, "the script must query once and unregister once")
  %w[rm rmdir trash unlink mv sudo].each do |tool|
    assert(!shell.match?(/(^|[\s;&|(])#{tool}\s/), "the uninstall script must not remove files (#{tool})")
  end
  assert(!shell.include?("pluginkit -a"), "uninstall must never register an extension")

  zap = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Zap) }
  zapped = zap.directives.values.flatten.map(&:to_s)
  assert(zapped.none? { |path| path.include?("AgentDeck.app") || path.end_with?("/.agentdeck") || path.include?("Application Support") },
         "zap must not remove the App, AgentDeck state or installation records: #{zapped}")

  assert(Dir.children(appdir).empty?, "the test wrote into appdir")
end
puts "Cask declaration regression passed"
