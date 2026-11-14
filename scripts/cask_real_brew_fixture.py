"""Build the disposable cask App before sealing its ad-hoc signature."""

import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile


def run_fixture_command(argv, *, env, success=True, timeout=180):
    command = list(map(str, argv))
    try:
        result = subprocess.run(command, env=env, text=True, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=timeout)
    except subprocess.TimeoutExpired as failure:
        captured = []
        for stream in (failure.output, failure.stderr):
            if stream:
                captured.append(stream.decode('utf-8', errors='replace') if isinstance(stream, bytes) else stream)
        output = '\n'.join(captured) or '(no captured subprocess output)'
        raise AssertionError(f'Command timed out after {failure.timeout} seconds: {command}\n'
                             f'Captured subprocess output:\n{output}') from failure
    if (result.returncode == 0) != success:
        raise AssertionError(f'Unexpected exit {result.returncode}: {command}\n{result.stdout}')
    return result.stdout


def prepare_fixture_app(fixture_app, bundle, helper, version, run, *, env=None):
    bundle.parent.mkdir()
    run(['/usr/bin/ditto', fixture_app, bundle])
    shutil.copy2(helper, bundle / 'Contents/Helpers/agentdeck')
    (bundle / 'Contents/Resources/version').write_text(version + '\n')
    # Debug builds have not passed through package-macos-app.sh. Populate the
    # same cask resources using the replacement helper before signing.
    completions = bundle / 'Contents/Resources/completions'
    completions.mkdir(parents=True, exist_ok=True)
    # The replacement is acceptance-tagged and requires an explicit account
    # even for completion commands. Do not use the runner's HOME or a later
    # installation scenario's account while preparing these shared archives.
    with tempfile.TemporaryDirectory(prefix='completion-', dir=bundle.parent) as temporary:
        home, account = (Path(temporary) / name for name in ('home', 'account'))
        home.mkdir(mode=0o700)
        account.mkdir(mode=0o700)
        completion_env = {name: value for name, value in (os.environ if env is None else env).items()
                          if not name.startswith(('AGENTDECK_ACCEPTANCE_', 'HOMEBREW_AGENTDECK_'))}
        completion_env.update(HOME=str(home), AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=str(account))
        for shell in ('bash', 'zsh', 'fish'):
            output = run([bundle / 'Contents/Helpers/agentdeck', 'completion', shell], env=completion_env)
            if not output.strip():
                raise ValueError(f'fixture {shell} completion is empty')
            destination = completions / f'agentdeck.{shell}'
            destination.write_text(output)
            destination.chmod(0o644)
    sign_fixture_app(bundle, run)


def sign_fixture_app(bundle, run):
    widget = bundle / 'Contents/PlugIns/AgentDeckWidget.appex'
    app_group = plistlib.loads((bundle / 'Contents/Info.plist').read_bytes()).get('AgentDeckAppGroupIdentifier')
    widget_group = plistlib.loads((widget / 'Contents/Info.plist').read_bytes()).get('AgentDeckAppGroupIdentifier')
    if not isinstance(app_group, str) or not app_group or '$(' in app_group or widget_group != app_group:
        raise ValueError('fixture App and widget must have one matching resolved App Group')
    entitlements_root = Path(__file__).resolve().parents[1] / 'apps/macos'
    with tempfile.TemporaryDirectory(prefix='signing-', dir=bundle.parent) as temporary:
        resolved = {}
        for name in ('AgentDeck', 'AgentDeckWidget'):
            entitlements = plistlib.loads((entitlements_root / f'{name}.entitlements').read_bytes())
            if entitlements.get('com.apple.security.application-groups') != ['$(AGENTDECK_APP_GROUP)']:
                raise ValueError(f'unexpected fixture App Group entitlement template: {name}')
            entitlements['com.apple.security.application-groups'] = [app_group]
            if name == 'AgentDeckWidget' and entitlements.get('com.apple.security.app-sandbox') is not True:
                raise ValueError('fixture widget must retain its App Sandbox entitlement')
            destination = Path(temporary) / f'{name}.entitlements'
            destination.write_bytes(plistlib.dumps(entitlements))
            resolved[name] = destination

        # Unsigned Debug builds have no entitlements to preserve. Match the
        # packaging contract: sign nested code explicitly, then seal the host.
        # --deep signing would re-sign children with the wrong/empty permissions.
        run(['/usr/bin/codesign', '--force', '--sign', '-', bundle / 'Contents/Helpers/agentdeck'])
        for framework in sorted((bundle / 'Contents/Frameworks').glob('*.framework')):
            run(['/usr/bin/codesign', '--force', '--sign', '-', framework])
        for target, name in ((widget, 'AgentDeckWidget'), (bundle, 'AgentDeck')):
            run(['/usr/bin/codesign', '--force', '--sign', '-', '--generate-entitlement-der',
                 '--entitlements', resolved[name], target])
    run(['/usr/bin/codesign', '--verify', '--deep', '--strict', '--verbose=2', bundle])
