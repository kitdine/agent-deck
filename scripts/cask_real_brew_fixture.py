"""Build the disposable cask App before sealing its ad-hoc signature."""

import shutil


def prepare_fixture_app(fixture_app, bundle, helper, version, run):
    bundle.parent.mkdir()
    run(['/usr/bin/ditto', fixture_app, bundle])
    shutil.copy2(helper, bundle / 'Contents/Helpers/agentdeck')
    (bundle / 'Contents/Resources/version').write_text(version + '\n')
    # Debug builds have not passed through package-macos-app.sh. Populate the
    # same cask resources using the replacement helper before signing.
    completions = bundle / 'Contents/Resources/completions'
    completions.mkdir(parents=True, exist_ok=True)
    for shell in ('bash', 'zsh', 'fish'):
        output = run([bundle / 'Contents/Helpers/agentdeck', 'completion', shell])
        if not output.strip():
            raise ValueError(f'fixture {shell} completion is empty')
        destination = completions / f'agentdeck.{shell}'
        destination.write_text(output)
        destination.chmod(0o644)
    run(['/usr/bin/codesign', '--force', '--deep', '--sign', '-', bundle])
