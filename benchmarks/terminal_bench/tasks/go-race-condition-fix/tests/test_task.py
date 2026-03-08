from __future__ import annotations

import re
import subprocess
from pathlib import Path


def test_race_detector_passes() -> None:
    """go test -race must exit 0 (no data races)."""
    result = subprocess.run(
        ["go", "test", "-race", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, (
        f"race detector failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


def test_synchronization_primitive_present() -> None:
    """counter.go must use sync.Mutex, sync.RWMutex, or sync/atomic."""
    contents = Path("/app/counter.go").read_text()
    has_sync = (
        "sync.Mutex" in contents
        or "sync.RWMutex" in contents
        or "atomic." in contents
    )
    assert has_sync, "counter.go must use sync.Mutex, sync.RWMutex, or sync/atomic"


def test_build_succeeds() -> None:
    """go build must succeed."""
    result = subprocess.run(
        ["go", "build", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, (
        f"build failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


def test_endpoints_intact() -> None:
    """main.go must still define /inc, /get, and /reset handlers."""
    contents = Path("/app/main.go").read_text()
    for endpoint in ["/inc", "/get", "/reset"]:
        assert endpoint in contents, f"endpoint {endpoint} missing from main.go"
