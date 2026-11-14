"""Run the real probe decision body with a PlugInKit stub, never brew or PlugInKit."""

import argparse
import json
from pathlib import Path
import shlex
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--temp-root', type=Path, required=True)
    args = parser.parse_args()
    body = Path(__file__).with_name('probe-cask-tolerant-uninstall.sh.tmpl').read_text()
    assert body.count('/usr/bin/pluginkit') == 2
    target = '/fixture/Target Extension.appex'
    cases = (
        ('no-matches', 0, '  (no matches)\n', '', 0, False, 0),
        ('empty', 0, '', '', 0, False, 0),
        ('zero-plugins', 0, '(0 plug-ins)', '', 0, False, 0),
        ('exact-target', 0, f'  Path = {target}\n', '', 7, True, 7),
        ('other-path-only', 0, f'  Path = {target}.other\n', '', 0, False, 0),
        ('query-failure', 9, f'  Path = {target}\n', 'query failed\n', 0, False, 9),
    )
    with tempfile.TemporaryDirectory(prefix='cask-uninstall-stub-', dir=args.temp_root) as temporary:
        temporary = Path(temporary)
        stub = temporary / 'pluginkit-stub'
        stub.write_text('''#!/bin/sh
case "$1" in
  -m)
    printf 'query\n' >>"$FIXTURE_CALLS"
    /bin/cat "$FIXTURE_STDOUT"
    /bin/cat "$FIXTURE_STDERR" >&2
    exit "$FIXTURE_QUERY_EXIT"
    ;;
  -r)
    printf 'remove:%s\n' "$2" >>"$FIXTURE_CALLS"
    exit "$FIXTURE_REMOVE_EXIT"
    ;;
  *) exit 99 ;;
esac
''')
        stub.chmod(0o700)
        isolated_body = body.replace('/usr/bin/pluginkit', shlex.quote(str(stub)))
        assert '/usr/bin/pluginkit' not in isolated_body
        for name, query_exit, stdout, stderr, remove_exit, removed, expected_exit in cases:
            directory = temporary / name
            directory.mkdir()
            (directory / 'stdout').write_text(stdout)
            (directory / 'stderr').write_text(stderr)
            env = dict(PATH='/usr/bin:/bin', HOME=str(directory), TMPDIR=str(directory) + '/',
                       FIXTURE_CALLS=str(directory / 'calls'), FIXTURE_STDOUT=str(directory / 'stdout'),
                       FIXTURE_STDERR=str(directory / 'stderr'), FIXTURE_QUERY_EXIT=str(query_exit),
                       FIXTURE_REMOVE_EXIT=str(remove_exit))
            result = subprocess.run(['/bin/sh', '-c', isolated_body, 'probe-local-stub', target, 'fixture.widget'],
                                    env=env, capture_output=True, text=True)
            calls = (directory / 'calls').read_text().splitlines()
            expected_calls = ['query', f'remove:{target}'] if removed else ['query']
            assert calls == expected_calls, (name, calls, result)
            assert result.returncode == expected_exit, (name, result)
            skipped = 'PROBE_UNREGISTER=skipped_unregistered' in result.stdout
            assert skipped == (query_exit == 0 and not removed), (name, result)
            print(json.dumps(dict(case=name, result='PASS', query_exit=query_exit, query_stdout=stdout,
                                  script_exit=result.returncode, unregister_stub_called=removed,
                                  skipped_unregistered=skipped), sort_keys=True))
    print('PASS: six fixed inputs, exact-path matching and unchanged removal exit status; no brew or real PlugInKit ran')


if __name__ == '__main__':
    main()
