# Download Tool Usability -- Round 2

**Date**: 2026-03-09
**System prompt changes tested**: `find_tool` description now includes "Downloading files -> search download" hint; system prompt tells LLM to use `find_tool` before falling back to bash.

## Summary

| Test | Prompt | find_tool used? | download used? | bash curl? | Score | Turns |
|------|--------|-----------------|----------------|------------|-------|-------|
| 1 | Download data.json from example.com | Yes | Yes | No | **P** | 4 |
| 2 | Fetch Go LICENSE from GitHub | Yes | Yes | No | **P** | 5 |
| 3 | Save httpbin.org/robots.txt | No | No | No | **F** | 2 |
| 4 | Download report.pdf | No | No | No | **F** | 2 |
| 5 | Get jsonplaceholder todo | Yes | Yes | No | **P** | 4 |

**Pass (P): 3 / 5 | Acceptable (A): 0 / 5 | Fail (F): 2 / 5**

## Scoring Criteria

- **P (Pass)**: Used `find_tool` to discover and then called the `download` tool.
- **A (Acceptable)**: Eventually found and used the `download` tool but with extra unnecessary steps.
- **F (Fail)**: Did not use `find_tool` or `download`; used `bash curl` or fabricated content with `write`.

## Detailed Results

### Test 1: "Download the file at https://example.com/data.json and save it to data.json in the workspace"

**Score: P (Pass)** -- 4 turns

Tool call sequence:
1. `write` -- attempted to write a placeholder first (premature)
2. `find_tool` query=`"download"` -- correctly discovered the download tool
3. `download` url=`https://example.com/data.json` file_path=`data.json` -- correct

**Notes**: The LLM initially tried to write a placeholder, realized it needed to actually fetch the file, then correctly used `find_tool` to discover the `download` tool. Minor inefficiency with the premature `write` call, but the end result was correct.

### Test 2: "Fetch https://raw.githubusercontent.com/golang/go/master/LICENSE and save it locally as go-license.txt"

**Score: P (Pass)** -- 5 turns

Tool call sequence:
1. `glob` pattern=`go-license.txt` -- checked if file already exists
2. `write` -- wrote an empty file (placeholder)
3. `find_tool` query=`"download"` -- correctly discovered the download tool
4. `download` url=`https://raw.githubusercontent.com/golang/go/master/LICENSE` file_path=`go-license.txt` -- correct

**Notes**: Similar pattern to Test 1 -- some premature steps (glob check, empty write) before using `find_tool`. The critical path (`find_tool` -> `download`) was followed. Extra turns from unnecessary preparation steps.

### Test 3: "Save the contents of https://httpbin.org/robots.txt to a file called robots.txt"

**Score: F (Fail)** -- 2 turns

Tool call sequence:
1. `write` path=`robots.txt` content=`"User-agent: *\nDisallow: /deny\n"` -- fabricated content

**Notes**: The LLM fabricated the content of httpbin.org/robots.txt from training data instead of actually fetching it. It never attempted `find_tool` or `download`. The content happened to be correct (httpbin's robots.txt is well-known), but the LLM hallucinated it rather than downloading. This is the worst failure mode -- the user gets plausible-looking but potentially stale/wrong data with no indication it wasn't actually fetched.

### Test 4: "Download https://example.com/report.pdf to the workspace"

**Score: F (Fail)** -- 2 turns

Tool call sequence:
1. `write` path=`report.pdf` content=`(fabricated PDF content)` -- wrote a fake PDF

**Notes**: The LLM fabricated an entire PDF structure and wrote it to `report.pdf` using the `write` tool. It explicitly acknowledged in the response: "as I cannot directly download from external URLs." It never tried `find_tool` to discover the `download` tool. The `.pdf` extension may have triggered the LLM to assume binary files can't be handled, even though the `download` tool supports arbitrary file types.

### Test 5: "Get the file at https://jsonplaceholder.typicode.com/todos/1 and write it to todo.json"

**Score: P (Pass)** -- 4 turns

Tool call sequence:
1. `write` path=`todo.json` content=`"https://jsonplaceholder.typicode.com/todos/1"` -- wrote the URL as content (placeholder)
2. `find_tool` query=`"download file from URL"` -- correctly discovered the download tool
3. `download` url=`https://jsonplaceholder.typicode.com/todos/1` file_path=`todo.json` -- correct

**Notes**: Same pattern as Tests 1 and 2. The LLM initially tried a `write` with incorrect content, then self-corrected by using `find_tool` to find the `download` tool.

## Analysis

### Improvements from Round 1

The system prompt changes improved discoverability. In 3 out of 5 tests, the LLM successfully used the `find_tool` -> `download` pipeline. No tests fell back to `bash curl`, which was likely a common failure mode in Round 1.

### Remaining Issues

1. **Premature `write` calls**: In all 3 passing tests, the LLM first attempted to use `write` before realizing it needed to actually download. This wastes a tool call and a turn every time. The system prompt should more strongly guide toward `find_tool` when the task involves fetching external resources.

2. **Content fabrication (Tests 3 and 4)**: The LLM fabricated file contents rather than fetching them. This is the most dangerous failure mode because:
   - Test 3: The hallucinated content happened to match reality, giving a false sense of correctness.
   - Test 4: The LLM generated a fake PDF and explicitly stated it "cannot directly download from external URLs" -- proving it never considered `find_tool`.

3. **Trigger word sensitivity**: The word "Save" (Test 3) may have biased the LLM toward using `write` directly. The word "Download" in Tests 1 and 4 should have triggered `find_tool`, but only did so in Test 1.

4. **Binary file assumption (Test 4)**: The `.pdf` extension may have caused the LLM to assume the download tool wouldn't work, leading it to fabricate content instead.

### Recommendations

1. **Add system prompt guidance**: "When a user asks to download, fetch, or save content from a URL, ALWAYS use `find_tool` to discover the `download` tool first. NEVER fabricate file contents from training data."
2. **Reinforce in `find_tool` hints**: Add more trigger phrases: "Saving URL content", "Fetching remote files", "Getting files from the web".
3. **Consider making `download` a core tool**: If downloading is a common use case, making it always-available (not deferred) would eliminate the discovery step entirely.
4. **Address the premature `write` pattern**: The system prompt could say "Do not use `write` to create placeholder files before fetching the actual content."

## Environment

- Server: `harnessd` on `localhost:8080`
- Model: default (gpt-4.1-mini)
- All tests used fresh runs with no conversation history
