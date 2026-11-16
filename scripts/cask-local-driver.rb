# Test support for scripts/test-cask-migration.sh. Not part of any installation.
#
# Evaluates a rendered cask without Homebrew and reports the declarations the
# local checks need: the app artifact, the formula conflict step, the postflight
# registration step and the uninstall script. It runs nothing itself. The caller
# decides what to execute, so a declaration that names a system tool can be run
# against a stub instead.
#
# usage: ruby cask-local-driver.rb <cask> <appdir> <prefix> <action>
#   apps                 print every app artifact
#   binaries             print "<source>\t<target>" for every binary artifact
#   formulae             print the formulae the preflight step refuses
#   postflight-argv      print the registration step's argv, NUL-separated
#   postflight-options   print the registration step's sandbox declarations
#   uninstall-argv       print the uninstall script's argv, NUL-separated
#   uninstall-quit       print the bundle identifier the uninstall quits

cask_file, appdir, prefix, action = ARGV
abort "usage: cask-local-driver.rb <cask> <appdir> <prefix> <action>" if action.nil?

HOMEBREW_PREFIX = prefix

# Records what a *_steps block declares. Homebrew substitutes {{name}} template
# variables when it runs a step; the driver substitutes the ones it is given.
class StepRecorder
  attr_reader :paths, :runs

  def initialize
    @paths = []
    @runs = []
  end

  def if_path_exists(path)
    @paths << path
    yield if block_given?
  end

  def run(executable, args: [], **options)
    @runs << { executable: executable, args: args, options: options }
  end
end

class LocalCask
  attr_reader :token, :appdir, :apps, :binaries, :uninstall_directives, :formula_paths, :postflight_runs

  def initialize(token, appdir)
    @token = token
    @appdir = appdir
    @apps = []
    @binaries = []
    @postflight_runs = []
  end

  def version(value = nil)
    @version = value unless value.nil?
    @version
  end

  def app(source)
    @apps << source
  end

  def binary(source, target: nil)
    @binaries << [source, target || File.join(HOMEBREW_PREFIX, "bin", File.basename(source))]
  end

  def preflight_steps(&block)
    recorder = StepRecorder.new
    recorder.instance_eval(&block)
    @formula_paths = recorder.paths
  end

  def postflight_steps(&block)
    recorder = StepRecorder.new
    recorder.instance_eval(&block)
    @postflight_runs = recorder.runs
  end

  def uninstall(**directives)
    @uninstall_directives = directives
  end

  # The classic Ruby flight blocks are deprecated by Homebrew and are how the
  # withdrawn cask protocol published the App itself. A cask that brings one
  # back must fail these checks rather than be silently ignored.
  %i[preflight postflight uninstall_preflight uninstall_postflight].each do |stanza|
    define_method(stanza) { |*| abort "the cask declares a classic #{stanza} block" }
  end

  # Metadata stanzas that do not affect installation.
  %i[sha256 url name desc homepage depends_on conflicts_with caveats zap].each do |stanza|
    define_method(stanza) { |*_args, **_kwargs| nil }
  end
end

$local_cask_appdir = appdir
$local_cask = nil

def cask(token, &block)
  $local_cask = LocalCask.new(token, $local_cask_appdir)
  $local_cask.instance_eval(&block)
end
load cask_file
loaded = $local_cask

def registration_step(cask)
  abort "the cask must declare exactly one postflight step" unless cask.postflight_runs.length == 1
  cask.postflight_runs.first
end

def substitute(cask, value)
  value.to_s.gsub("{{appdir}}", cask.appdir)
end

case action
when "apps"
  loaded.apps.each { |source| puts source }
when "binaries"
  loaded.binaries.each { |source, target| puts "#{source}\t#{target}" }
when "formulae"
  loaded.formula_paths.each { |path| puts File.basename(path) }
when "postflight-argv"
  step = registration_step(loaded)
  print [step.fetch(:executable), *step.fetch(:args)].map { |value| substitute(loaded, value) }.join("\0")
when "postflight-options"
  registration_step(loaded).fetch(:options).sort_by { |name, _| name.to_s }.each do |name, value|
    puts "#{name}=#{value.inspect}"
  end
when "uninstall-argv"
  script = loaded.uninstall_directives.fetch(:script)
  abort "the uninstall script must not use sudo" unless script.fetch(:sudo) == false
  print [script.fetch(:executable), *script.fetch(:args).map(&:to_s)].join("\0")
when "uninstall-quit"
  puts loaded.uninstall_directives.fetch(:quit)
else
  abort "unknown action #{action}"
end
