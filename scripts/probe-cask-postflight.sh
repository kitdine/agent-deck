#!/usr/bin/env bash
set -euo pipefail

# Never run real Homebrew or account-directory operations on a developer Mac.
if [[ ${1:-} != --ci || ${GITHUB_ACTIONS:-} != true || ${RUNNER_ENVIRONMENT:-} != github-hosted || ${RUNNER_OS:-} != macOS ]]; then
  echo 'REFUSED: requires --ci, GITHUB_ACTIONS=true, RUNNER_ENVIRONMENT=github-hosted, and RUNNER_OS=macOS.' >&2
  exit 2
fi
if [[ ${2:-} != --inspect && ${2:-} != --run ]]; then
  echo 'usage: probe-cask-postflight.sh --ci <--inspect|--run>' >&2
  exit 2
fi

export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1
python3 - <<'PY'
import json
import os
from pathlib import Path
import shutil
import subprocess

output = Path(os.environ['RUNNER_TEMP']) / 'cask-postflight-probe'
output.mkdir()


def run(argv):
    result = subprocess.run(argv, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    print('$ ' + ' '.join(argv), flush=True)
    print(result.stdout, end='', flush=True)
    result.check_returncode()
    return result.stdout.strip()


metadata = {
    'run_id': os.environ['GITHUB_RUN_ID'],
    'job': os.environ['GITHUB_JOB'],
    'sha': os.environ['GITHUB_SHA'],
    'macos': run(['sw_vers']),
    'uname': run(['uname', '-a']),
    'brew_version': run(['brew', '--version']),
    'brew_repository': run(['brew', '--repository']),
}
repository = Path(metadata['brew_repository'])
metadata['brew_commit'] = run(['git', '-C', str(repository), 'rev-parse', 'HEAD'])
metadata['brew_home'] = os.environ['HOME']
(output / 'runtime.json').write_text(json.dumps(metadata, indent=2) + '\n')

files = []
for relative in ('install_steps.rb', 'cask/artifact/install_steps.rb', 'cask/artifact/abstract_artifact.rb',
                 'cask/dsl.rb', 'sandbox.rb', 'cask_artifact.rb'):
    path = repository / 'Library/Homebrew' / relative
    text = path.read_text()
    files.append(path)
    destination = output / 'brew-source' / path.relative_to(repository)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, destination)
    print(f'\nSOURCE {path.relative_to(repository)}', flush=True)
    for number, line in enumerate(text.splitlines(), 1):
        print(f'{number}: {line}')
print('INSPECTION_COMPLETE: ' + json.dumps(metadata, sort_keys=True), flush=True)
print('SOURCE_FILES: ' + json.dumps([str(path.relative_to(repository)) for path in files]), flush=True)
PY

if [[ $2 == --run ]]; then
  root=$(cd "$(dirname "$0")/.." && pwd)
  python3 "$root/scripts/probe-cask-postflight.py" --ci
fi
