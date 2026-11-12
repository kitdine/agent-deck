"""Local, no-brew check of every command and the actual Scenario.command adapter."""

import ast
from pathlib import Path
from types import SimpleNamespace

from probe_cask_commands import cask_command_argv


def main():
    source = Path(__file__).with_name('probe-cask-postflight.py').read_text()
    scenario = next(node for node in ast.parse(source).body if isinstance(node, ast.ClassDef) and node.name == 'Scenario')
    adapter = next(node for node in scenario.body if isinstance(node, ast.FunctionDef) and node.name == 'command')
    calls = []

    def record(argv, **kwargs):
        calls.append((argv, kwargs))
        return argv

    namespace = {'run': record, 'cask_command_argv': cask_command_argv}
    exec(compile(ast.Module(body=[adapter], type_ignores=[]), '<isolated Scenario.command>', 'exec'), namespace)
    instance = SimpleNamespace(brew=Path('/fixture/brew/bin/brew'), appdir=Path('/fixture/Custom Applications'),
                               token='agentdeck-probe', env={'HOME': '/fixture/home'})
    for operation in ('install', 'reinstall', 'upgrade', 'uninstall'):
        expected = ['/fixture/brew/bin/brew', operation, '--cask', '--verbose']
        if operation != 'uninstall':
            expected.append('--appdir=/fixture/Custom Applications')
        expected.append('agentdeck/probe/agentdeck-probe')
        assert cask_command_argv(instance.brew, operation, instance.appdir, instance.token) == expected
        assert namespace['command'](instance, operation, check=False) == expected
        assert calls[-1][1] == {'env': instance.env, 'check': False}
        assert sum(arg.startswith('--appdir=') for arg in expected) == (operation != 'uninstall')
        print(f'{operation}: {expected!r}')
    before = len(calls)
    try:
        namespace['command'](instance, 'unsupported')
    except ValueError:
        pass
    else:
        raise AssertionError('unknown subcommand must be refused')
    assert len(calls) == before
    print('PASS: all four exact argv lists and adapter forwarding; no subprocess or brew was executed')


if __name__ == '__main__':
    main()
