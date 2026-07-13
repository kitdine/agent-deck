#!/usr/bin/env python3
"""Shared helpers for the local AI provider switch commands."""

import json
import os
import re
import stat
import tempfile
from datetime import datetime
from decimal import Decimal, InvalidOperation
from pathlib import Path

ROOT = Path.home() / ".config" / "ai-provider-mode"
CONFIG = ROOT / "providers.json"
ALIAS_RE = re.compile(r"^[a-z0-9][a-z0-9_-]*$")


def fail(message):
    raise SystemExit(f"ERROR: {message}")


def key_name(provider_name, client, alias):
    if not ALIAS_RE.fullmatch(alias):
        fail("invalid alias; use lowercase letters, digits, - or _")
    return f"{provider_name}-{client}-{alias}"


def load_config():
    if not CONFIG.is_file():
        fail(f"provider config not found: {CONFIG}")
    if stat.S_IMODE(CONFIG.stat().st_mode) != 0o600:
        fail(f"provider config must have mode 0600: {CONFIG}")
    try:
        data = json.loads(CONFIG.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"cannot read provider config: {exc}")
    if not isinstance(data.get("providers"), dict) or not isinstance(data.get("keys"), dict):
        fail("provider config needs object fields: providers and keys")
    return data


def save_config(data):
    ROOT.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".providers.", dir=ROOT)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(data, handle, indent=2, ensure_ascii=False)
            handle.write("\n")
        os.chmod(temporary, 0o600)
        os.replace(temporary, CONFIG)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def service(data, provider_name, client):
    provider = data["providers"].get(provider_name)
    if not isinstance(provider, dict):
        fail(f"unknown provider: {provider_name}")
    host = provider.get("host")
    if not isinstance(host, str) or not host.startswith(("http://", "https://")):
        fail(f"provider host is missing: {provider_name}")
    selected = provider.get(client)
    if not isinstance(selected, dict) or selected.get("enabled") is False:
        fail(f"{client} is unavailable for provider: {provider_name}")
    return host.rstrip("/"), selected


def cost_multiplier(data, provider_name):
    provider = data["providers"].get(provider_name)
    if not isinstance(provider, dict):
        fail(f"unknown provider: {provider_name}")
    value = provider.get("cost_multiplier", 1)
    if isinstance(value, bool):
        fail(f"provider cost_multiplier is invalid: {provider_name}")
    try:
        multiplier = Decimal(str(value))
    except (InvalidOperation, ValueError):
        fail(f"provider cost_multiplier is invalid: {provider_name}")
    if not multiplier.is_finite() or multiplier < 0:
        fail(f"provider cost_multiplier is invalid: {provider_name}")
    return format(multiplier, "f")


def selected_key(data, provider_name, client, alias=None):
    _, selected = service(data, provider_name, client)
    if selected.get("auth") == "openai_login":
        if alias:
            fail(f"{provider_name}/{client} uses login auth and accepts no key")
        return None, None
    allowed = selected.get("keys")
    if not isinstance(allowed, list) or not all(isinstance(item, str) for item in allowed):
        fail(f"no keys configured for {provider_name}/{client}")
    if alias is None:
        alias = selected.get("default_key")
        if alias is None and len(allowed) == 1:
            alias = allowed[0]
        if alias is None:
            fail(f"choose a key with --key; available: {', '.join(allowed)}")
    if alias is not None and alias not in allowed:
        qualified = key_name(provider_name, client, alias)
        if qualified in allowed:
            alias = qualified
    if alias not in allowed:
        fail(f"key '{alias}' is not allowed for {provider_name}/{client}")
    value = data["keys"].get(alias, {}).get("value")
    if not isinstance(value, str) or not value.strip() or value == "replace-me":
        fail(f"key '{alias}' has no value")
    return alias, value


def backup(path, client, text):
    directory = ROOT / "backups" / client
    directory.mkdir(parents=True, exist_ok=True)
    name = f"{path.stem}.{datetime.now():%Y%m%d-%H%M%S-%f}{path.suffix}"
    target = directory / name
    # Replace the complete token assignment, including triple-quoted values.
    token_value = (
        r'(?:"{3}.*?"{3}|' + r"'{3}.*?'{3}|" +
        r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|[^\n#]*)'
    )
    redacted = re.sub(
        r'''(?ms)^\s*(?:experimental_bearer_token|["']experimental_bearer_token["'])\s*=\s*'''
        + token_value + r'''(?:\s*#.*)?$''',
        'experimental_bearer_token = "<redacted>"', text,
    )
    redacted = re.sub(
        r'''(?m)^(\s*"?ANTHROPIC_AUTH_TOKEN"?\s*:\s*)"(?:\\.|[^"\\])*"''',
        r'\1"<redacted>"', redacted,
    )
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as handle:
        handle.write(redacted)
    return target
