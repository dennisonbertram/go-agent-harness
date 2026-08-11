Resume a paused cron job. First call cron_get, then pass the job ID and its updated_at value as expected_updated_at. A stale version is rejected instead of overwriting a concurrent change.
