# Live-agent scenario runner

The repository includes [a Python standard-library runner](../../scripts/decision_continuity_agent_eval.py) for twelve isolated Decision Continuity cases. This is a **runner**, not a bundled agent or an automatic proof of correctness. It invokes a CLI chosen by the operator; no agent CLI, credentials or model access are included.

## Run

Use a trusted local agent CLI that loads all three skills from a pinned checkout. The executable must accept the prompt on stdin or support a `{prompt_file}` argument. For example, **replace** `/path/to/your-agent` and its arguments with your real installed agent invocation:

```sh
python3 scripts/decision_continuity_agent_eval.py \\
  --revision FULL_40_CHARACTER_GIT_SHA \\
  --agent-version 'AGENT_AND_MODEL_VERSION' \\
  --agent-argv-json '["/path/to/your-agent"]' \\
  --skills-loaded \\
  --output /tmp/decision-continuity-results
```

`--skills-loaded` is an **operator attestation**, not automated verification. Omit it when the agent's skill loading cannot be confirmed. The runner creates a fresh temporary working directory per case, invokes a new subprocess, captures prompt/stdout/stderr, and writes a JSON manifest. Isolation is limited to the process working directory; a CLI that restores its own global memory/session or reads user credentials is **not** fully isolated. Use a disposable account/environment with no production write permissions.

The runner always returns `REVIEW_REQUIRED` for successful executions and `INCONCLUSIVE` for unsuccessful/unattested executions. Keyword hits are diagnostic, not a PASS/FAIL oracle. A human reviewer must inspect transcripts against the [scenario matrix](scenario-tests.md) and [execution protocol](agent-execution-protocol.md) before recording behavioral PASS/FAIL. Never upload transcripts containing secrets or private histories.

## Execution status

No real agent subprocess has been invoked by the repository change. Live scenario coverage remains **0/12** until an operator runs the command with a working CLI and reviews the resulting evidence.
