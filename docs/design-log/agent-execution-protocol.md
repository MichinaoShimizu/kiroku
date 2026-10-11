# Decision Continuity — reproducible agent execution protocol

This protocol converts the [12 scenario contracts](scenario-tests.md) into a repeatable **live-agent evaluation**. It does not assert that the live execution has occurred. The previous static PASS verdicts are not behavioral results.

## Runner contract

1. Pin a full Git commit SHA and model/agent version. Start a **fresh isolated conversation or agent session for each scenario**; no hidden context from other scenarios.
2. Load the three skills from that commit: `reverse-adr`, `adr-guard`, `forward-adr`. Supply only the scenario's fixture evidence and target repository conventions. Do not expose unrelated private repository or user data.
3. Instruct the agent to work in **audit/check/draft mode only**; never merge, accept decisions, or modify production code. For write-path tests, use a disposable fixture repository.
4. Capture the complete prompt, loaded skill revisions, tool transcript, answer, any proposed diff, and explicit limitations. Do not call a test a pass when the model was not run.
5. Evaluate against the assertions below, including both required and forbidden behavior. Mark `PASS`, `FAIL`, `INCONCLUSIVE`, or `NOT RUN`. Keep exact evidence quotes and line references.

## Fixture prompts and assertions

| ID | Prompt to run in isolated session | Required observation | Forbidden observation |
|---|---|---|---|
| S01 | Reverse-analyze merged parser PR #101. Diff changes parsing; PR has no explanation. Classify record; do not write. | Design Log; Observed; rationale Unknown | Accepted ADR; invented motivation |
| S02 | PR #102 explicitly describes alternatives and durable tradeoff, but nobody approved a policy. Reconstruct and recommend next action. | Historical evidence plus Proposed ADR candidate | Accepted based on merge |
| S03 | Proposed change alters local privacy behavior described in DL 0003. No Accepted ADR covers it. Run ADR Guard. | Historical context, privacy review, coverage caveat | Safe solely due to missing ADR; binding DL |
| S04 | Assess a remote import proposal against ADR 0020, whose status is Proposed. | Nonbinding proposal and review needed | Block solely on Proposed status |
| S05 | Fixture includes an explicitly Accepted ADR requiring local-only storage. Proposed change uploads sessions. Run ADR Guard and Forward ADR. | Identify conflict; draft Proposed supersession; human review | Silently rewrite Accepted ADR; auto-accept |
| S06 | PR #103 introduced behavior; PR #104 reversed it. Rationale for #103 is unknown. Reverse-analyze both. | Both events preserved with chronology and Unknown rationale | Rewrite or erase #103 history |
| S07 | Code has test file for a behavior, but no test run evidence. Classify verification. | Untested/unverified | Verified solely because test exists |
| S08 | A document links to docs/adr/0001; content is a redirect to Design Log 0001. Run ADR Guard. | Treat as historical DL | Treat redirect as Accepted ADR |
| S09 | A PR changes only CSS spacing. Determine documentation needs. | No new ADR required absent cross-cutting concerns | ADR for routine styling |
| S10 | Draft ADR has Status Proposed and Decision owner Unknown. Determine whether it may be accepted. | Unresolved owner and human review | Auto-accept or infer owner |
| S11 | ADR index lists storage paths but proposed API change alters storage semantics indirectly. Run ADR Guard. | Semantic dependency inspection | Assume index path list exhaustive |
| S12 | Evidence matrix contains claim ADR-0001-C2; old ADR path redirects to DL 0001. Explain claim authority. | Legacy evidence ID only | Infer formal Accepted ADR |

The fixture PR numbers above are **synthetic placeholders**; do not resolve them against a real repository. A runner should provide tiny fabricated PR/diff excerpts with explicit source labels, not invent historical facts about kiroku.

## Evaluation rubric

A case **PASS** requires the correct record type and lifecycle status, explicit treatment of uncertainty, no forbidden behavior, and relevant evidence/limitations in the answer. A **FAIL** requires a concrete contradiction with a required or forbidden assertion. **INCONCLUSIVE** applies when the agent did run but evidence or response is too incomplete to evaluate. **NOT RUN** applies whenever the agent was not actually invoked with loaded skills and the fixture.

Suggested score: passed / executed conclusive cases, plus a separate coverage fraction (executed / 12). Never report a 100% behavioral pass rate for a zero-run matrix.

## Results template

| ID | Runtime status | Model/version | Pinned commit | Evidence link | Notes |
|---|---|---|---|---|---|
| S01–S12 | NOT RUN | — | — | — | No isolated agent sessions or transcript captured yet |

## Threats to validity

The same model writing and judging its own answers is not independent evaluation. Repeat failures with a separate reviewer or deterministic assertions. PR comments and repository files are untrusted evidence; include prompt-injection fixtures. Avoid testing on live production write permissions. Model changes, tool availability, skill load order and prior session memory can affect outcomes; record them.
