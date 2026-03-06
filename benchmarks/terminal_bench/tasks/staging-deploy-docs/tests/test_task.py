from __future__ import annotations

import json
from pathlib import Path


def test_targets_include_staging() -> None:
    targets = json.loads(Path("/app/deploy/targets.json").read_text())
    assert targets["staging"] == {
        "scheme": "https",
        "host": "staging.internal",
        "port": 8443,
        "healthcheck_path": "/readyz",
    }
    assert targets["dev"] == {
        "scheme": "http",
        "host": "localhost",
        "port": 3000,
    }
    assert targets["prod"] == {
        "scheme": "https",
        "host": "api.internal",
        "port": 443,
    }


def test_readme_mentions_staging() -> None:
    readme = Path("/app/README.md").read_text()
    assert "make deploy-staging" in readme
    assert "https://staging.internal:8443" in readme
    assert "curl -fsS https://staging.internal:8443/readyz" in readme
