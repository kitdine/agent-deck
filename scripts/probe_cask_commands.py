"""Pure command construction for the disposable cask probe; never runs brew."""


def cask_command_argv(brew, operation, appdir, token):
    if operation not in ('install', 'reinstall', 'upgrade', 'uninstall'):
        raise ValueError(f'unsupported probe operation: {operation}')
    argv = [str(brew), operation, '--cask', '--verbose']
    if operation in ('install', 'reinstall', 'upgrade'):
        argv.append(f'--appdir={appdir}')
    argv.append(f'agentdeck/probe/{token}')
    return argv
