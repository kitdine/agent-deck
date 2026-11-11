"""Build the disposable cask App before sealing its ad-hoc signature."""

import os
from pathlib import Path
import shutil
import tempfile


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
    run(['/usr/bin/codesign', '--force', '--deep', '--sign', '-', bundle])
