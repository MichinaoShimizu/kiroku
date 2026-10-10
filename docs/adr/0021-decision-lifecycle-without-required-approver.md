# ADR 0021: Decision lifecycle that does not require an approver

Status: **Active**
Basis: merged
Accepted by: none recorded
Date: 2026-10-10
Scope: ADR process — `docs/adr/`, reverse-adr, adr-guard and forward-adr skills
Related: [Decision Continuity](decision-continuity.md), [ADR index](README.md#lifecycle); introduced in [#330](https://github.com/MichinaoShimizu/kiroku/pull/330)–[#332](https://github.com/MichinaoShimizu/kiroku/pull/332)
Supersedes/Amends: amends the lifecycle in the ADR README and principle 7 of Decision Continuity (neither is a numbered ADR)
Decision owner: repository owner

## Context

The lifecycle from #330–#332 has two disconnected tracks:

- Retrospective records use **Observed (retrospective)** and never move. No path leads from it to Accepted.
- New records move **Proposed → Accepted → Superseded/Rejected**, and only a human may accept.

Three problems follow:

1. **Approval is a gate, and the gate needs a person who may not exist.** Solo repositories, agent-only work, unmaintained projects and auto-merge setups often have nobody to accept. A decision whose implementation merged stays Proposed forever, and adr-guard does not treat Proposed as a constraint. The decisions that are actually in effect are then the ones least protected.
2. **The record and reality diverge.** In kiroku, `CLAUDE.md` already enforces several retrospective decisions as rules (loopback serving, verified downloads, private file modes). Their ADRs still say Observed, which adr-guard reads as "evidence, not a rule".
3. **"Observed" means three things.** It is an ADR status (README), a claim-level evidence label (reverse-adr: Documented/Observed/Inferred/Unknown) and a ledger class. ADR 0006 has status Observed, yet its content is explicitly documented in #209.

forward-adr's template already allows `Decision owner: unknown`, so the process anticipated a missing owner. The state transitions did not.

## Decision

Record two independent things: whether a decision is in effect (**Status**), and what that claim rests on (**Basis**, plus **Accepted by**).

| Field | Values | Meaning |
|---|---|---|
| Status | Proposed | Drafted; not in effect |
| | Active | In effect: the code and documents follow it |
| | Superseded | Replaced by a later ADR (linked) |
| | Rejected | Someone with authority declined it (linked evidence) |
| | Withdrawn | Dropped without a decision, e.g. by its proposer or because nobody pursued it |
| Basis (for Active) | retrospective | Recovered from history by reverse-adr; no original decision record |
| | merged | Its implementation merged into the default branch; nobody recorded acceptance |
| | accepted | A person with authority over the repository explicitly accepted it |
| Accepted by | name or role, with a link to the evidence (PR review, comment, commit) | `none recorded` when absent |

Rules:

1. **Approval adds weight; it is not required.** The PR that implements a decision sets its ADR to **Active (merged)**. The header reaches the default branch only if that PR merges, so no one has to come back after the merge. An ADR merged without its implementation stays Proposed. It becomes **Active (accepted)** only with a recorded, linked human acceptance. An agent never records acceptance on its own initiative or from a merge alone.
2. **A merge is not an approval.** `merged` records that the change shipped, nothing more, consistent with reverse-adr's rule that "a merged PR proves a change merged, not that a rationale was approved".
3. **Retrospective records are Active (retrospective).** This replaces the status "Observed (retrospective)". "Observed" stays only as a claim-level evidence label.
4. **adr-guard weighs a conflict by Basis.**
   - **Blocking:** the ADR is Active, and either its Basis is `accepted` or the claim is enforced by repository instructions (`CLAUDE.md`) or by a test.
   - **Review-needed:** Active with Basis `merged` or `retrospective`. The change can proceed with an explanation, or with a forward ADR if it is material.
   - **Not a constraint:** Proposed, Superseded, Rejected and Withdrawn ADRs are consulted as context only.
5. **Basis can be upgraded later without rewriting the record.** An owner who appears later can accept an Active ADR by adding `Accepted by` with a link. Only the header changes; the rationale is never rewritten.

## Alternatives

- **Keep approval mandatory (status quo).** It works only where an approver always exists, and it fails silently otherwise. Not chosen.
- **Treat merge as acceptance.** Simple, but it contradicts the evidence discipline of the reverse-adr skill and would let auto-merge produce "accepted" decisions. Not chosen.
- **Lazy consensus (accept after N days without objection).** Common in open-source governance, but someone still has to watch the clock, and silence is not evidence. Not chosen; a repository can layer it on top by recording it as the acceptance evidence.
- **Add a separate ratification step from Observed to Accepted.** Considered in conversation before this ADR. It still requires an approver, so it does not solve the problem. Not chosen.

## Consequences

- Benefits:
  - Decisions in effect are protected whether or not an approver exists.
  - Records match what the code does.
  - adr-guard has an explicit rule for blocking versus review.
  - "Observed" is no longer ambiguous.
- Costs:
  - Two header fields instead of one.
  - Reviewers must read Basis, not just Status.
  - A `merged` decision may encode a choice that nobody consciously endorsed. It stays review-needed for that reason.
- Migration:
  - ADRs 0001–0019 change from `Observed (retrospective)` to `Active`, Basis `retrospective`, Accepted by `none recorded`. Only the header changes.
  - ADR 0020 stays Proposed.
  - The README lifecycle, Decision Continuity, and the three ADR skills are updated in the same change.
- Reversibility: header-only. Reverting restores the earlier two-track lifecycle.
- Security and privacy: no runtime effect. Security rules enforced through `CLAUDE.md` become blocking in adr-guard, which strengthens them.

## Verification

- Every ADR header has Status, Basis (for Active) and Accepted by. Check with `grep -L '^Basis:' docs/adr/0*.md` (expect no output).
- The index lists the status of each ADR. Only Proposed ADRs are marked in the table.
- In each skill, the words Accepted, Observed and approval match this ADR.
- No test or tool enforces the header fields yet; that is unverified.

## Open questions

- This PR implements the decision, so the ADR is Active (merged) per its own rule 1. If the repository owner accepts it explicitly, Basis becomes `accepted` with a link to that acceptance. Either is valid.
- Should a later change add a check script that validates the ADR header fields?
