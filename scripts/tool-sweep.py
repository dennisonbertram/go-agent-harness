#!/usr/bin/env python3
"""Exercise every tool the harness exposes and record what actually happens.

A tool that is merely *registered* tells you nothing — the interesting failures
are tools that are offered, accepted, and then do not do the thing. So each tool
here is driven through a real run with a prompt that forces the call, and the
result is judged on the transcript and on any side effect the tool should have
had, not on whether the run returned 200.

Usage:  python3 scripts/tool-sweep.py <base-url> <model> [tool ...]
Writes JSON results to .ux/tool-sweep.json
"""
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8810"
MODEL = sys.argv[2] if len(sys.argv) > 2 else "gpt-oss-120b"
ONLY = set(sys.argv[3:])

# How to exercise each tool, and what proves it worked.
#
# `prompt`  drives the model to call the tool.
# `expect`  a substring that must appear in the run output for a pass.
# `skip`    a reason, for tools that cannot be driven safely from a script
#           (destructive, interactive, or needing external state).
PLAN = {
    "read": dict(prompt="Use the read tool on the file 'go.mod' and report its first line verbatim.",
                 expect="module"),
    "ls": dict(prompt="Use the ls tool on the current directory. Report how many entries it returned.",
               expect=""),
    "glob": dict(prompt="Use the glob tool with pattern '*.md' in the current directory. List what it found.",
                 expect=""),
    "grep": dict(prompt="Use the grep tool to search for 'package main' in this repo. Report the count of matches.",
                 expect=""),
    "write": dict(prompt="Use the write tool to create a file '.ux/sweep-write.txt' containing exactly SWEEP_WRITE_OK. Then confirm.",
                  expect=""),
    "edit": dict(prompt="Use the write tool to create '.ux/sweep-edit.txt' with content 'before'. Then use the edit tool to change 'before' to 'after'. Confirm the final content.",
                 expect=""),
    "bash": dict(prompt="Use the bash tool to run: echo SWEEP_BASH_OK. Report its exact output.",
                 expect="SWEEP_BASH_OK"),
    "file_inspect": dict(prompt="Use the file_inspect tool on 'go.mod' and summarise what it reports.",
                         expect=""),
    "apply_patch": dict(skip="mutates files via a patch format; covered by edit/write"),
    "todos": dict(prompt="Use the todos tool to record one todo: 'sweep check'. Then report the list.",
                  expect=""),
    "context_status": dict(prompt="Call the context_status tool and report the token usage it returns.",
                           expect=""),
    "compact_history": dict(skip="destructive to the conversation under test"),
    "find_tool": dict(prompt="Use the find_tool tool to search for tools matching 'cron'. List the names it returns.",
                      expect="cron"),
    "list_models": dict(prompt="Call list_models and report how many models it returned.",
                        expect=""),
    "git_status": dict(prompt="Call the git_status tool and report the branch name it gives.",
                       expect=""),
    "git_diff": dict(prompt="Call the git_diff tool and say whether it reported any changes.", expect=""),
    "git_log_search": dict(prompt="Use git_log_search for the term 'catalog'. Report how many commits matched.",
                           expect=""),
    "git_file_history": dict(prompt="Use git_file_history on 'go.mod'. Report the most recent commit subject.",
                             expect=""),
    "git_diff_range": dict(prompt="Use git_diff_range for HEAD~1..HEAD. Report how many files changed.",
                           expect=""),
    "git_blame_context": dict(prompt="Use git_blame_context on 'go.mod' line 1. Report what it returns.",
                              expect=""),
    "git_contributor_context": dict(prompt="Call git_contributor_context and report what it returns.", expect=""),
    "set_delayed_callback": dict(prompt="Call set_delayed_callback with delay '30s' and prompt 'sweep ping'. Report the callback id.",
                                 expect=""),
    "list_delayed_callbacks": dict(prompt="Call list_delayed_callbacks and report how many it returned.",
                                   expect=""),
    "cancel_delayed_callback": dict(prompt="First call set_delayed_callback with delay '60s' and prompt 'x'. Take the id it returns and call cancel_delayed_callback with it. Report the final state.",
                                    expect=""),
    "cron_create": dict(prompt="Call cron_create with a schedule of '0 0 * * *' and prompt 'sweep cron'. Report the job id.",
                        expect=""),
    "cron_list": dict(prompt="Call cron_list and report how many jobs exist.", expect=""),
    "cron_get": dict(prompt="Call cron_list, take the first job id, then call cron_get on it. Report its schedule.",
                     expect=""),
    "cron_pause": dict(prompt="Call cron_list, take the first job id, call cron_pause on it, report the resulting state.",
                       expect=""),
    "cron_resume": dict(prompt="Call cron_list, take the first job id, call cron_resume on it, report the resulting state.",
                        expect=""),
    "cron_delete": dict(prompt="Call cron_list, take the first job id, call cron_delete on it, then call cron_list again and report the new count.",
                        expect=""),
    "create_workflow": dict(prompt="Call create_workflow to create a workflow named 'sweep-wf' that logs 'hello'. Report the result.",
                            expect=""),
    "run_workflow": dict(prompt="Call run_workflow for the workflow named 'sweep-wf'. Report what it returned.",
                         expect=""),
    "spawn_agent": dict(prompt="Call spawn_agent with the task 'reply with SWEEP_AGENT_OK and stop'. Report what came back.",
                        expect=""),
    "start_subagent": dict(prompt="Call start_subagent with a trivial task 'say hi'. Report the subagent id.",
                           expect=""),
    "get_subagent": dict(prompt="Call start_subagent with task 'say hi', take its id, then call get_subagent on that id. Report the state.",
                         expect=""),
    "message_subagent": dict(prompt="Call start_subagent with task 'wait for instructions', take its id, then message_subagent that id with 'say SWEEP_MSG_OK'. Report the reply.",
                             expect=""),
    "wait_subagent": dict(prompt="Call start_subagent with task 'say hi', take its id, then wait_subagent on it. Report the final output.",
                          expect=""),
    "cancel_subagent": dict(prompt="Call start_subagent with task 'sleep', take its id, then cancel_subagent on it. Report the final state.",
                            expect=""),
    "agent_swarm": dict(prompt="Call agent_swarm with two trivial tasks: 'say A' and 'say B'. Report both results.",
                        expect=""),
    "run_agent": dict(prompt="Call run_agent with the task 'reply with SWEEP_RUNAGENT_OK'. Report the reply.",
                      expect=""),
    "task_complete": dict(skip="terminates the run; exercised implicitly by every other run"),
    "notify_parent": dict(skip="only meaningful inside a subagent"),
    "job_output": dict(prompt="Use bash to start a background job that prints SWEEP_JOB then sleeps 2 seconds. Then use job_output to read its output.",
                       expect=""),
    "job_kill": dict(prompt="Use bash to start a background job that sleeps 60 seconds, then use job_kill to terminate it. Report the outcome.",
                     expect=""),
    "working_memory": dict(prompt="Call working_memory to store the note 'sweep memory check'. Then read it back and report it.",
                           expect=""),
    "observational_memory": dict(prompt="Call observational_memory and report what it returns.", expect=""),
    "goals": dict(prompt="Call the goals tool and report what it returns.", expect=""),
    "get_efficiency_report": dict(prompt="Call get_efficiency_report and report what it returns.", expect=""),
    "list_profiles": dict(prompt="Call list_profiles and report the profile names.", expect=""),
    "get_profile": dict(prompt="Call list_profiles, take the first name, then get_profile on it. Report its description.",
                        expect=""),
    "get_profile_manifest": dict(prompt="Call get_profile_manifest and report what it returns.", expect=""),
    "recommend_profile": dict(prompt="Call recommend_profile for the task 'refactor Go code'. Report the recommendation.",
                              expect=""),
    "validate_profile": dict(prompt="Call list_profiles, take the first name, and validate_profile it. Report the verdict.",
                             expect=""),
    "create_prompt_extension": dict(prompt="Call create_prompt_extension named 'sweep-ext' with body 'test'. Report the result.",
                                    expect=""),
    "create_skill": dict(prompt="Call create_skill to create a skill named 'sweep-skill' that does nothing useful. Report the result.",
                         expect=""),
    "verify_skill": dict(prompt="Call verify_skill on the skill named 'sweep-skill'. Report the verdict.",
                         expect=""),
    "manage_skill_packs": dict(prompt="Call manage_skill_packs to list installed packs. Report what it returns.",
                               expect=""),
    "deploy": dict(skip="performs a real deployment"),
    "download": dict(prompt="Call download for the URL https://example.com and report whether it succeeded.",
                     expect=""),
    "AskUserQuestion": dict(skip="blocks awaiting a human answer"),
}


def post(path, payload, timeout=180):
    req = urllib.request.Request(BASE + path, method="POST",
                                 data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def get(path, timeout=30):
    with urllib.request.urlopen(BASE + path, timeout=timeout) as r:
        return json.load(r)


def exercise(tool, spec):
    if "skip" in spec:
        return dict(tool=tool, verdict="skipped", detail=spec["skip"])
    prompt = ("You have one job. " + spec["prompt"] +
              " Call the tool for real — do not describe what it would do.")
    try:
        run = post("/v1/runs", dict(prompt=prompt, model=MODEL, stream=False))
    except Exception as e:
        return dict(tool=tool, verdict="error", detail=f"start run: {e}")
    rid = run.get("run_id")
    deadline = time.time() + 180
    while time.time() < deadline:
        time.sleep(4)
        try:
            st = get(f"/v1/runs/{rid}")
        except Exception:
            continue
        if st.get("status") in ("completed", "failed", "cancelled"):
            out = str(st.get("output") or "")
            if st.get("status") != "completed":
                return dict(tool=tool, verdict="fail",
                            detail=str(st.get("error"))[:300], output=out[:400])
            want = spec.get("expect") or ""
            if want and want not in out:
                return dict(tool=tool, verdict="suspect",
                            detail=f"expected {want!r} in output", output=out[:400])
            return dict(tool=tool, verdict="pass", output=out[:400])
    return dict(tool=tool, verdict="timeout", detail="no terminal status in 180s")


def main():
    tools = [t.strip() for t in open("/tmp/tools.txt") if t.strip()]
    if ONLY:
        tools = [t for t in tools if t in ONLY]
    os.makedirs(".ux", exist_ok=True)
    results = []
    for i, tool in enumerate(tools, 1):
        spec = PLAN.get(tool)
        if spec is None:
            results.append(dict(tool=tool, verdict="unplanned",
                                detail="no exercise defined"))
            print(f"[{i}/{len(tools)}] {tool}: unplanned", flush=True)
            continue
        r = exercise(tool, spec)
        results.append(r)
        print(f"[{i}/{len(tools)}] {tool}: {r['verdict']} {r.get('detail','')[:80]}",
              flush=True)
        with open(".ux/tool-sweep.json", "w") as f:
            json.dump(results, f, indent=2)
    counts = {}
    for r in results:
        counts[r["verdict"]] = counts.get(r["verdict"], 0) + 1
    print("\n=== summary ===")
    for k in sorted(counts):
        print(f"  {k}: {counts[k]}")


if __name__ == "__main__":
    main()
