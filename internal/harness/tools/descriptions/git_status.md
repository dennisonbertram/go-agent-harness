Show the git status of the workspace repository. Reports staged, unstaged, modified, untracked, and deleted files. Use this tool to check whether the working tree is clean or dirty, which files have been added to the index, and which are new or pending commit.

Returns a compact summary equivalent to `git status --short`, including the current branch name, ahead/behind counts relative to upstream, and a per-file status code (M = modified, A = added, D = deleted, ?? = untracked).

This tool does NOT show line-level changes or diffs — to see what actually changed inside a file, use git_diff instead.
