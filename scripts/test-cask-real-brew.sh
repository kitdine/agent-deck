#!/usr/bin/env bash
set -euo pipefail

# This acceptance test lets real Homebrew stage and run the cask's helper and
# queries real PlugInKit. It belongs exclusively on the disposable
# GitHub-hosted macOS runner; on a developer machine the staged helper triggers
# Gatekeeper. It never registers an extension: unregistering an extension that
# is registered is verified by the release workflow, not here.
#
# Five cases, and nothing else: install; upgrade, including an upgrade whose
# registration is refused while a session is open; uninstall followed by
# another installation taking the stale receipt; another valid owner; and a
# brew HOME that is not the account directory.
if [[ ${1:-} != --ci || ${GITHUB_ACTIONS:-} != true || ${RUNNER_ENVIRONMENT:-} != github-hosted || ${RUNNER_OS:-} != macOS ]]; then
  echo 'This test requires --ci on a GitHub-hosted macOS runner.' >&2
  exit 2
fi
root=$(cd "$(dirname "$0")/.." && pwd)
fixture_app=${2:?usage: test-cask-real-brew.sh --ci <built-app>}
python3 - "$root" "$fixture_app" <<'PY'
import atexit
import re
import shlex
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
from cask_real_brew_fixture import prepare_fixture_app, run_fixture_command

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
    def __init__(self, name, account=None):
        self.directory = workspace / name
        self.prefix = self.directory / 'brew'
        self.home = self.directory / 'home'
        # The step may write below brew's HOME only, and the helper writes below
        # its account directory, so the two are one directory unless a case
        # says otherwise.
        self.account = self.directory / account if account else self.home
        self.appdir = self.directory / 'Applications'
        for path in (self.prefix / 'bin', self.home, self.account, self.appdir,
                     self.prefix / 'Library/Taps/agentdeck/homebrew-test/Casks'):
            path.mkdir(parents=True, exist_ok=True)
        shutil.copy2(brew_repository / 'bin/brew', self.prefix / 'bin/brew')
        (self.prefix / 'Library/Homebrew').symlink_to(brew_repository / 'Library/Homebrew')
        self.brew = self.prefix / 'bin/brew'
        self.cask = self.prefix / 'Library/Taps/agentdeck/homebrew-test/Casks/agentdeck-app.rb'
        self.caskroom = self.prefix / 'Caskroom/agentdeck-app'
        self.app = self.appdir / 'AgentDeck.app'
        self.installed_helper = self.app / 'Contents/Helpers/agentdeck'
        self.support = self.account / 'Library/Application Support/AgentDeck'
        self.receipt = self.support / 'installation.json'
        self.env = dict(base_env, HOME=str(self.home), HOMEBREW_CACHE=str(self.directory / 'cache'),
                        HOMEBREW_LOGS=str(self.directory / 'logs'), HOMEBREW_TEMP=str(self.directory / 'temp'),
                        HOMEBREW_NO_ANALYTICS='1', HOMEBREW_NO_AUTO_UPDATE='1', HOMEBREW_NO_ENV_HINTS='1',
                        HOMEBREW_NO_INSTALL_CLEANUP='1',
                        HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=str(self.account))
        # A program run directly, outside brew, reads the unprefixed variable.
        self.direct_env = dict(self.env, AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=str(self.account))
        # The fully-qualified fixture cask still loads from our local tap. Keep
        # official formula metadata in default API mode: forcing Git checkouts
        # makes upgrade's unrelated pkgconf check clone all of homebrew/core.
        self.env.pop('HOMEBREW_NO_INSTALL_FROM_API', None)
        assert run([self.brew, '--prefix'], env=self.env).strip() == str(self.prefix)
        self.render('1.2.3')
        print(f'Running {name}', flush=True)

    def render(self, version):
        archive = archives[version]
        checksums = self.directory / 'checksums.txt'
        checksums.write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  AgentDeck_v{version}_universal.dmg\n')
        run(['bash', root / 'scripts/render-homebrew-cask.sh', root / 'packaging/homebrew/agentdeck-app.rb.tmpl',
             'v' + version, checksums, self.cask])
        lines = self.cask.read_text().splitlines(keepends=True)
        index = next(i for i, line in enumerate(lines) if line.startswith('  url '))
        lines[index:index + 2] = [f'  url {json.dumps(archive.as_uri())}\n']
        self.cask.write_text(''.join(lines))

    def argv(self, operation):
        # Homebrew records the appdir at install time and reads it back on
        # uninstall, which does not accept the option at all.
        appdir = [] if operation == 'uninstall' else [f'--appdir={self.appdir}']
        return [self.brew, operation, '--cask', *appdir, 'agentdeck/test/agentdeck-app']

    def command(self, operation, *, success=True, timeout=180):
        output = run(self.argv(operation), env=self.env, success=success, timeout=timeout)
        assert 'deprecat' not in output.lower(), output
        return output

    def record(self):
        return json.loads(self.receipt.read_text())

    def version(self):
        return (self.app / 'Contents/Resources/version').read_text().strip()

    def versions(self):
        return sorted(path.name for path in self.caskroom.iterdir() if not path.name.startswith('.')) \
            if self.caskroom.exists() else []

    def identity(self, program=None):
        """What a program reports about itself: (installed, reason or channel)."""
        program = program or self.installed_helper
        data = json.loads(run([program, '--format', 'json', 'desktop', 'state-location'], env=self.direct_env))['data']
        return data['installed'], data.get('channel') or data.get('reason')

    def materials(self):
        return sorted(str(path.relative_to(self.support)) for path in self.support.rglob('*')) \
            if self.support.exists() else []

    def copy_helper(self, name):
        target = self.directory / name / 'bin/agentdeck'
        target.parent.mkdir(parents=True)
        shutil.copy2(helper, target)
        return target


def no_extension_was_registered(output):
    # This suite never registers an extension, and neither may the cask.
    assert 'pluginkit -a' not in output and 'pluginkit -v -a' not in output, output


# 1. Install. The real step sandbox runs the wrapper and the helper; a failure
#    here means the design's premise does not hold on this runner.
s = Scenario('lifecycle')
output = s.command('install')
no_extension_was_registered(output)
assert f'registered {s.installed_helper} (homebrew_cask)' in output, output
first = s.record()
assert (first['channel'], first['executable']) == ('homebrew_cask', str(s.installed_helper)), first
assert s.identity() == (True, 'homebrew_cask')
assert s.version() == '1.2.3' and s.versions() == ['1.2.3']
assert (s.prefix / 'bin/agentdeck').is_symlink()
for relative in ('etc/bash_completion.d/agentdeck', 'share/zsh/site-functions/_agentdeck',
                 'share/fish/vendor_completions.d/agentdeck.fish'):
    assert (s.prefix / relative).is_symlink()
# The cask keeps no installer materials, and the wrapper's buffer is gone.
assert s.materials() == ['installation.json', 'ownership.lock'], s.materials()
# mktemp's default name; Homebrew's own temporary files are named otherwise.
leftovers = [str(path) for path in s.directory.rglob('tmp.*')
             if path.is_file() and re.fullmatch(r'tmp\.[A-Za-z0-9]{8,}', path.name)]
assert not leftovers, leftovers

# 2. Upgrade. State the user owns is created first and must still be usable.
run([s.installed_helper, 'provider', 'list'], env=s.direct_env)
state = s.home / '.agentdeck'
assert state.is_dir() and any(state.iterdir())
s.render('1.2.4')
output = s.command('upgrade')
no_extension_was_registered(output)
second = s.record()
assert s.version() == '1.2.4' and s.versions() == ['1.2.4']
assert second['executable'] == first['executable'] and second['channel'] == 'homebrew_cask'
assert second['installation_id'] != first['installation_id'] and second['installed_at'] >= first['installed_at']
assert s.identity() == (True, 'homebrew_cask')
assert 'official' in run([s.installed_helper, 'provider', 'list'], env=s.direct_env)

# 3. Uninstall, then another installation takes the stale receipt. The fixture
#    extension was never registered, so the script must skip it and succeed.
before = s.receipt.read_bytes()
retained = s.copy_helper('retained')
output = s.command('uninstall')
no_extension_was_registered(output)
assert not s.app.exists() and not s.caskroom.exists(), (s.app.exists(), s.versions())
assert not (s.prefix / 'bin/agentdeck').is_symlink()
assert s.receipt.read_bytes() == before
assert s.identity(retained) == (False, 'receipt_missing')
successor = s.copy_helper('successor')
run([successor, 'install', 'register', '--channel', 'make', '--executable', successor], env=s.direct_env)
taken = s.record()
assert (taken['channel'], taken['executable']) == ('make', str(successor)), taken
assert s.identity(successor) == (True, 'make')


# 2, refused. A real session of the installed program is open while Homebrew
#    upgrades. Homebrew has already replaced the files when the registration
#    step asks for the lock, so the step fails and Homebrew must put the old
#    version back. The session stays open until brew has finished: releasing it
#    earlier would only prove the first refusal, not the restored state.
s = Scenario('upgrade-refused')
s.command('install')
run([s.installed_helper, 'provider', 'list'], env=s.direct_env)
before_receipt = s.receipt.read_bytes()
before_app = digest(s.app)
log = (s.directory / 'session.log').open('w+')
session = subprocess.Popen([str(s.installed_helper), 'watch', '--domains', 'usage'], env=s.direct_env,
                           stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
children.append(session)
deadline = time.monotonic() + 60
while (s.directory / 'session.log').stat().st_size == 0:
    assert session.poll() is None and time.monotonic() < deadline, 'the session never started'
    time.sleep(0.1)
s.render('1.2.4')
output = s.command('upgrade', success=False, timeout=600)
session_survived = session.poll() is None
refusals = output.count('reason: owner_running')
print(f'upgrade-refused: owner_running reported {refusals} time(s); '
      f'old registration step ran again during restore: {refusals > 1}; '
      f'receipt changed: {s.receipt.read_bytes() != before_receipt}; '
      f'restored version: {s.version() if s.app.exists() else "no App"}; Caskroom: {s.versions()}', flush=True)
assert session_survived, 'the session ended before brew finished, so the refusal was not held to the end'
assert refusals >= 1, output
assert 'stop any agentdeck watch' in output, output
# Any failure below means Homebrew's own restore does not give the state the
# design expects. That goes back to design; it is not a reason to exempt the
# step from the lock or to add a recovery protocol here.
assert s.app.exists(), 'DESIGN: a refused upgrade left no App\n' + output
assert s.version() == '1.2.3' and digest(s.app) == before_app, 'DESIGN: the old App was not restored intact\n' + output
assert s.versions() == ['1.2.3'], f'DESIGN: Caskroom after a refused upgrade is {s.versions()}\n' + output
assert s.receipt.read_bytes() == before_receipt, 'DESIGN: a refused upgrade changed the receipt\n' + output
assert s.record()['executable'] == str(s.installed_helper)
os.killpg(session.pid, signal.SIGTERM)
session.wait(timeout=30)
log.close()
assert s.identity() == (True, 'homebrew_cask')
# With the session gone, the same command succeeds.
s.command('upgrade')
assert s.version() == '1.2.4' and s.versions() == ['1.2.4'] and s.identity() == (True, 'homebrew_cask')
assert s.record()['installation_id'] != json.loads(before_receipt)['installation_id']
s.command('uninstall')


# 4. Another valid owner at another path. The refusal must come from the
#    registration step itself, not from the formula conflict step.
s = Scenario('other-owner')
owner = s.copy_helper('owner')
run([owner, 'install', 'register', '--channel', 'make', '--executable', owner], env=s.direct_env)
before_receipt, before_owner = s.receipt.read_bytes(), owner.read_bytes()
output = s.command('install', success=False)
assert 'CLI-only' not in output, output
for line in ('install_claim_refused: Another AgentDeck installation owns AgentDeck state.',
             '  reason: owned_by_other',
             '  next: Keep the current installation, or uninstall it with its original installation method, '
             'then retry this Homebrew operation for agentdeck-app.'):
    assert line in output, output
assert 'takeover' not in output, output
assert not s.app.exists() and not s.caskroom.exists(), (s.app.exists(), s.versions())
assert not (s.prefix / 'bin/agentdeck').is_symlink()
assert s.receipt.read_bytes() == before_receipt and owner.read_bytes() == before_owner
assert s.identity(owner) == (True, 'make')


# 5. brew's HOME is not the account directory. The step may write below HOME
#    only, so the helper's write to the account must be denied, the install must
#    fail and roll back, and no receipt may appear anywhere.
#
#    A failed install proves none of that by itself: a download, cache or
#    staging error also fails, also leaves nothing behind, and may also say
#    "permission denied". So the case has two halves. The control runs a probe
#    in the cask's own step, with the cask's own sandbox declarations, and must
#    show the declared directory writable and the account directory not. Only
#    then does the real install run, and it counts only if the helper itself
#    reports being denied at this account's directory.
#
#    If the sandbox lets the account write through, the design's assumption
#    does not hold on this runner: say so and fail. It is never recorded as a
#    pass, and never worked around.
CONTROL_MARKER = '.sandbox-control'


def control_cask(rendered, declared, account):
    """The rendered cask with only the registration step's script replaced."""
    lines = rendered.splitlines(keepends=True)
    start = next(i for i, line in enumerate(lines) if '"agentdeck-cask-register"' in line and "<<~'SH'" in line)
    end = next(i for i in range(start + 1, len(lines)) if lines[i].strip() == 'SH')
    indent = ' ' * 10
    probe = [
        'set -u',
        f'declared={shlex.quote(str(declared))}',
        f'account={shlex.quote(str(account))}',
        'for name in declared account; do',
        '  eval "directory=\\$$name"',
        f'  if ( /bin/mkdir -p "$directory" && : >"$directory/{CONTROL_MARKER}" ) 2>/dev/null; then',
        '    echo "SANDBOX_CONTROL_$name=allowed"',
        '  else',
        '    echo "SANDBOX_CONTROL_$name=denied"',
        '  fi',
        'done',
        'exit 0',
    ]
    lines[start + 1:end] = [indent + line + '\n' for line in probe]
    return ''.join(lines)


def judge_sandbox_control(returncode, output, declared_written, account_written):
    """Whether the step sandbox denies exactly the write the design relies on."""
    reported = dict(line.strip().split('=', 1) for line in output.splitlines()
                    if line.strip().startswith('SANDBOX_CONTROL_') and '=' in line)
    assert returncode == 0 and set(reported) == {'SANDBOX_CONTROL_declared', 'SANDBOX_CONTROL_account'}, \
        'the sandbox control step did not run to completion, so it proves nothing\n' + output
    assert reported['SANDBOX_CONTROL_declared'] == 'allowed' and declared_written, \
        'the control could not write the declared directory, so its denial of the account is not evidence\n' + output
    if reported['SANDBOX_CONTROL_account'] != 'denied' or account_written:
        raise AssertionError('DESIGN: with brew HOME and the account in different directories the step sandbox '
                             'let a step write the account directory. The refusal the design relies on does not '
                             'hold on this runner.\n' + output)


def judge_home_mismatch(returncode, output, support, receipts, app_exists, caskroom_exists):
    """Whether a failed install is the helper being denied at this account."""
    if receipts:
        raise AssertionError('DESIGN: with brew HOME and the account in different directories a receipt was still '
                             f'written: {receipts}\n' + output)
    assert returncode != 0 and not app_exists and not caskroom_exists, \
        'the install did not fail and roll back\n' + output
    # The helper names the directory it could not write. Nothing before the
    # registration step knows this account's path, so an earlier failure, such
    # as a download or cache error, cannot produce this line.
    denied = [line for line in output.splitlines()
              if str(support) in line and ('not permitted' in line or 'permission denied' in line.lower())]
    assert denied, ('the install failed, but not because the helper was denied at the account directory; '
                    'a failure before the registration step is not this case\n' + output)


s = Scenario('home-mismatch', account='account')
assert s.account != s.home and s.home not in s.account.parents and s.account not in s.home.parents
declared = s.home / 'Library/Application Support/AgentDeck'
production = s.cask.read_text()
s.cask.write_text(control_cask(production, declared, s.support))
attempt = subprocess.run(list(map(str, s.argv('install'))), env=s.env, text=True, stdout=subprocess.PIPE,
                         stderr=subprocess.STDOUT, timeout=300)
judge_sandbox_control(attempt.returncode, attempt.stdout,
                      (declared / CONTROL_MARKER).exists(), (s.support / CONTROL_MARKER).exists())
s.command('uninstall')
assert not s.app.exists() and not s.caskroom.exists()
(declared / CONTROL_MARKER).unlink()
s.cask.write_text(production)
attempt = subprocess.run(list(map(str, s.argv('install'))), env=s.env, text=True, stdout=subprocess.PIPE,
                         stderr=subprocess.STDOUT, timeout=300)
judge_home_mismatch(attempt.returncode, attempt.stdout, s.support,
                    sorted(map(str, s.directory.rglob('installation.json'))), s.app.exists(), s.caskroom.exists())

print('Cask real Homebrew regression: PASS', flush=True)
PY
