"""One-off real-brew experiment; never an installer or a product test runner."""

import os
import sys

# Keep the Python entry point just as restricted as the shell entry point.
allowed_arguments = (['--ci'], ['--ci', '--focus', 'main-uninstall'], ['--ci', '--focus', 'receipt-lifecycle'],
                     ['--ci', '--focus', 'followup-original'], ['--ci', '--focus', 'followup-lifecycle'])
if not (sys.argv[1:] in allowed_arguments and os.environ.get('GITHUB_ACTIONS') == 'true'
        and os.environ.get('RUNNER_ENVIRONMENT') == 'github-hosted'
        and os.environ.get('RUNNER_OS') == 'macOS'):
    sys.exit('REFUSED: this probe runs only with --ci on GitHub-hosted macOS.')
focus = sys.argv[3] if len(sys.argv) == 4 else None

import hashlib
import json
from pathlib import Path
import pwd
import plistlib
import re
import shutil
import subprocess
import tempfile

from probe_cask_commands import cask_command_argv

root = Path(__file__).resolve().parents[1]
output = Path(os.environ['RUNNER_TEMP']) / 'cask-postflight-probe'
runtime = json.loads((output / 'runtime.json').read_text())
# Round 1 read these exact installed sources before the cask DSL was written.
assert runtime['brew_commit'] == 'c6069d2a943b71b0a90d91fc1ac93cb9c1634dcc', 'Homebrew source changed since inspection'
repository = Path(runtime['brew_repository'])
real_home = Path(pwd.getpwuid(os.getuid()).pw_dir).resolve()
support = real_home / 'Library/Application Support/AgentDeck'
receipt = support / 'installation.json'
workspace = Path(tempfile.mkdtemp(prefix='cask-postflight-', dir=os.environ['RUNNER_TEMP'])).resolve()
observations = {f'Q{i}': {'result': 'NOT_EXECUTED'} for i in range(1, 9)}
observations.update({f'R{i}': {'result': 'NOT_EXECUTED'} for i in range(1, 6)})
brew_results = []
command_number = 0
base_env = {key: value for key, value in os.environ.items()
            if not key.startswith(('AGENTDECK_ACCEPTANCE_', 'HOMEBREW_AGENTDECK_'))
            and key not in ('HOMEBREW_DEVELOPER', 'HOMEBREW_AVOID_NESTED_SANDBOXING', 'HOMEBREW_CASK_OPTS', 'GOFLAGS')}
base_env.update(HOMEBREW_NO_ANALYTICS='1', HOMEBREW_NO_AUTO_UPDATE='1', HOMEBREW_NO_ENV_HINTS='1',
                HOMEBREW_NO_INSTALL_CLEANUP='1')


def save_observations():
    (output / 'observations.json').write_text(json.dumps(observations, indent=2) + '\n')


def observe(question, result, **facts):
    observations[question] = dict(result=result, **facts)
    print(question + ': ' + json.dumps(observations[question], sort_keys=True), flush=True)
    save_observations()


def run(argv, *, env=None, check=True, timeout=240):
    global command_number
    command_number += 1
    argv = list(map(str, argv))
    prefix = f'{command_number:03d}'
    print(f'COMMAND {prefix}: ' + json.dumps(argv), flush=True)
    timed_out = False
    try:
        result = subprocess.run(argv, env=env or base_env, cwd=root, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=timeout)
    except subprocess.TimeoutExpired as failure:
        timed_out = True
        result = subprocess.CompletedProcess(argv, 124, failure.stdout or b'', failure.stderr or b'')
    (output / f'{prefix}.stdout').write_bytes(result.stdout)
    (output / f'{prefix}.stderr').write_bytes(result.stderr)
    record = dict(argv=argv, exit=result.returncode, timed_out=timed_out,
                  HOME=(env or base_env).get('HOME'))
    (output / f'{prefix}.command.json').write_text(json.dumps(record, indent=2) + '\n')
    stdout, stderr = (value.decode('utf-8', errors='replace') for value in (result.stdout, result.stderr))
    print(f'COMMAND {prefix} STDOUT:\n{stdout}', flush=True)
    print(f'COMMAND {prefix} STDERR:\n{stderr}', flush=True)
    print(f'COMMAND {prefix} EXIT={result.returncode}', flush=True)
    result.stdout, result.stderr = stdout, stderr
    result.probe_command = prefix
    if Path(argv[0]).name == 'brew':
        brew_results.append(result)
        assert not re.search(r'without (?:Homebrew.s )?sandbox|sandbox unavailable', stdout + stderr, re.I), 'Sandbox was bypassed'
    if check and result.returncode:
        raise RuntimeError(f'command {prefix} failed with {result.returncode}; full stdout/stderr retained')
    return result


def location(helper, env):
    result = run([helper, '--format', 'json', 'desktop', 'state-location'], env=env)
    return json.loads(result.stdout)['data']


def register(helper, channel, env):
    return run([helper, 'install', 'register', '--channel', channel, '--executable', helper], env=env)


def register_step(*, absolute_support=None, declare=True):
    if not declare:
        paths = ''
    elif absolute_support is None:
        paths = '        writable_paths: ["Library/Application Support/AgentDeck"],\n        writable_base: :home,\n'
    else:
        paths = f'        writable_paths: [{json.dumps(str(absolute_support))}],\n'
    return ('    run "{{appdir}}/AgentDeck.app/Contents/Helpers/agentdeck",\n'
            '        args: ["install", "register", "--channel", "homebrew_cask",\n'
            '               "--executable", "{{appdir}}/AgentDeck.app/Contents/Helpers/agentdeck"],\n'
            + paths + '        print_stdout: true')


class Scenario:
    def __init__(self, name, *, home=real_home, account=None, appdir=None, receipt_only=False, tolerant=False):
        self.name = name
        self.directory = workspace / name
        self.prefix = self.directory / 'brew'
        self.home = home
        self.appdir = appdir or self.directory / 'Applications'
        self.receipt_only = receipt_only
        self.tolerant = tolerant
        self.token = ('agentdeck-tolerant-probe' if tolerant else
                      'agentdeck-receipt-only-control' if receipt_only else 'agentdeck-postflight-probe')
        self.cask = self.prefix / f'Library/Taps/agentdeck/homebrew-probe/Casks/{self.token}.rb'
        self.app = self.appdir / 'AgentDeck.app'
        self.helper = self.app / 'Contents/Helpers/agentdeck'
        for path in (self.prefix / 'bin', self.appdir, self.cask.parent, home,
                     self.directory / 'cache', self.directory / 'logs', self.directory / 'temp'):
            path.mkdir(parents=True, exist_ok=True)
        shutil.copy2(repository / 'bin/brew', self.prefix / 'bin/brew')
        (self.prefix / 'Library/Homebrew').symlink_to(repository / 'Library/Homebrew')
        self.brew = self.prefix / 'bin/brew'
        self.env = dict(base_env, HOME=str(home), HOMEBREW_CACHE=str(self.directory / 'cache'),
                        HOMEBREW_LOGS=str(self.directory / 'logs'), HOMEBREW_TEMP=str(self.directory / 'temp'))
        self.env.pop('HOMEBREW_NO_INSTALL_FROM_API', None)
        if account is not None:
            self.env['HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME'] = str(account)
        result = run([self.brew, '--prefix'], env=self.env)
        assert result.stdout.strip() == str(self.prefix)

    def render(self, version, *, absolute_support=None, skip_register=False, acceptance=False):
        archive = archives[('acceptance' if acceptance else 'production', version)]
        template = ('probe-cask-tolerant.rb.tmpl' if self.tolerant else
                    'probe-cask-receipt-only.rb.tmpl' if self.receipt_only else 'probe-cask-postflight.rb.tmpl')
        text = (root / 'scripts' / template).read_text()
        values = dict(TOKEN=self.token, VERSION=version, SHA256=hashlib.sha256(archive.read_bytes()).hexdigest(),
                      URL=archive.as_uri(), REGISTER_STEP='' if skip_register else register_step(absolute_support=absolute_support),
                      UNINSTALL_SCRIPT='\n'.join('                ' + line for line in tolerant_uninstall_script().splitlines()) if self.tolerant else '',
                      WIDGET_ID=plistlib.loads((fixture / 'Contents/PlugIns/AgentDeckWidget.appex/Contents/Info.plist').read_bytes())['CFBundleIdentifier'] if self.tolerant else '')
        for key, value in values.items():
            text = text.replace('@' + key + '@', value)
        assert not re.search(r'\b(?:preflight|postflight|uninstall_preflight)\s+do\b', text)
        self.cask.write_text(text)
        (output / f'{self.name}-{version}.rb').write_text(text)
        print(f'RENDERED_CASK {self.name} {version}:\n{text}', flush=True)

    def command(self, operation, *, check=True):
        return run(cask_command_argv(self.brew, operation, self.appdir, self.token), env=self.env, check=check)

    def caskroom_entries(self):
        path = self.prefix / 'Caskroom' / self.token
        return [str(item.relative_to(path)) for item in sorted(path.rglob('*'))] if path.exists() else []


def home_base_control(archive):
    fake_home = workspace / 'home-base-control-home'
    s = Scenario('home-base-control', home=fake_home)
    marker = fake_home / 'Library/Application Support/AgentDeck/base.marker'
    marker.parent.mkdir(parents=True)
    # The marker's parent is NOT the prefix, appdir, cache or HOMEBREW_TEMP.
    # A successful write therefore observes the actual :home grant resolution.
    script = 'printf "PROBE_STEP_HOME=%s\\n" "$HOME"; printf home-base-marker > "$1"'
    text = f'''cask "{s.token}" do
  version "1.2.3"
  sha256 "{hashlib.sha256(archive.read_bytes()).hexdigest()}"
  url "{archive.as_uri()}"
  name "AgentDeck home base control"
  desc "Observe the parent home base without running a production helper"
  homepage "https://github.com/kitdine/agent-deck"
  postflight_steps do
    run "/bin/sh", args: ["-c", {json.dumps(script)}, "probe", {json.dumps(str(marker))}],
        writable_paths: ["Library/Application Support/AgentDeck"],
        writable_base: :home,
        print_stdout: true
  end
end
'''
    s.cask.write_text(text)
    (output / 'home-base-control.rb').write_text(text)
    result = s.command('install')
    assert marker.read_text() == 'home-base-marker'
    assert not (support / 'base.marker').exists()
    facts = dict(brew_HOME=str(fake_home), actual_marker=str(marker), marker_bytes=marker.read_text(),
                 real_account_home=str(real_home), command=result.probe_command,
                 step_HOME=re.findall(r'PROBE_STEP_HOME=([^\r\n]+)', result.stdout + result.stderr))
    print('HOME_BASE_CONTROL: ' + json.dumps(facts, sort_keys=True), flush=True)
    (output / 'home-base-control.json').write_text(json.dumps(facts, indent=2) + '\n')
    return facts


def zero_declaration_control(s):
    control = Scenario('zero-declaration-control', appdir=s.appdir)
    # Deliberate ablation AFTER Q2 succeeds: the exact same self-register call
    # without writable_paths. No App artifact, replacement, retry or fallback.
    text = f'''cask "{control.token}" do
  version "1.2.3"
  sha256 "{hashlib.sha256(archives['production', '1.2.3'].read_bytes()).hexdigest()}"
  url "{archives['production', '1.2.3'].as_uri()}"
  name "AgentDeck zero declaration control"
  desc "Measure whether the single writable directory grant is necessary"
  homepage "https://github.com/kitdine/agent-deck"
  postflight_steps do
{register_step(declare=False)}
  end
end
'''
    control.cask.write_text(text)
    (output / 'zero-declaration-control.rb').write_text(text)
    before = receipt.read_bytes()
    result = control.command('install', check=False)
    facts = dict(exit=result.returncode, command=result.probe_command, receipt_unchanged=receipt.read_bytes() == before,
                 stdout=result.stdout, stderr=result.stderr)
    print('ZERO_DECLARATION_CONTROL: ' + json.dumps(facts, sort_keys=True), flush=True)
    observations['Q2']['zero_declaration_control'] = facts
    save_observations()
    if result.returncode:
        assert facts['receipt_unchanged']
    return facts


def acceptance_probe():
    account = Path(tempfile.mkdtemp(prefix='agentdeck-probe-account-', dir='/private/tmp')).resolve()
    assert not account.is_relative_to(real_home)
    acceptance_helper = workspace / 'acceptance-helper'
    run(['go', 'build', '-mod=vendor', '-trimpath', '-tags', 'agentdeck_acceptance',
         '-o', acceptance_helper, './cmd/agentdeck'])
    info = run(['go', 'version', '-m', acceptance_helper]).stdout
    assert 'agentdeck_acceptance' in info
    bundle = workspace / 'acceptance-1.2.3/AgentDeck.app'
    prepare_fixture_app(fixture, bundle, acceptance_helper, '1.2.3', fixture_run, env=base_env)
    archive = workspace / 'AgentDeck-acceptance-1.2.3.zip'
    run(['/usr/bin/ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', bundle, archive])
    archives['acceptance', '1.2.3'] = archive
    acceptance = Scenario('acceptance-outside-home', account=account, receipt_only=True)
    acceptance_support = account / 'Library/Application Support/AgentDeck'
    acceptance_support.parent.mkdir(parents=True)
    acceptance.render('1.2.3', absolute_support=acceptance_support, acceptance=True)
    installed = acceptance.command('install', check=False)
    acceptance_receipt = acceptance_support / 'installation.json'
    owner = json.loads(acceptance_receipt.read_text()) if acceptance_receipt.exists() else None
    passed = (installed.returncode == 0 and owner is not None and owner['executable'] == str(acceptance.helper)
              and (acceptance_support / 'ownership.lock').is_file())
    observe('Q8', 'PASS' if passed else 'FAIL', command=installed.probe_command, exit=installed.returncode,
            account=str(account), outside_real_home=True, receipt=owner,
            receipt_path=str(acceptance_receipt), declared_paths=[str(acceptance_support)],
            override='HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME', override_value=str(account),
            stdout=installed.stdout, stderr=installed.stderr)


def focused_probe():
    receipt_only = focus == 'receipt-lifecycle'
    s = Scenario(focus, receipt_only=receipt_only)
    s.render('1.2.3')
    installed = s.command('install', check=False)
    if installed.returncode:
        observe('Q2', 'FAIL', command=installed.probe_command, exit=installed.returncode,
                stdout=installed.stdout, stderr=installed.stderr)
        raise RuntimeError('STOP: production receipt gate failed; no alternative attempted')
    initial = json.loads(receipt.read_text())
    assert initial['executable'] == str(s.helper) and (support / 'ownership.lock').is_file()
    observe('Q2', 'PASS', command=installed.probe_command, receipt_path=str(receipt),
            lock_path=str(support / 'ownership.lock'), receipt=initial,
            declared_paths=['Library/Application Support/AgentDeck'], writable_base='home')

    if not receipt_only:
        # This is the unchanged failing template, installed once and immediately
        # uninstalled. No upgrade, launch, registration or recovery in between.
        extension = s.app / 'Contents/PlugIns/AgentDeckWidget.appex'
        listing = run(['/usr/bin/pluginkit', '-m', '-A', '-D', '-vv'], check=False)
        extension_existed = extension.is_dir()
        before = receipt.read_bytes()
        uninstalled = s.command('uninstall', check=False)
        statuses = re.findall(r'PROBE_PLUGINKIT_R_EXIT=([0-9]+)', uninstalled.stdout + uninstalled.stderr)
        passed = uninstalled.returncode == 0 and statuses == ['0'] and not s.app.exists()
        observe('Q7', 'PASS' if passed else 'FAIL', command=uninstalled.probe_command,
                brew_exit=uninstalled.returncode, pluginkit_r_exit=statuses, never_launched=True,
                extension_existed_before_uninstall=extension_existed,
                pre_uninstall_query_exit=listing.returncode,
                target_in_query=str(extension) in listing.stdout + listing.stderr,
                app_exists_after=s.app.exists(), receipt_byte_equal=receipt.read_bytes() == before,
                stdout=uninstalled.stdout, stderr=uninstalled.stderr)
        return 0 if passed else 1

    print('CONTROL_SCOPE: Q4/Q5 receipt-only cask; the main-style uninstall failure is NOT repaired or waived.', flush=True)
    s.render('1.2.4')
    upgraded = s.command('upgrade', check=False)
    after = json.loads(receipt.read_text())
    identity = location(s.helper, s.env)
    version = (s.app / 'Contents/Resources/version').read_text().strip()
    repeated = f'registered {s.helper} (homebrew_cask)' in upgraded.stdout + upgraded.stderr
    passed = (upgraded.returncode == 0 and version == '1.2.4' and repeated and identity['installed']
              and initial['executable'] == after['executable'] and initial['channel'] == after['channel'] == 'homebrew_cask')
    observe('Q4', 'PASS' if passed else 'FAIL', scope='receipt-only control; main-style callback absent',
            command=upgraded.probe_command, exit=upgraded.returncode, installed_version=version,
            registered_again=repeated, before_receipt=initial, after_receipt=after, location=identity,
            stdout=upgraded.stdout, stderr=upgraded.stderr)
    if not passed:
        raise RuntimeError('Receipt-only upgrade failed; no retries or repairs')

    before = receipt.read_bytes()
    s.render('1.2.5', skip_register=True)
    without_register = s.command('upgrade', check=False)
    identity = location(s.helper, s.env)
    version = (s.app / 'Contents/Resources/version').read_text().strip()
    facts = dict(command=without_register.probe_command, exit=without_register.returncode,
                 installed_version=version, receipt_byte_equal=receipt.read_bytes() == before,
                 registered_output_present=f'registered {s.helper} (homebrew_cask)' in without_register.stdout + without_register.stderr,
                 location=identity, stdout=without_register.stdout, stderr=without_register.stderr)
    observations['Q4']['upgrade_without_register'] = facts
    save_observations()
    print('UPGRADE_WITHOUT_REGISTER: ' + json.dumps(facts, sort_keys=True), flush=True)
    if not (facts['exit'] == 0 and version == '1.2.5' and facts['receipt_byte_equal']
            and not facts['registered_output_present'] and identity['installed']):
        observations['Q4']['result'] = 'FAIL'
        raise RuntimeError('Upgrade without registration failed; no repair')

    before = receipt.read_bytes()
    uninstalled = s.command('uninstall', check=False)
    assert uninstalled.returncode == 0 and not s.app.exists()
    retained = receipt.read_bytes()
    stale = json.loads(retained)
    assert retained == before and not Path(stale['executable']).exists()
    (output / 'stale-after-uninstall.json').write_bytes(retained)
    successor = workspace / 'other-channel/bin/agentdeck'
    successor.parent.mkdir(parents=True)
    shutil.copy2(helper, successor)
    registered = register(successor, 'make', s.env)
    successor_identity = location(successor, s.env)
    observe('Q5', 'PASS' if successor_identity['installed'] else 'FAIL',
            scope='receipt-only control; main-style callback absent',
            uninstall_command=uninstalled.probe_command, uninstall_exit=uninstalled.returncode,
            command=registered.probe_command, app_absent=True, receipt_retained=True,
            receipt_byte_equal_after_uninstall=True, stale_executable=stale['executable'],
            stale_executable_exists=False, successor=str(successor), channel='make', takeover=False,
            location=successor_identity, stdout=registered.stdout)
    # Q1/Q2/Q3/Q6 are observed in round 2. Q7's separate job has already finished
    # (including a preserved FAIL) before this job starts. This is secondary.
    acceptance_probe()
    return 0 if observations['Q8']['result'] == 'PASS' else 1


def tolerant_uninstall_script():
    return (root / 'scripts/probe-cask-tolerant-uninstall.sh.tmpl').read_text()


def installed_record(s, label):
    caskroom = s.prefix / 'Caskroom' / s.token
    metadata = caskroom / '.metadata'
    config_path = metadata / 'config.json'
    config = json.loads(config_path.read_text()) if config_path.exists() else None
    caskfiles = [str(path.relative_to(caskroom)) for path in sorted(metadata.rglob('*'))
                 if path.is_file() and path.parent.name == 'Casks'
                 and path.name in (s.token + '.rb', s.token + '.json')]
    if config is not None:
        (output / f'{s.name}-{label}-brew-config.json').write_bytes(config_path.read_bytes())
    facts = dict(caskroom_exists=caskroom.exists(), installed_caskfiles=caskfiles,
                 config_path=str(config_path), config=config,
                 explicit_appdir=config.get('explicit', {}).get('appdir') if config is not None else None)
    print('INSTALLED_RECORD ' + label + ': ' + json.dumps(facts, sort_keys=True), flush=True)
    return facts


def lifecycle_state(s, label, observer):
    present = s.helper.is_file()
    queried = s.helper if present else observer
    result = run([queried, '--format', 'json', 'desktop', 'state-location'], env=s.env, check=False)
    identity = json.loads(result.stdout)['data'] if result.returncode == 0 else None
    raw_receipt = receipt.read_bytes() if receipt.exists() else None
    if raw_receipt is not None:
        (output / f'{s.name}-{label}-installation.json').write_bytes(raw_receipt)
    version_file = s.app / 'Contents/Resources/version'
    facts = dict(app_exists=s.app.exists(), installed_helper_exists=present,
                 queried_helper=str(queried), retained_copy_used=not present,
                 helper_query_command=result.probe_command, helper_query_exit=result.returncode,
                 identity=identity, receipt=json.loads(raw_receipt) if raw_receipt is not None else None,
                 receipt_sha256=hashlib.sha256(raw_receipt).hexdigest() if raw_receipt is not None else None,
                 version=version_file.read_text().strip() if version_file.exists() else None,
                 homebrew_record=installed_record(s, label))
    print('LIFECYCLE_STATE ' + label + ': ' + json.dumps(facts, sort_keys=True), flush=True)
    return facts


def query_streams(result):
    text = result.stdout.replace('\r\n', '\n')
    captured = {}
    for stream in ('STDOUT', 'STDERR'):
        match = re.search(f'PROBE_QUERY_{stream}_BEGIN\n(.*?)\nPROBE_QUERY_{stream}_END', text, re.S)
        captured[stream.lower()] = match.group(1) if match else None
    captured['exit_codes'] = re.findall(r'PROBE_QUERY_EXIT=([0-9]+)', text)
    return captured


def followup_acceptance_probe():
    account = Path(tempfile.mkdtemp(prefix='agentdeck-followup-account-', dir='/private/tmp')).resolve()
    assert not account.is_relative_to(real_home)
    (account / 'Library/Application Support').mkdir(parents=True)
    acceptance_helper = workspace / 'followup-acceptance-helper'
    run(['go', 'build', '-mod=vendor', '-trimpath', '-tags', 'agentdeck_acceptance',
         '-o', acceptance_helper, './cmd/agentdeck'])
    info = run(['go', 'version', '-m', acceptance_helper]).stdout
    assert 'agentdeck_acceptance' in info
    bundle = workspace / 'followup-acceptance-1.2.3/AgentDeck.app'
    prepare_fixture_app(fixture, bundle, acceptance_helper, '1.2.3', fixture_run, env=base_env)
    archive = workspace / 'AgentDeck-followup-acceptance-1.2.3.zip'
    run(['/usr/bin/ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', bundle, archive])
    archives['acceptance', '1.2.3'] = archive

    def attempt(name, brew_home):
        s = Scenario(name, home=brew_home, account=account, tolerant=True)
        # Keep the production relative :home grant unchanged. R5's second
        # attempt changes only the brew HOME premise, not the path whitelist.
        s.render('1.2.3', acceptance=True)
        result = s.command('install', check=False)
        path = account / 'Library/Application Support/AgentDeck/installation.json'
        owner = json.loads(path.read_text()) if path.exists() else None
        lock = path.with_name('ownership.lock')
        passed = (result.returncode == 0 and owner is not None and owner['executable'] == str(s.helper)
                  and owner['channel'] == 'homebrew_cask' and lock.is_file())
        return dict(passed=passed, command=result.probe_command, brew_exit=result.returncode,
                    brew_HOME=str(brew_home), account_override=str(account), outside_real_home=True,
                    declared_paths=['Library/Application Support/AgentDeck'], writable_base='home',
                    receipt_path=str(path), receipt=owner, ownership_lock_exists=lock.is_file(),
                    stdout=result.stdout, stderr=result.stderr)

    first = attempt('followup-acceptance-real-home', real_home)
    second = attempt('followup-acceptance-matching-home', account) if not first['passed'] else None
    passed = first['passed'] or (second is not None and second['passed'])
    observe('R5', 'PASS' if passed else 'FAIL', initial=first, matching_HOME_attempt=second,
            affects_product_feasibility=False)


def followup_probe():
    original = focus == 'followup-original'
    s = Scenario(focus, tolerant=not original)
    s.render('1.2.3')
    observer = workspace / 'retained-observer/agentdeck'
    observer.parent.mkdir()
    shutil.copy2(helper, observer)
    installed = s.command('install', check=False)
    install_state = lifecycle_state(s, 'install', observer)
    if installed.returncode:
        observe('Q2', 'FAIL', command=installed.probe_command, exit=installed.returncode)
        observe('R1' if original else 'R3', 'FAIL', failed_stage='install',
                command=installed.probe_command, brew_exit=installed.returncode, state=install_state,
                stdout=installed.stdout, stderr=installed.stderr)
        return 1  # Mandatory stop, including no R5, on a production install failure.
    observe('Q2', 'PASS', command=installed.probe_command, receipt_path=str(receipt),
            receipt=install_state['receipt'])
    assert install_state['homebrew_record']['explicit_appdir'] == str(s.appdir)

    if original:
        before = receipt.read_bytes()
        removed = s.command('uninstall', check=False)
        state = lifecycle_state(s, 'uninstall', observer)
        statuses = re.findall(r'PROBE_PLUGINKIT_R_EXIT=([0-9]+)', removed.stdout + removed.stderr)
        passed = removed.returncode == 0 and not state['app_exists'] and not state['homebrew_record']['installed_caskfiles']
        observe('R1', 'PASS' if passed else 'FAIL', install_command=installed.probe_command,
                uninstall_command=removed.probe_command, brew_exit=removed.returncode,
                pluginkit_r_exit=sorted(set(statuses)), never_launched=True,
                uninstall_argv=cask_command_argv(s.brew, 'uninstall', s.appdir, s.token),
                before_record=install_state['homebrew_record'], after=state,
                receipt_byte_equal=receipt.exists() and receipt.read_bytes() == before,
                stdout=removed.stdout, stderr=removed.stderr)
        return 0 if passed else 1

    # R2 uses the EXACT script embedded in the R3 cask. The failure control is
    # explicitly synthetic and cannot register, deregister or remove anything.
    extension = s.app / 'Contents/PlugIns/AgentDeckWidget.appex'
    identifier = plistlib.loads((extension / 'Contents/Info.plist').read_bytes())['CFBundleIdentifier']
    script = tolerant_uninstall_script()
    queried = run(['/bin/sh', '-c', script, 'probe-r2', extension, identifier], env=s.env, check=False)
    streams = query_streams(queried)
    query_call = '/usr/bin/pluginkit -m -A -D -vv -i "$identifier"'
    assert script.count(query_call) == 1
    failure_script = script.replace(query_call, '/usr/bin/false', 1)
    failure = run(['/bin/sh', '-c', failure_script, 'probe-r2-query-failure', extension, identifier], env=s.env, check=False)
    failure_streams = query_streams(failure)
    r2_passed = (queried.returncode == 0 and streams['exit_codes'] == ['0']
                 and 'PROBE_UNREGISTER=skipped_unregistered' in queried.stdout
                 and failure.returncode != 0 and failure_streams['exit_codes'] == ['1']
                 and 'PROBE_UNREGISTER=skipped_unregistered' not in failure.stdout)
    observe('R2', 'PASS' if r2_passed else 'FAIL', command=queried.probe_command,
            script_sha256=hashlib.sha256(script.encode()).hexdigest(), script_exit=queried.returncode,
            query_argv=['/usr/bin/pluginkit', '-m', '-A', '-D', '-vv', '-i', identifier],
            query=streams, target=str(extension), stdout=queried.stdout, stderr=queried.stderr,
            synthetic_query_failure=dict(command=failure.probe_command, script_exit=failure.returncode,
                                         query=failure_streams, stdout=failure.stdout, stderr=failure.stderr),
            registered_removal_branch_verified=False)

    s.render('1.2.4')
    upgraded = s.command('upgrade', check=False)
    upgrade_state = lifecycle_state(s, 'upgrade', observer)
    stages = dict(install=dict(command=installed.probe_command, brew_exit=installed.returncode, state=install_state),
                  upgrade=dict(command=upgraded.probe_command, brew_exit=upgraded.returncode, state=upgrade_state,
                               stdout=upgraded.stdout, stderr=upgraded.stderr))
    if upgraded.returncode:
        observe('R3', 'FAIL', failed_stage='upgrade', stages=stages)
        return 1  # Stop immediately: do not uninstall, take over or run R5.
    assert upgrade_state['homebrew_record']['explicit_appdir'] == str(s.appdir)
    before = receipt.read_bytes()
    removed = s.command('uninstall', check=False)
    uninstall_state = lifecycle_state(s, 'uninstall', observer)
    stages['uninstall'] = dict(command=removed.probe_command, brew_exit=removed.returncode, state=uninstall_state,
                               stdout=removed.stdout, stderr=removed.stderr,
                               argv=cask_command_argv(s.brew, 'uninstall', s.appdir, s.token))
    r3_passed = (removed.returncode == 0 and (install_state['identity'] or {}).get('installed')
                 and (upgrade_state['identity'] or {}).get('installed') and upgrade_state['version'] == '1.2.4'
                 and not uninstall_state['app_exists'] and not uninstall_state['homebrew_record']['installed_caskfiles']
                 and uninstall_state['identity'] is not None and not uninstall_state['identity']['installed']
                 and receipt.exists() and receipt.read_bytes() == before)
    observe('R3', 'PASS' if r3_passed else 'FAIL', stages=stages,
            uninstall_appdir_source='saved .metadata/config.json explicit.appdir; no argv or HOMEBREW_CASK_OPTS override')

    stale = json.loads(receipt.read_text()) if receipt.exists() else None
    if removed.returncode == 0 and not s.app.exists() and stale is not None and not Path(stale['executable']).exists():
        successor = workspace / 'other-channel/bin/agentdeck'
        successor.parent.mkdir(parents=True)
        shutil.copy2(observer, successor)
        claimed = run([successor, 'install', 'register', '--channel', 'make', '--executable', successor],
                      env=s.env, check=False)
        current = json.loads(receipt.read_text()) if receipt.exists() else None
        identity = location(successor, s.env) if claimed.returncode == 0 else None
        passed = (claimed.returncode == 0 and current is not None and current['channel'] == 'make'
                  and current['executable'] == str(successor) and current['installation_id'] != stale['installation_id']
                  and identity['installed'])
        observe('R4', 'PASS' if passed else 'FAIL', command=claimed.probe_command, brew_uninstall_command=removed.probe_command,
                register_exit=claimed.returncode, takeover=False, stale_receipt=stale,
                stale_executable_exists=False, receipt_after=current, successor_identity=identity,
                stdout=claimed.stdout, stderr=claimed.stderr)
    else:
        observe('R4', 'NOT_EXECUTED', reason='R3 did not leave an absent App and a retained stale receipt')
    followup_acceptance_probe()
    return 0 if r2_passed and r3_passed and observations['R4']['result'] == 'PASS' else 1


archives = {}
save_observations()
try:
    print('ACCOUNT: ' + json.dumps(dict(real_home=str(real_home), brew_HOME=base_env['HOME'],
                                        support_exists=support.exists(), support_parent_exists=support.parent.exists())), flush=True)
    assert not support.exists(), 'Refuse to overwrite pre-existing runner AgentDeck receipt or lock'
    assert Path(base_env['HOME']).resolve() == real_home, 'Production :home probe needs the normal account HOME'
    run(['bash', root / 'scripts/build-macos-app.sh'], timeout=1200)
    fixture = root / 'apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app'
    helper = fixture / 'Contents/Helpers/agentdeck'
    build_info = run(['go', 'version', '-m', helper]).stdout
    assert 'agentdeck_acceptance' not in build_info
    from cask_real_brew_fixture import prepare_fixture_app

    def fixture_run(argv, *, env=None):
        return run(argv, env=env).stdout

    versions = (('1.2.3',) if focus == 'followup-original' else
                ('1.2.3', '1.2.4') if focus == 'followup-lifecycle' else ('1.2.3', '1.2.4', '1.2.5'))
    for version in versions:
        bundle = workspace / ('production-' + version) / 'AgentDeck.app'
        prepare_fixture_app(fixture, bundle, helper, version, fixture_run, env=base_env)
        archive = workspace / f'AgentDeck-production-{version}.zip'
        run(['/usr/bin/ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', bundle, archive])
        archives['production', version] = archive
    run(['/usr/bin/codesign', '-d', '--verbose=4', workspace / 'production-1.2.3/AgentDeck.app/Contents/Helpers/agentdeck'])
    run(['xcrun', 'stapler', 'validate', workspace / 'production-1.2.3/AgentDeck.app'], check=False)
    if focus in ('followup-original', 'followup-lifecycle'):
        raise SystemExit(followup_probe())
    if focus:
        raise SystemExit(focused_probe())
    home_control = home_base_control(archives['production', '1.2.3'])

    s = Scenario('production-lifecycle')
    s.render('1.2.3')
    result = s.command('install', check=False)
    text = result.stdout + result.stderr
    helper_executed = 'Release Version:' in text and 'Go Version:' in text
    if helper_executed:
        observe('Q1', 'PASS', command=result.probe_command, helper_build_info=build_info,
                helper_version_lines=[line for line in text.splitlines() if 'Version:' in line or 'Git Commit Hash:' in line])
    if result.returncode:
        # No alternative path grant, classic block or first-launch workaround.
        observe('Q2', 'FAIL' if helper_executed else 'BLOCKED', exit=result.returncode,
                command=result.probe_command, receipt_exists=receipt.exists(), stdout=result.stdout, stderr=result.stderr,
                home_base_control=home_control)
        raise RuntimeError('STOP: production receipt registration failed; no alternative attempted')
    assert helper_executed, 'Unrecognised version output; do not infer Q1'
    owner = json.loads(receipt.read_text())
    assert owner['executable'] == str(s.helper) and owner['channel'] == 'homebrew_cask'
    assert (support / 'ownership.lock').is_file()
    assert not (s.prefix / 'var/homebrew/sandbox/Library/Application Support/AgentDeck/installation.json').exists()
    observe('Q2', 'PASS', command=result.probe_command, brew_HOME=str(s.home), real_account_home=str(real_home),
            step_HOME=re.findall(r'PROBE_STEP_HOME=([^\r\n]+)', text), receipt=owner,
            receipt_path=str(receipt), lock_path=str(support / 'ownership.lock'),
            declared_paths=['Library/Application Support/AgentDeck'], writable_base='home',
            home_base_control=home_control)
    (output / 'production-initial-installation.json').write_bytes(receipt.read_bytes())
    zero_declaration_control(s)

    # Q6 uses the already valid receipt for s.helper, which remains present.
    # The refusal target is a DIFFERENT App path in a separate prefix/appdir.
    assert location(s.helper, s.env)['installed'] is True
    before = receipt.read_bytes()
    (output / 'rollback-preset-installation.json').write_bytes(before)
    rollback = Scenario('foreign-owner-rollback')
    rollback.render('1.2.3')
    rejected = rollback.command('install', check=False)
    facts = dict(command=rejected.probe_command, exit=rejected.returncode,
                 owned_by_other='owned_by_other' in rejected.stdout + rejected.stderr,
                 app_exists=rollback.app.exists(), caskroom_entries=rollback.caskroom_entries(),
                 receipt_byte_equal=receipt.read_bytes() == before,
                 before_sha256=hashlib.sha256(before).hexdigest(),
                 after_sha256=hashlib.sha256(receipt.read_bytes()).hexdigest(),
                 preset_executable=str(s.helper), preset_executable_exists=s.helper.is_file())
    passed = (rejected.returncode != 0 and facts['owned_by_other'] and not facts['app_exists']
              and not facts['caskroom_entries'] and facts['receipt_byte_equal'] and facts['preset_executable_exists'])
    observe('Q6', 'PASS' if passed else 'FAIL', **facts)
    assert facts['receipt_byte_equal'], 'Do not continue after unexpected receipt mutation'

    # Normal upgrade re-runs the step against a live same-path/channel owner.
    previous = json.loads(receipt.read_text())
    s.render('1.2.4')
    upgraded = s.command('upgrade', check=False)
    after = json.loads(receipt.read_text())
    identity = location(s.helper, s.env)
    version = (s.app / 'Contents/Resources/version').read_text().strip()
    repeated = f'registered {s.helper} (homebrew_cask)' in upgraded.stdout + upgraded.stderr
    facts = dict(command=upgraded.probe_command, exit=upgraded.returncode, installed_version=version,
                 registered_again=repeated, before_executable=previous['executable'], after_executable=after['executable'],
                 same_channel=previous['channel'] == after['channel'] == 'homebrew_cask',
                 location=identity, postflight_lines=[line for line in (upgraded.stdout + upgraded.stderr).splitlines()
                                                     if 'PROBE_POSTFLIGHT_VERSION=' in line or 'registered ' in line])
    passed = (upgraded.returncode == 0 and repeated and version == '1.2.4' and identity['installed']
              and previous['executable'] == after['executable'] and facts['same_channel'])
    observe('Q4', 'PASS' if passed else 'FAIL', **facts)
    if upgraded.returncode:
        raise RuntimeError('Upgrade failed; preserve it without retries or uninstall fixes')

    # Directly test the separate "same-path upgrade does not need re-registration"
    # premise, instead of inferring it from a successful re-registration.
    before = receipt.read_bytes()
    s.render('1.2.5', skip_register=True)
    no_register_upgrade = s.command('upgrade', check=False)
    identity = location(s.helper, s.env)
    facts = dict(command=no_register_upgrade.probe_command, exit=no_register_upgrade.returncode,
                 receipt_byte_equal=receipt.read_bytes() == before, location=identity,
                 installed_version=(s.app / 'Contents/Resources/version').read_text().strip(),
                 registered_output_present=f'registered {s.helper} (homebrew_cask)' in no_register_upgrade.stdout + no_register_upgrade.stderr)
    observations['Q4']['upgrade_without_register'] = facts
    save_observations()
    print('UPGRADE_WITHOUT_REGISTER: ' + json.dumps(facts, sort_keys=True), flush=True)
    if not (facts['exit'] == 0 and facts['receipt_byte_equal'] and identity['installed']
            and facts['installed_version'] == '1.2.5' and not facts['registered_output_present']):
        observations['Q4']['result'] = 'FAIL'
        raise RuntimeError('Same-path upgrade without registration failed; no repair attempted')

    # No App launch, pluginkit -a or lsregister call occurs anywhere in the probe.
    listing = run(['/usr/bin/pluginkit', '-m', '-A', '-D', '-vv'], check=False)
    target_extension = s.app / 'Contents/PlugIns/AgentDeckWidget.appex'
    extension_existed = target_extension.is_dir()
    before = receipt.read_bytes()
    uninstalled = s.command('uninstall', check=False)
    removal_status = re.findall(r'PROBE_PLUGINKIT_R_EXIT=([0-9]+)', uninstalled.stdout + uninstalled.stderr)
    observe('Q7', 'PASS' if uninstalled.returncode == 0 and removal_status == ['0'] else 'FAIL',
            command=uninstalled.probe_command, brew_exit=uninstalled.returncode, pluginkit_r_exit=removal_status,
            never_launched=True, pre_uninstall_query_exit=listing.returncode,
            target_in_query=str(target_extension) in listing.stdout + listing.stderr,
            extension_existed_before_uninstall=extension_existed, app_exists_after=s.app.exists(),
            stdout=uninstalled.stdout, stderr=uninstalled.stderr)
    if uninstalled.returncode or s.app.exists():
        raise RuntimeError('Uninstall failed; record Q7 without changing its script')
    retained = receipt.read_bytes()
    stale = json.loads(retained)
    assert before == retained and not Path(stale['executable']).exists()
    successor = workspace / 'other-channel/bin/agentdeck'
    successor.parent.mkdir(parents=True)
    shutil.copy2(helper, successor)
    registered = register(successor, 'make', s.env)
    successor_identity = location(successor, s.env)
    observe('Q5', 'PASS' if successor_identity['installed'] else 'FAIL', command=registered.probe_command,
            app_absent=True, receipt_retained=True, receipt_byte_equal_after_uninstall=True,
            stale_executable=stale['executable'], stale_executable_exists=False,
            successor=str(successor), channel='make', takeover=False, location=successor_identity,
            stdout=registered.stdout)
    (output / 'stale-after-uninstall.json').write_bytes(retained)

    # Q8 is secondary and cannot run before all seven primary observations exist.
    if all(observations[f'Q{i}']['result'] not in ('NOT_EXECUTED', 'BLOCKED') for i in (1, 2, 4, 5, 6, 7)):
        account = Path(tempfile.mkdtemp(prefix='agentdeck-probe-account-', dir='/private/tmp')).resolve()
        assert not account.is_relative_to(real_home)
        acceptance_helper = workspace / 'acceptance-helper'
        run(['go', 'build', '-mod=vendor', '-trimpath', '-tags', 'agentdeck_acceptance',
             '-o', acceptance_helper, './cmd/agentdeck'])
        info = run(['go', 'version', '-m', acceptance_helper]).stdout
        assert 'agentdeck_acceptance' in info
        bundle = workspace / 'acceptance-1.2.3/AgentDeck.app'
        prepare_fixture_app(fixture, bundle, acceptance_helper, '1.2.3', fixture_run, env=base_env)
        archive = workspace / 'AgentDeck-acceptance-1.2.3.zip'
        run(['/usr/bin/ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', bundle, archive])
        archives['acceptance', '1.2.3'] = archive
        acceptance = Scenario('acceptance-outside-home', account=account)
        acceptance_support = account / 'Library/Application Support/AgentDeck'
        # Only ancestors are prepared outside the step; the helper must create
        # its own AgentDeck directory, lock and receipt inside the sandbox.
        acceptance_support.parent.mkdir(parents=True)
        acceptance.render('1.2.3', absolute_support=acceptance_support, acceptance=True)
        installed = acceptance.command('install', check=False)
        acceptance_receipt = acceptance_support / 'installation.json'
        owner = json.loads(acceptance_receipt.read_text()) if acceptance_receipt.exists() else None
        passed = (installed.returncode == 0 and owner is not None and owner['executable'] == str(acceptance.helper)
                  and (acceptance_support / 'ownership.lock').is_file())
        observe('Q8', 'PASS' if passed else 'FAIL', command=installed.probe_command, exit=installed.returncode,
                account=str(account), outside_real_home=True, receipt=owner,
                receipt_path=str(acceptance_receipt), declared_paths=[str(acceptance_support)],
                stdout=installed.stdout, stderr=installed.stderr)
finally:
    warnings = []
    for result in brew_results:
        for line in (result.stdout + result.stderr).splitlines():
            if re.search(r'deprecat', line, re.I):
                warnings.append(dict(command=result.probe_command, line=line))
    observe('Q3', 'PASS' if not warnings else 'FAIL', brew_command_count=len(brew_results), deprecation_lines=warnings)
    save_observations()
    print('PROBE_OBSERVATIONS: ' + json.dumps(observations, sort_keys=True), flush=True)
    # Keep all scenario state and raw logs until artifact upload; no cleanup
    # callback is allowed to hide an install, upgrade or uninstall failure.
