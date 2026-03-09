Read the output of a background bash job. Use this after launching a command with bash's run_in_background=true to check its progress or retrieve results.

Parameters:
- shell_id (required): The job identifier returned by the bash tool when run_in_background was true.
- wait (optional): If true, blocks up to 5 seconds for the job to finish before returning. Useful when you expect the job to complete soon.