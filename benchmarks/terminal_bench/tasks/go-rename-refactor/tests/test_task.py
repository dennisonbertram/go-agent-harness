from __future__ import annotations

import glob
import subprocess
from pathlib import Path


def test_no_user_repo_references() -> None:
    """No .go file should contain 'UserRepo' after the rename."""
    go_files = glob.glob("/app/*.go")
    for filepath in go_files:
        contents = Path(filepath).read_text()
        assert "UserRepo" not in contents, (
            f"{filepath} still contains 'UserRepo'"
        )


def test_account_store_type_exists() -> None:
    """store.go must define the AccountStore type."""
    contents = Path("/app/store.go").read_text()
    assert "type AccountStore struct" in contents, (
        "store.go must define 'type AccountStore struct'"
    )


def test_account_store_constructor_exists() -> None:
    """store.go must have a NewAccountStore constructor."""
    contents = Path("/app/store.go").read_text()
    assert "func NewAccountStore()" in contents, (
        "store.go must define NewAccountStore constructor"
    )


def test_build_succeeds() -> None:
    """go build must succeed after rename."""
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


def test_go_tests_pass() -> None:
    """go test must succeed after rename."""
    result = subprocess.run(
        ["go", "test", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, (
        f"tests failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


def test_middleware_updated() -> None:
    """middleware.go must reference AccountStore, not UserRepo."""
    contents = Path("/app/middleware.go").read_text()
    assert "AccountStore" in contents, (
        "middleware.go must reference AccountStore"
    )
    assert "UserRepo" not in contents, (
        "middleware.go still references UserRepo"
    )


def test_handler_uses_account_store() -> None:
    """handler.go must reference AccountStore."""
    contents = Path("/app/handler.go").read_text()
    assert "AccountStore" in contents, (
        "handler.go must reference AccountStore"
    )


def test_context_key_updated() -> None:
    """The context key in middleware.go should not contain 'userRepo'."""
    contents = Path("/app/middleware.go").read_text()
    assert "userRepo" not in contents or "accountStore" in contents, (
        "context key should be updated from userRepo"
    )
