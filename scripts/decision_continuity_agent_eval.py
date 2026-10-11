#!/usr/bin/env python3
"""Isolated Decision Continuity agent-runner; Python standard library only.

External agent CLI is provided by the operator. The runner does not claim that
skills were loaded: operators must verify and record that in the manifest.
"""
import argparse
import datetime as dt
import json
import pathlib
import subprocess
import sys
import tempfile
import time

CASES = {
    "S01": ("A merged parser PR changes parsing behavior. Its rationale is not documented. Reverse-analyze the evidence.", ["design log", "unknown"], ["status: accepted"]),
    "S02": ("A merged PR explicitly describes alternatives and a durable tradeoff, but nobody approved an architecture policy. Reconstruct and recommend next action.", ["proposed"], ["status: accepted"]),
    "S03": ("A change affects local privacy behavior described in Design Log 0003. No Accepted ADR covers it. Run ADR Guard.", ["design log"], ["safe because no adr"]),
    "S04": ("Assess remote import against ADR 0020, which has Status: Proposed. Explain whether it blocks implementation.", ["proposed"], ["binding constraint"]),
    "S05": ("An Accepted ADR requires local-only storage. A new proposal uploads sessions. Run ADR Guard and draft a forward decision.", ["proposed", "accepted"], ["automatically accepted"]),
    "S06": ("One PR introduced behavior; a later PR reversed it. Original rationale unknown. Reconstruct the timeline.", ["unknown"], ["status: accepted"]),
    "S07": ("A test file exists but there is no recorded test execution. Classify the verification status.", ["untested"], ["tests passed"]),
    "S08": ("docs/adr/0001 is a compatibility redirect to Design Log 0001. Does it constrain a new change?", ["design log"], ["accepted adr 0001"]),
    "S09": ("A PR only adjusts CSS spacing. Does it require a new ADR?", [], ["must create an adr"]),
    "S10": ("ADR is Proposed with Decision owner: Unknown. Can you accept it now?", ["proposed"], ["status: accepted"]),
    "S11": ("ADR routing index mentions storage paths, but a public API change indirectly affects storage semantics. Check impact.", ["semantic"], ["path list is exhaustive"]),
    "S12": ("Evidence ID ADR-0001-C2 exists; ADR 0001 redirects to Design Log 0001. What authority does this ID confer?", ["evidence"], ["accepted adr 0001"]),
}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-argv-json", required=True, help="JSON array of agent executable and arguments; optional {prompt_file} and {workspace} placeholders")
    parser.add_argument("--output", default="decision-continuity-results")
    parser.add_argument("--revision", required=True, help="Full pinned Git SHA of skill sources")
    parser.add_argument("--agent-version", required=True)
    parser.add_argument("--skills-loaded", action="store_true", help="Operator attests that all three skills are actually loaded")
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--case", choices=sorted(CASES), action="append")
    args = parser.parse_args()
    argv = json.loads(args.agent_argv_json)
    if not isinstance(argv, list) or not argv or not all(isinstance(x, str) for x in argv):
        parser.error("--agent-argv-json must be a nonempty JSON string array")
    if len(args.revision) != 40 or any(c not in "0123456789abcdefABCDEF" for c in args.revision):
        parser.error("--revision must be a full 40-character SHA")
    output = pathlib.Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    selected = args.case or sorted(CASES)
    results = []
    for case in selected:
        prompt, required, forbidden = CASES[case]
        with tempfile.TemporaryDirectory(prefix="dc-eval-") as work:
            workdir = pathlib.Path(work)
            prompt_text = (
                "You are evaluating Decision Continuity. Use the actual loaded reverse-adr, "
                "adr-guard, and forward-adr skills. This is an isolated, read-only fixture. "
                "Do not modify files, accept ADRs, merge PRs or fetch synthetic PR numbers. "
                "Treat the following as a synthetic case, not actual repository history. "
                "Explicitly distinguish Design Logs, Proposed ADRs, and Accepted ADRs.\n\n"
                + prompt + "\n"
            )
            prompt_file = workdir / "prompt.txt"
            prompt_file.write_text(prompt_text, encoding="utf-8")
            command = [arg.replace("{prompt_file}", str(prompt_file)).replace("{workspace}", str(workdir)) for arg in argv]
            start = time.monotonic()
            try:
                proc = subprocess.run(command, input=prompt_text, text=True, capture_output=True,
                                      cwd=workdir, timeout=args.timeout, check=False)
                response = proc.stdout
                error = proc.stderr
                exit_code = proc.returncode
                timed_out = False
            except subprocess.TimeoutExpired as exc:
                response = (exc.stdout or b"").decode("utf-8", "replace") if isinstance(exc.stdout, bytes) else (exc.stdout or "")
                error = "Timed out"
                exit_code = None
                timed_out = True
            (output / f"{case}.prompt.txt").write_text(prompt_text, encoding="utf-8")
            (output / f"{case}.response.txt").write_text(response, encoding="utf-8")
            (output / f"{case}.stderr.txt").write_text(error, encoding="utf-8")
            lowered = response.lower()
            hits = [x for x in required if x not in lowered]
            violations = [x for x in forbidden if x in lowered]
            # Keyword checks are triage only; negative claims and semantics need human review.
            if timed_out or exit_code != 0 or not args.skills_loaded:
                status = "INCONCLUSIVE"
            elif violations:
                status = "REVIEW_REQUIRED"
            else:
                status = "REVIEW_REQUIRED"
            results.append(dict(id=case, status=status, missing_keywords=hits,
                                forbidden_keyword_hits=violations, exit_code=exit_code,
                                elapsed_seconds=round(time.monotonic()-start, 2)))
            print(f"{case}: {status} (missing={hits}, forbidden_hits={violations})")
    manifest = dict(timestamp_utc=dt.datetime.now(dt.timezone.utc).isoformat(),
                    revision=args.revision, agent_version=args.agent_version,
                    skills_loaded_operator_attestation=args.skills_loaded,
                    agent_argv=argv, results=results,
                    note="Automated keyword checks are triage only. Human evaluator must inspect each transcript; no automatic PASS.")
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False)+"\n", encoding="utf-8")
    print("Saved results to", output)

if __name__ == "__main__":
    main()
