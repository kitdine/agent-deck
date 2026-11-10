# Test support for scripts/test-cask-migration.sh. Not part of any installation.
#
# Evaluates a rendered cask without Homebrew and runs the cask's own preflight
# block, rollback block and uninstall script against directories the caller
# chooses. The commands those declarations run are executed for real, so the
# helper they name must be an agentdeck_acceptance build in a temporary
# account. Nothing here stages, downloads or quarantines anything, and no
# Homebrew-staged program is executed.
#
# usage: ruby cask-local-driver.rb <cask> <staged-path> <appdir> <prefix> <action>
#   install            run the publishing preflight block
#   install-rollback   run it, then the rollback block as a later failure would
#   uninstall          run the uninstall script, then the rollback block
#   binaries           print "<source>\t<target>" for every binary artifact
#   formulae           print the formulae the preflight step refuses
#   uninstall-argv     print the uninstall script's arguments, NUL-separated
require "open3"

cask_file, staged_path, appdir, prefix, action = ARGV
abort "usage: cask-local-driver.rb <cask> <staged-path> <appdir> <prefix> <action>" if action.nil?

HOMEBREW_PREFIX = prefix

module Cask
  class CaskError < StandardError; end
end

# The subset of SystemCommand the cask uses.
class LocalCommand
  Result = Struct.new(:status, :stdout, :stderr) do
    def success?
      status.success?
    end
  end

  def self.run(executable, args: [], **_options)
    stdout, stderr, status = Open3.capture3(executable.to_s, *args.map(&:to_s))
    $stderr.print(stderr)
    Result.new(status, stdout, stderr)
  rescue SystemCallError => e
    $stderr.puts(e.message)
    Result.new(Struct.new(:success?).new(false), "", e.message)
  end
end

# What a classic flight block can see.
class FlightContext
  attr_reader :token, :version, :staged_path, :appdir

  def initialize(token, version, staged_path, appdir)
    @token = token
    @version = version
    @staged_path = staged_path
    @appdir = appdir
  end

  def command
    LocalCommand
  end

  def system_command(executable, **options)
    result = LocalCommand.run(executable, **options)
    raise Cask::CaskError, "command failed: #{executable}" unless result.success?

    result
  end
end

# Records the paths the formula conflict step would test.
class StepRecorder
  attr_reader :paths

  def initialize
    @paths = []
  end

  def if_path_exists(path)
    @paths << path
  end

  def run(*); end
end

class LocalCask
  attr_reader :token, :staged_path, :appdir, :binaries, :flights, :uninstall_directives, :formula_paths

  def initialize(token, staged_path, appdir)
    @token = token
    @staged_path = staged_path
    @appdir = appdir
    @binaries = []
    @flights = {}
  end

  def version(value = nil)
    @version = value unless value.nil?
    @version
  end

  def binary(source, target: nil)
    @binaries << [source, target || File.join(HOMEBREW_PREFIX, "bin", File.basename(source))]
  end

  def preflight(&block)
    @flights[:preflight] = block
  end

  def uninstall_preflight(&block)
    @flights[:uninstall_preflight] = block
  end

  def preflight_steps(&block)
    recorder = StepRecorder.new
    recorder.instance_eval(&block)
    @formula_paths = recorder.paths
  end

  def uninstall(**directives)
    @uninstall_directives = directives
  end

  def app(*)
    abort "the cask declares an app stanza; the local driver cannot publish outside the ownership lock"
  end

  # Metadata stanzas that do not affect installation.
  %i[sha256 url name desc homepage depends_on conflicts_with caveats zap].each do |stanza|
    define_method(stanza) { |*_args, **_kwargs| nil }
  end

  def run_flight(key)
    block = flights.fetch(key)
    FlightContext.new(token, version, staged_path, appdir).instance_eval(&block)
  end
end

$local_cask_directories = [staged_path, appdir]
$local_cask = nil

def cask(token, &block)
  $local_cask = LocalCask.new(token, *$local_cask_directories)
  $local_cask.instance_eval(&block)
end
load cask_file
loaded = $local_cask

def uninstall_argv(cask)
  script = cask.uninstall_directives.fetch(:script)
  [script.fetch(:executable), *script.fetch(:args).map(&:to_s)]
end

begin
  case action
  when "install"
    loaded.run_flight(:preflight)
  when "install-rollback"
    loaded.run_flight(:preflight)
    loaded.run_flight(:uninstall_preflight)
  when "uninstall"
    system(*uninstall_argv(loaded))
    status = $?.exitstatus
    loaded.run_flight(:uninstall_preflight)
    exit status
  when "binaries"
    loaded.binaries.each { |source, target| puts "#{source}\t#{target}" }
  when "formulae"
    loaded.formula_paths.each { |path| puts File.basename(path) }
  when "uninstall-argv"
    print uninstall_argv(loaded).join("\0")
  else
    abort "unknown action #{action}"
  end
rescue Cask::CaskError, RuntimeError => e
  $stderr.puts "Error: #{e.message}"
  exit 1
end
