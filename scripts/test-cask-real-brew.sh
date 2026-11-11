#!/usr/bin/env bash
set -euo pipefail

# This acceptance test executes downloaded cask helpers and real PlugInKit.
# It belongs exclusively on the disposable GitHub-hosted macOS runner.
if [[ ${1:-} != --ci || ${GITHUB_ACTIONS:-} != true || ${RUNNER_ENVIRONMENT:-} != github-hosted || ${RUNNER_OS:-} != macOS ]]; then
  echo 'This test requires --ci on a GitHub-hosted macOS runner.' >&2
  exit 2
fi
root=$(cd "$(dirname "$0")/.." && pwd)
fixture_app=${2:?usage: test-cask-real-brew.sh --ci <built-app>}
python3 - "$root" "$fixture_app" <<'PY'
import atexit
import select
import signal
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys
import tempfile
import time

root, fixture_app = map(Path, sys.argv[1:])
root, fixture_app = root.resolve(), fixture_app.resolve()
sys.path.insert(0, str(root / 'scripts'))
from cask_real_brew_fixture import prepare_fixture_app, run_fixture_command, verify_fixture_widget_registration

os.chdir(root)
workspace = Path(tempfile.mkdtemp(prefix='agentdeck-real-brew-', dir=os.environ['RUNNER_TEMP'])).resolve()
children = []

def stop_children():
    for child in children:
        if child.poll() is None:
            os.killpg(child.pid, signal.SIGKILL)
            child.wait()

atexit.register(stop_children)
base_env = os.environ.copy()
for name in list(base_env):
    if name.startswith(('AGENTDECK_ACCEPTANCE_', 'HOMEBREW_AGENTDECK_')) or name == 'HOMEBREW_DEVELOPER':
        del base_env[name]


def run(argv, *, env=None, success=True, timeout=180):
    return run_fixture_command(argv, env=env or base_env, success=success, timeout=timeout)


brew_repository = Path(run(['brew', '--repository']).strip())
helper = workspace / 'agentdeck'
run(['go', 'build', '-mod=vendor', '-trimpath', '-tags', 'agentdeck_acceptance', '-o', helper, './cmd/agentdeck'])
assert 'installation.acceptanceAccountHome' in run(['go', 'tool', 'nm', helper])
session_helper = workspace / 'session-helper'
run(['bash', root / 'scripts/run-go-test.sh', '-c', '-tags', 'agentdeck_acceptance',
     '-o', session_helper, './cmd/agentdeck'])
archives = {}
for version in ('1.2.3', '1.2.4'):
    bundle = workspace / version / 'AgentDeck.app'
    prepare_fixture_app(fixture_app, bundle, helper, version, run, env=base_env)
    archive = workspace / f'AgentDeck-{version}.zip'
    run(['/usr/bin/ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', bundle, archive])
    archives[version] = archive


def digest(path):
    path = Path(path)
    if not path.exists() and not path.is_symlink():
        return 'absent'
    checksum = hashlib.sha256()
    for item in [path, *sorted(path.rglob('*'))] if path.is_dir() else [path]:
        checksum.update(str(item.relative_to(path.parent)).encode())
        checksum.update(str(item.lstat().st_mode).encode())
        if item.is_symlink():
            checksum.update(os.readlink(item).encode())
        elif item.is_file():
            checksum.update(item.read_bytes())
    return checksum.hexdigest()


class Scenario:
    def __init__(self, name):
        self.directory = workspace / name
        self.prefix = self.directory / 'brew'
        self.home = self.directory / 'home'
        self.account = self.directory / 'account'
        self.appdir = self.directory / 'Applications'
        for path in (self.prefix / 'bin', self.home, self.account, self.appdir,
                     self.prefix / 'Library/Taps/agentdeck/homebrew-test/Casks'):
            path.mkdir(parents=True, exist_ok=True)
        shutil.copy2(brew_repository / 'bin/brew', self.prefix / 'bin/brew')
        (self.prefix / 'Library/Homebrew').symlink_to(brew_repository / 'Library/Homebrew')
        self.brew = self.prefix / 'bin/brew'
        self.cask = self.prefix / 'Library/Taps/agentdeck/homebrew-test/Casks/agentdeck-app.rb'
        self.app = self.appdir / 'AgentDeck.app'
        self.installed_helper = self.app / 'Contents/Helpers/agentdeck'
        self.support = self.account / 'Library/Application Support/AgentDeck'
        self.receipt = self.support / 'installation.json'
        self.attempts = self.support / 'installers/homebrew_cask/attempts'
        self.env = dict(base_env, HOME=str(self.home), HOMEBREW_CACHE=str(self.directory / 'cache'),
                        HOMEBREW_LOGS=str(self.directory / 'logs'), HOMEBREW_TEMP=str(self.directory / 'temp'),
                        HOMEBREW_NO_ANALYTICS='1', HOMEBREW_NO_AUTO_UPDATE='1', HOMEBREW_NO_ENV_HINTS='1',
                        HOMEBREW_NO_INSTALL_CLEANUP='1', HOMEBREW_NO_INSTALL_FROM_API='1',
                        HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=str(self.account))
        assert run([self.brew, '--prefix'], env=self.env).strip() == str(self.prefix)
        self.render('1.2.3')
        print(f'Running {name}', flush=True)

    def render(self, version, *, injection=''):
        archive = archives[version]
        checksums = self.directory / 'checksums.txt'
        checksums.write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  AgentDeck_v{version}_universal.dmg\n')
        run(['bash', root / 'scripts/render-homebrew-cask.sh', root / 'packaging/homebrew/agentdeck-app.rb.tmpl',
             'v' + version, checksums, self.cask])
        lines = self.cask.read_text().splitlines(keepends=True)
        index = next(i for i, line in enumerate(lines) if line.startswith('  url '))
        lines[index:index + 2] = [f'  url {json.dumps(archive.as_uri())}\n']
        content = ''.join(lines)
        if injection:
            # Add one failing artifact to the fixture; production declarations
            # and Homebrew's rollback/restore implementation remain real.
            content = content.replace('  binary "#{appdir}/AgentDeck.app/Contents/Helpers/agentdeck"',
                                      injection + '\n  binary "#{appdir}/AgentDeck.app/Contents/Helpers/agentdeck"')
        self.cask.write_text(content)

    def command(self, operation, *, extra=None, success=True, flags=()):
        env = dict(self.env, **(extra or {}))
        result = run([self.brew, operation, '--cask', *flags, f'--appdir={self.appdir}',
                      'agentdeck/test/agentdeck-app'], env=env, success=success)
        assert not (self.home / 'Library/Application Support/AgentDeck').exists()
        return result

    def identity(self):
        return json.loads(self.receipt.read_text())['installation_id']

    def locator(self):
        locators = list((self.prefix / 'Caskroom/agentdeck-app').glob('*/.agentdeck-attempt'))
        assert len(locators) == 1, locators
        return locators[0]

    def attempt(self):
        return self.attempts / json.loads(self.locator().read_text())['attempt_id']

    def unchanged(self, snapshot):
        assert (digest(self.app), digest(self.receipt)) == snapshot

    def snapshot(self):
        return digest(self.app), digest(self.receipt)

    def widget(self):
        verify_fixture_widget_registration(self.app, root / 'scripts/check-macos-widget-registration.sh', run)

    def uninstall(self):
        extension = self.app / 'Contents/PlugIns/AgentDeckWidget.appex'
        self.command('uninstall')
        assert not self.app.exists() and not self.receipt.exists()
        assert str(extension) not in run(['/usr/bin/pluginkit', '-m', '-A', '-D', '-vv'])

    def cleanup_attempt(self, attempt, *, success=True):
        record = json.loads((attempt / 'attempt.json').read_text())
        return run([attempt / 'helper', 'install', 'attempt', 'cleanup', '--attempt', record['attempt_id'],
                    '--token', record['token'], '--version', record['version'], '--executable', record['executable']],
                   env=self.env, success=success)

    def capture(self):
        return run([self.installed_helper, 'install', 'repair-identity', '--channel', 'homebrew_cask',
                    '--executable', self.installed_helper], env=self.env).strip()

    def successor(self, channel='homebrew_cask', session=False):
        source = self.appdir / '.successor.app'
        run(['/usr/bin/ditto', workspace / '1.2.3/AgentDeck.app', source])
        if session:
            shutil.copy2(session_helper, source / 'Contents/Helpers/agentdeck')
        manifest = self.directory / f'successor-{time.monotonic_ns()}.json'
        run([helper, 'install', 'register', '--channel', channel, '--executable', self.installed_helper,
             '--source', source, '--cleanup-manifest', manifest, '--takeover'], env=self.env)
        return manifest


    def start(self, operation, extra=None):
        log = (self.directory / 'async.log').open('w+')
        process = subprocess.Popen([str(self.brew), operation, '--cask', f'--appdir={self.appdir}',
                                    'agentdeck/test/agentdeck-app'], env=dict(self.env, **(extra or {})),
                                   stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        children.append(process)
        return process, log


def wait_reached(process, directory, log):
    deadline = time.monotonic() + 90
    while not (directory / 'reached').exists():
        if process.poll() is not None or time.monotonic() > deadline:
            process.kill()
            process.wait()
            log.seek(0)
            raise AssertionError(f'Pause was never reached: {log.read()}')
        time.sleep(0.05)


def finish(process, log, success):
    try:
        code = process.wait(timeout=180)
        log.seek(0)
        output = log.read()
        assert (code == 0) == success, output
        return output
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        log.close()


# Real installation is itself the feasibility gate. A failure aborts this suite.
s = Scenario('lifecycle')
s.command('install')
s.widget()
assert (s.prefix / 'bin/agentdeck').is_symlink()
for relative in ('etc/bash_completion.d/agentdeck', 'share/zsh/site-functions/_agentdeck',
                 'share/fish/vendor_completions.d/agentdeck.fish'):
    assert (s.prefix / relative).is_symlink()
old = s.attempt()
old_digest = (digest(old / 'manifest.json'), digest(old / 'helper'))
old_id = s.identity()
s.command('reinstall')
assert s.identity() != old_id and s.attempt() != old
assert (digest(old / 'manifest.json'), digest(old / 'helper')) == old_digest
s.render('1.2.4')
s.command('upgrade')
assert (s.app / 'Contents/Resources/version').read_text().strip() == '1.2.4'
s.widget()
s.uninstall()
# A different installation method can register after a genuine uninstall.
manifest = s.successor('local_app')
run([helper, 'install', 'unregister', '--channel', 'local_app', '--executable', s.installed_helper,
     '--expected-installation', s.identity(), '--cleanup-manifest', manifest, '--remove'], env=s.env)


# Reproduce the first CI failure deliberately: a missing completion artifact
# must roll back a published App even before its widget was ever registered.
s = Scenario('completion-artifact-rollback')
content = s.cask.read_text()
source = 'Contents/Resources/completions/agentdeck.bash'
assert content.count(source) == 1
s.cask.write_text(content.replace(source, 'Contents/Resources/completions/missing.bash'))
output = s.command('install', success=False)
assert 'symlink source' in output and 'missing.bash' in output, output
assert 'could not be verified for cleanup' not in output, output
assert not s.app.exists() and not s.receipt.exists()
assert list(s.attempts.glob('*/manifest.json')) and list(s.attempts.glob('*/helper'))
assert list(s.attempts.glob('*/cleaned'))


# Cleanup must use the retained program even if the final helper is corrupt.
s = Scenario('corrupt-final-helper')
s.command('install')
s.widget()
s.installed_helper.write_text('corrupt helper\n')
s.installed_helper.chmod(0o600)
s.uninstall()

# Account write failure must never fall back to HOME or publish the App.
s = Scenario('unwritable-account')
s.account.chmod(0o500)
try:
    s.command('install', success=False)
    assert not s.receipt.exists() and not s.app.exists()
finally:
    s.account.chmod(0o700)

# Publication succeeds, then its marker fails: preflight rescue must clean it.
s = Scenario('preflight-failure')
s.command('install', extra={'HOMEBREW_AGENTDECK_ACCEPTANCE_INSTALL_HOOK': 'attempt_complete=fail'}, success=False)
assert not s.app.exists() and not s.receipt.exists()
assert list(s.attempts.glob('*/helper'))
assert list(s.attempts.glob('*/manifest.json'))

# Upgrade rollback restores the old version through a fresh attempt, preserving
# the immutable O/N materials despite Homebrew's purge of the new Caskroom.
s = Scenario('upgrade-rollback')
s.command('install')
old = s.attempt()
old_id = s.identity()
old_manifest = digest(old / 'manifest.json')
old_helper = digest(old / 'helper')
s.render('1.2.4', injection='  binary "#{staged_path}/missing-artifact", target: "agentdeck-failure"')
s.command('upgrade', success=False)
assert (s.app / 'Contents/Resources/version').read_text().strip() == '1.2.3'
assert s.identity() != old_id and s.attempt() != old
assert digest(old / 'manifest.json') == old_manifest and digest(old / 'helper') == old_helper
restored = s.snapshot()
for attempt in s.attempts.iterdir():
    if attempt != s.attempt() and (attempt / 'manifest.json').exists():
        s.cleanup_attempt(attempt)
        s.unchanged(restored)
s.uninstall()


# Failure while removing the staging copy is within preflight rescue too.
s = Scenario('staging-cleanup-failure')
content = s.cask.read_text()
needle = '      FileUtils.rm_r(staged_copy) if File.exist?(staged_copy)'
assert content.count(needle) == 1
s.cask.write_text(content.replace(needle, '      raise "injected staging cleanup failure"'))
s.command('install', success=False)
assert not s.app.exists() and not s.receipt.exists()
assert list(s.attempts.glob('*/manifest.json'))

# A kill cannot execute rescue. Retained material outside Caskroom must still
# perform conditional cleanup after the entire process group has exited.
s = Scenario('killed-publisher')
pause = s.directory / 'pause'
pause.mkdir()
process, log = s.start('install', {'HOMEBREW_AGENTDECK_ACCEPTANCE_INSTALL_HOOK': f'register_recorded=wait:{pause}'})
wait_reached(process, pause, log)
assert s.app.exists() and s.receipt.exists()
attempt = s.attempt()
os.killpg(process.pid, signal.SIGKILL)
finish(process, log, False)
shutil.rmtree(s.prefix / 'Caskroom')
assert (attempt / 'helper').exists() and (attempt / 'manifest.json').exists()
s.cleanup_attempt(attempt)
assert not s.app.exists() and not s.receipt.exists()

# Interrupt rollback before it reaches the conditional callback, using a
# filesystem failure in an earlier real Binary artifact's uninstall phase.
s = Scenario('interrupted-rollback')
s.cask.write_text(s.cask.read_text().replace('  caveats <<~EOS', '''
  postflight do
    File.chmod(0500, "#{HOMEBREW_PREFIX}/bin")
    raise "injected postflight failure"
  end
  caveats <<~EOS'''))
try:
    s.command('install', success=False)
    assert s.app.exists() and s.receipt.exists()
    retained = list(s.attempts.glob('*/manifest.json'))
    assert len(retained) == 1
    attempt = retained[0].parent
    assert (attempt / 'helper').exists() and (attempt / 'attempt.json').exists()
finally:
    (s.prefix / 'bin').chmod(0o755)
s.cleanup_attempt(attempt)
assert not s.app.exists() and not s.receipt.exists()

# Insert B after N cleanup but before Homebrew restores O. R must reject B,
# including an unrecorded B, and O/N delayed cleanup must preserve it.
for damage in ('valid', 'missing'):
    s = Scenario(f'upgrade-successor-{damage}')
    s.command('install')
    old = s.attempt()
    pause = s.directory / 'pause'
    pause.mkdir()
    s.render('1.2.4', injection='  binary "#{staged_path}/missing-artifact", target: "agentdeck-failure"')
    content = s.cask.read_text()
    needle = '    unless rolled_back.success?'
    waiting = f'    File.write({json.dumps(str(pause / "reached"))}, "ready")\n' + \
              f'    sleep 0.05 until File.exist?({json.dumps(str(pause / "release"))})\n'
    assert content.count(needle) == 1
    s.cask.write_text(content.replace(needle, waiting + needle))
    process, log = s.start('upgrade')
    wait_reached(process, pause, log)
    try:
        assert not s.app.exists() and not s.receipt.exists()
        s.successor()
        if damage == 'missing':
            s.receipt.unlink()
        snapshot = s.snapshot()
    finally:
        (pause / 'release').touch()
    finish(process, log, False)
    s.unchanged(snapshot)
    for attempt in s.attempts.iterdir():
        if (attempt / 'manifest.json').exists():
            s.cleanup_attempt(attempt, success=(damage == 'valid'))
            s.unchanged(snapshot)

# Receipt loss and material loss are independent from a valid-identity repair.
for damage in ('missing', 'invalid'):
    for materials in ('kept', 'removed'):
        s = Scenario(f'receipt-{damage}-{materials}')
        s.command('install')
        original_id = s.identity()
        attempt = s.attempt()
        if damage == 'missing':
            s.receipt.unlink()
        else:
            s.receipt.write_text('{')
        if materials == 'removed':
            shutil.rmtree(attempt)
        snapshot = s.snapshot()
        material_digest = digest(attempt)
        caskroom_digest = digest(s.prefix / 'Caskroom')
        output = s.command('reinstall', success=False)
        assert 'HOMEBREW_AGENTDECK_REPAIR=1 brew reinstall --cask agentdeck-app' in output
        s.unchanged(snapshot)
        assert digest(s.prefix / 'Caskroom') == caskroom_digest and digest(attempt) == material_digest
        s.command('reinstall', extra={'HOMEBREW_AGENTDECK_REPAIR': '1'})
        assert s.identity() != original_id and s.attempt() != attempt
        assert digest(attempt) == material_digest
        s.widget()
        s.uninstall()


# Complete materials must not weaken a captured identity if the receipt is
# later lost or damaged. Homebrew must stop before removing its own record.
for successor in (False, True):
    for damage in ('missing', 'invalid'):
        s = Scenario(f'bound-complete-{successor}-{damage}')
        s.command('install')
        expected = s.capture()
        if successor:
            s.successor()
        if damage == 'missing':
            s.receipt.unlink()
        else:
            s.receipt.write_text('{')
        snapshot = s.snapshot()
        caskroom_digest = digest(s.prefix / 'Caskroom')
        output = s.command('reinstall', extra={'HOMEBREW_AGENTDECK_REPAIR': '1',
                                              'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION': expected}, success=False)
        assert f'receipt_{damage}' in output
        s.unchanged(snapshot)
        assert digest(s.prefix / 'Caskroom') == caskroom_digest

for missing in ('manifest', 'helper', 'locator'):
    s = Scenario(f'bound-repair-{missing}')
    s.command('install')
    attempt = s.attempt()
    original_id = s.identity()
    lost = {'manifest': attempt / 'manifest.json', 'helper': attempt / 'helper', 'locator': s.locator()}[missing]
    lost.unlink()
    snapshot = s.snapshot()
    material_digest = digest(attempt)
    s.command('reinstall', success=False)
    s.unchanged(snapshot)
    s.command('reinstall', extra={'HOMEBREW_AGENTDECK_REPAIR': '1'}, success=False)
    s.unchanged(snapshot)
    assert s.capture() == original_id
    s.unchanged(snapshot)
    s.command('reinstall', extra={'HOMEBREW_AGENTDECK_REPAIR': '1',
                                'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION': original_id})
    assert s.identity() != original_id and s.attempt() != attempt and digest(attempt) == material_digest
    s.widget()
    repaired = s.snapshot()
    original_manifest = attempt / 'manifest.json'
    if original_manifest.exists():
        output = run([helper, 'install', 'unregister', '--channel', 'homebrew_cask',
                      '--executable', s.installed_helper, '--expected-installation', original_id,
                      '--cleanup-manifest', original_manifest, '--remove'], env=s.env)
        assert 'skipped superseded' in output
        s.unchanged(repaired)
    s.uninstall()

# Existing unrecorded App, without a Homebrew installation record.
s = Scenario('unowned-app')
run(['/usr/bin/ditto', workspace / '1.2.3/AgentDeck.app', s.app])
snapshot = s.snapshot()
s.command('install', success=False)
s.unchanged(snapshot)
s.command('install', extra={'HOMEBREW_AGENTDECK_REPAIR': '1'})
s.uninstall()

# Authorization cannot replace another live owner; a deleted formula keg is
# stale and permits normal installation without repair or takeover.
for channel in ('make', 'local_app', 'homebrew_formula'):
    s = Scenario(f'foreign-{channel}')
    executable = s.directory / ('Cellar/agentdeck/1.2.3/bin/agentdeck' if channel == 'homebrew_formula' else 'other/bin/agentdeck')
    executable.parent.mkdir(parents=True)
    shutil.copy2(helper, executable)
    run([executable, 'install', 'register', '--channel', channel, '--executable', executable], env=s.env)
    snapshot = s.snapshot()
    executable_digest = digest(executable)
    for extra in ({}, {'HOMEBREW_AGENTDECK_REPAIR': '1'},
                  {'HOMEBREW_AGENTDECK_REPAIR': '1', 'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION': 'ab' * 16}):
        assert 'owned_by_other' in s.command('install', extra=extra, success=False)
        s.unchanged(snapshot)
        assert digest(executable) == executable_digest
    if channel == 'homebrew_formula':
        executable.unlink()
        s.command('install')
        s.uninstall()


# Bound repairs retain the captured identity across the complete reinstall,
# including Homebrew's attempt to restore an earlier version after failure.
for missing in ('manifest', 'helper', 'locator'):
    for timing in ('after-capture', 'after-uninstall'):
        for damage in ('valid', 'missing', 'invalid'):
            s = Scenario(f'bound-successor-{missing}-{timing}-{damage}')
            s.command('install')
            attempt = s.attempt()
            lost = {'manifest': attempt / 'manifest.json', 'helper': attempt / 'helper', 'locator': s.locator()}[missing]
            lost.unlink()
            expected = s.capture()
            repair = {'HOMEBREW_AGENTDECK_REPAIR': '1',
                      'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION': expected}
            pause = s.directory / 'pause'
            pause.mkdir()
            if timing == 'after-uninstall':
                # register_unlocked is after the uninstall's tolerated refusal
                # and before the new publisher takes ownership.
                repair['HOMEBREW_AGENTDECK_ACCEPTANCE_INSTALL_HOOK'] = f'register_unlocked=wait:{pause}'
                process, log = s.start('reinstall', repair)
                wait_reached(process, pause, log)
                assert s.identity() == expected
            s.successor()
            if damage == 'missing':
                s.receipt.unlink()
            elif damage == 'invalid':
                s.receipt.write_text('{')
            snapshot = s.snapshot()
            if timing == 'after-uninstall':
                (pause / 'release').touch()
                finish(process, log, False)
            else:
                s.command('reinstall', extra=repair, success=False)
            s.unchanged(snapshot)

# The original App/receipt survive a failure between exchange and commit.
s = Scenario('bound-publication-failure')
s.command('install')
attempt = s.attempt()
(attempt / 'manifest.json').unlink()
expected = s.capture()
snapshot = s.snapshot()
s.command('reinstall', extra={'HOMEBREW_AGENTDECK_REPAIR': '1',
                             'HOMEBREW_AGENTDECK_REPAIR_EXPECTED_INSTALLATION': expected,
                             'HOMEBREW_AGENTDECK_ACCEPTANCE_INSTALL_HOOK': 'register_published=fail'}, success=False)
s.unchanged(snapshot)

# I4: let A's helper return, install B, open B's actual Task 2 Store session,
# and then let Homebrew finish ALL of A's uninstall artifacts and purge.
for channel in ('local_app', 'homebrew_cask'):
    s = Scenario(f'delayed-uninstall-{channel}')
    pause = s.directory / 'pause'
    pause.mkdir()
    content = s.cask.read_text()
    needle = '                  exit "$status"'
    assert content.count(needle) == 1
    waiting = f'                  touch {str(pause / "reached")!r}\n' + \
              f'                  while [ ! -f {str(pause / "release")!r} ]; do sleep 0.05; done\n'
    s.cask.write_text(content.replace(needle, waiting + needle))
    s.command('install')
    old = s.attempt()
    process, log = s.start('uninstall')
    wait_reached(process, pause, log)
    assert not s.app.exists() and not s.receipt.exists()
    manifest = s.successor(channel, session=True)
    snapshot = s.snapshot()
    session_env = dict(s.env, AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=str(s.account),
                       AGENTDECK_OWNERSHIP_PROBE='hold-rw', AGENTDECK_OWNERSHIP_ROOT=str(s.account / '.agentdeck'))
    session = subprocess.Popen([str(s.installed_helper), '-test.run=^$'], env=session_env,
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
    children.append(session)
    try:
        assert select.select([session.stdout], [], [], 30)[0], 'B session did not become ready'
        assert session.stdout.readline() == 'ready\n'
        (pause / 'release').touch()
        finish(process, log, True)
        s.unchanged(snapshot)
        assert 'owner_running' in s.cleanup_attempt(old, success=False)
        s.unchanged(snapshot)
        output, errors = session.communicate('x', timeout=30)
        assert session.returncode == 0 and 'complete' in output, errors
        assert 'skipped superseded' in s.cleanup_attempt(old)
        s.unchanged(snapshot)
    finally:
        (pause / 'release').touch()
        if process.poll() is None:
            process.kill()
            process.wait()
        if session.poll() is None:
            session.kill()
            session.wait()

# Purge and --force must not turn missing cleanup materials into permission
# to remove the final App. Force outcome is checked through resulting state.
for missing in ('manifest', 'helper'):
    s = Scenario(f'force-uninstall-{missing}')
    s.command('install')
    attempt = s.attempt()
    (attempt / ('manifest.json' if missing == 'manifest' else 'helper')).unlink()
    snapshot = s.snapshot()
    result = subprocess.run([str(s.brew), 'uninstall', '--cask', '--force', f'--appdir={s.appdir}',
                             'agentdeck/test/agentdeck-app'], env=s.env, capture_output=True, text=True, timeout=180)
    s.unchanged(snapshot)
    assert result.returncode != 0 or not list((s.prefix / 'Caskroom/agentdeck-app').glob('*/.agentdeck-attempt'))
    print(f'{missing}: final App retained as an incomplete uninstall (brew exit {result.returncode})', flush=True)

print('Real Homebrew cask acceptance passed', flush=True)
shutil.rmtree(workspace)
PY
