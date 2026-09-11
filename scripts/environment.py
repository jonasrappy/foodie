"""Read Foodie's literal .env format without executing or interpolating its values."""
import json
import os
import re
from pathlib import Path


def read_environment(path, inherit=True):
    values = {}
    file = Path(path)
    if file.exists():
        for number, line in enumerate(file.read_text().splitlines(), 1):
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            key, sep, value = line.partition('=')
            key, value = key.strip(), value.strip()
            if not sep or not re.fullmatch(r'[A-Z][A-Z0-9_]*', key) or key in values:
                raise ValueError('Invalid environment entry on line {}'.format(number))
            try:
                if value.startswith('"'):
                    value = json.loads(value)
                elif value.startswith("'"):
                    if len(value) < 2 or not value.endswith("'"):
                        raise ValueError()
                    value = value[1:-1]
                if not isinstance(value, str) or any(c in value for c in '\r\n\0'):
                    raise ValueError()
            except (ValueError, TypeError):
                raise ValueError('Invalid environment value on line {}'.format(number)) from None
            values[key] = value
    if inherit:
        values.update(os.environ)
    return values
