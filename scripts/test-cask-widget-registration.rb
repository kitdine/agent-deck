# Run with: brew ruby scripts/test-cask-widget-registration.rb <rendered-cask>
require "cask/cask_loader"
require "cask/config"
require "tmpdir"
require "fileutils"

# Use the real uninstall artifact and its shell script; replace only PlugInKit.
class WidgetRecordingCommand < SystemCommand
  class << self
    attr_accessor :tool
    def run(command, **options)
      raise "unexpected command" unless command.to_s == "/bin/sh"
      raise "uninstall must run as the current user" unless options[:sudo] == false
      raise "uninstall must propagate failure" unless options[:must_succeed]
      args = options.fetch(:args).map { |arg| arg.to_s.gsub("/usr/bin/pluginkit", tool) }
      raise "registration failed" unless system(command.to_s, *args)
      nil
    end
  end
end

def assert(condition, message)
  raise message unless condition
end

Dir.mktmpdir("agentdeck-widget-uninstall") do |temporary|
  appdir = File.join(temporary, "Applications with spaces")
  extension = File.join(appdir, "AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex")
  FileUtils.mkdir_p(extension)
  config = Cask::Config.new(explicit: { appdir: })
  cask = Cask::CaskLoader::FromContentLoader.new(File.read(ARGV.fetch(0))).load(config:)
  cask.config = config
  uninstall = cask.artifacts.find { |a| a.is_a?(Cask::Artifact::Uninstall) }
  assert(uninstall && uninstall.directives[:script], "missing Widget uninstall registration lifecycle")
  tool = File.join(temporary, "pluginkit")
  calls = File.join(temporary, "calls")
  File.write(tool, "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$WIDGET_RECORD\"\nexit \"${WIDGET_FAIL:-0}\"\n")
  FileUtils.chmod(0755, tool)
  WidgetRecordingCommand.tool = tool
  ENV["WIDGET_RECORD"] = calls
  invoke = -> { uninstall.send(:uninstall_script, uninstall.directives[:script], command: WidgetRecordingCommand) }
  2.times { invoke.call }
  assert(File.readlines(calls, chomp: true) == ["-r", extension] * 2, "uninstall targeted another path")
  FileUtils.rm_rf(extension)
  invoke.call
  assert(File.readlines(calls, chomp: true) == ["-r", extension] * 2, "missing extension triggered unregistration")
  FileUtils.mkdir_p(extension)
  ENV["WIDGET_FAIL"] = "7"
  begin
    invoke.call
    raise "uninstall ignored registration failure"
  rescue RuntimeError => error
    raise unless error.message == "registration failed"
  end
end
puts "Widget uninstall lifecycle regression passed"
