import subprocess
from pathlib import Path


def test_go_tests_pass() -> None:
    """Run the Go unit tests that verify Schedule behavior."""
    result = subprocess.run(
        ["go", "test", "-v", "-count=1", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, (
        f"go test failed:\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )


def test_retry_first_delay_is_not_zero() -> None:
    """Verify the code no longer starts at zero delay."""
    contents = Path("/app/retry.go").read_text()
    # The original bug was `time.Duration(i)*base` which produces 0 for i=0.
    # Make sure the agent didn't leave the code unchanged.
    assert "time.Duration(i)*base" not in contents or "i := 1" in contents, (
        "retry.go still appears to contain the original buggy pattern "
        "(first delay would be zero)"
    )


def test_retry_has_thirty_second_cap() -> None:
    """Verify the code mentions a 30-second cap somewhere."""
    contents = Path("/app/retry.go").read_text()
    assert "30" in contents, (
        "retry.go does not reference 30 (the cap should be 30 * time.Second)"
    )
