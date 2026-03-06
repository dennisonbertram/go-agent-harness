from pathlib import Path


def test_retry_behavior_is_correct() -> None:
    contents = Path("/app/retry.go").read_text()

    starts_at_base = (
        "time.Duration(i+1)*base" in contents
        or "time.Duration(i + 1)*base" in contents
        or ("current := base" in contents and "current += base" in contents)
        or ("for i := 1; i <= attempts; i++" in contents and "time.Duration(i)*base" in contents)
    )
    assert starts_at_base

    caps_at_thirty_seconds = (
        "30 * time.Second" in contents
        and (
            "if delay > 30*time.Second" in contents
            or "if delay > 30 * time.Second" in contents
            or "if current > 30*time.Second" in contents
            or "if current > 30 * time.Second" in contents
            or "min(" in contents
        )
    )
    assert caps_at_thirty_seconds
