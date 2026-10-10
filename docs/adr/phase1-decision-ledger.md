# Phase 1 decision recovery — evidence ledger

Status: **In progress (retrospective)**. This ledger captures *documented decisions and reversals* in merged PRs, not invented meeting records. The GitHub merged-PR listing contained 318 entries when inspected on 2026-10-10. The entries below are a curated first pass, **not** an exhaustive review of all 318 PRs. Use this ledger to nominate focused ADRs; keep implementation details and minor UI changes here rather than creating an ADR for every PR.

## Evidence classes

- **Explicit**: a PR or issue says why a choice was made.
- **Observed**: implementation reveals a behavior, but historical motivation is not proven.
- **Reversed**: a later PR explicitly replaces or withdraws an earlier choice.

| Topic | Decision / evolution | Evidence | Class | Suggested ADR |
|---|---|---|---|---|
| Initial delivery | Start with Python stdlib and a standalone HTML report of local Claude/Kiro sessions. | [#1](https://github.com/MichinaoShimizu/kiroku/pull/1) | Explicit | Architecture/runtime |
| Runtime and distribution | Replace Python with Go for cross-OS single-binary delivery and agent adapters. | [#6](https://github.com/MichinaoShimizu/kiroku/pull/6) | Explicit; replaces #1 runtime | Architecture/runtime |
| Agent source abstraction | Share session normalization while retaining source-specific parsers. | [#6](https://github.com/MichinaoShimizu/kiroku/pull/6), [#7](https://github.com/MichinaoShimizu/kiroku/pull/7), [#9](https://github.com/MichinaoShimizu/kiroku/pull/9) | Explicit | Source model |
| Kiro Crew accounting | Treat Kiro CLI as the source of conversation/usage and Crew data as provenance/title to prevent double counting. | [#8](https://github.com/MichinaoShimizu/kiroku/pull/8) | Explicit | Identity/dedup |
| Historical compatibility | Support old Kiro/Amazon Q SQLite formats through shared QStore; prefer Kiro conversations_v2 on duplicate IDs. | [#7](https://github.com/MichinaoShimizu/kiroku/pull/7) | Explicit | Source model |
| Codex coverage | Read native Codex session directories and archived zstd history; avoid duplicate filenames. | [#9](https://github.com/MichinaoShimizu/kiroku/pull/9) | Explicit | Source model |
| Usage accounting | Deduplicate split Claude responses by message/request ID before token aggregation. | [#4](https://github.com/MichinaoShimizu/kiroku/pull/4) | Explicit | Metrics semantics |
| Native metrics | Present agent-specific metrics as references, with sample counts and without good/bad grading. | [#10](https://github.com/MichinaoShimizu/kiroku/pull/10) | Explicit | Nonjudgmental metrics |
| Live UX | Introduce loopback serve and file-stamp watching instead of only one-shot static exports. | [#11](https://github.com/MichinaoShimizu/kiroku/pull/11) | Explicit | Local serving |
| Reload cost | Debounce history reparsing while bounding stale-display time during continuous writes. | [#23](https://github.com/MichinaoShimizu/kiroku/pull/23) | Explicit | Local serving |
| CLI contract | Make `serve` the primary subcommand; preserve `html` for one-file export. | [#18](https://github.com/MichinaoShimizu/kiroku/pull/18) | Explicit | CLI compatibility |
| Release model | MIT license, multi-OS release verification and constrained publishing permissions. | [#12](https://github.com/MichinaoShimizu/kiroku/pull/12) | Explicit | Release/security |
| Reflective guidance | Introduce weekly reflection, then KPI-playbook decision flow; later remove prescriptive reflection in favor of descriptive weekly/monthly summaries. | [#2](https://github.com/MichinaoShimizu/kiroku/pull/2), [#5](https://github.com/MichinaoShimizu/kiroku/pull/5), [#14](https://github.com/MichinaoShimizu/kiroku/pull/14) | **Reversed** | Product philosophy |
| External AI boundary | Offer copyable prompts for users' own AI, without kiroku invoking AI itself; later remove the suggestion panel. | [#22](https://github.com/MichinaoShimizu/kiroku/pull/22), [#304](https://github.com/MichinaoShimizu/kiroku/pull/304) | **Reversed UI; boundary retained** | Privacy/product philosophy |
| UI originality | Redesign the borrowed visual layout before public release without changing measurement semantics. | [#13](https://github.com/MichinaoShimizu/kiroku/pull/13) | Explicit | UX decisions ledger |
| README scope | Keep README focused on purpose, appearance, install and use; move contributor details into docs. | [#19](https://github.com/MichinaoShimizu/kiroku/pull/19), [#21](https://github.com/MichinaoShimizu/kiroku/pull/21) | Explicit | Documentation policy |
| Public JSON compatibility | Version `kiroku json` schema, remove old CLI aliases, rename ambiguous `--root`, and preserve archive/serve-key compatibility. | [#208](https://github.com/MichinaoShimizu/kiroku/pull/208), [#209](https://github.com/MichinaoShimizu/kiroku/issues/209), [#211](https://github.com/MichinaoShimizu/kiroku/pull/211) | Explicit | Compatibility |
| Private histories | For Kiro Crew incognito/temporary sessions, preserve counts/timing but not content; avoid duplicate rows. | [#312](https://github.com/MichinaoShimizu/kiroku/pull/312) | Explicit | Privacy/dedup |
| Source correctness | Suppress copied Claude branch/fork histories and mirrored Codex review events from double counting. | [#311](https://github.com/MichinaoShimizu/kiroku/pull/311), [#310](https://github.com/MichinaoShimizu/kiroku/pull/310) | Explicit | Identity/dedup |
| Shell safety | Reject problematic quote characters in generated copy/paste commands. | [#307](https://github.com/MichinaoShimizu/kiroku/pull/307) | Explicit | Security |
| Year in Review | Reintroduce the shareable annual view while removing skill levels and personal rankings. | [#320](https://github.com/MichinaoShimizu/kiroku/pull/320) | **Reversed grading approach** | Product philosophy |
| Value proposition | Present the product as Remember → Understand → Improve, not visualization alone. | [#322](https://github.com/MichinaoShimizu/kiroku/pull/322) | Explicit | Product philosophy |

## Next recovery passes

1. Inspect the remaining merged PR titles/bodies systematically in chronological batches, excluding routine release/dependency bumps from ADR candidacy but preserving them in audit counts.
2. Read linked issues, review discussions and diffs for high-impact decisions; distinguish *intent at the time* from *current implementation*.
3. For each nominated ADR, record original choice, alternatives actually discussed, reversals/superseding PRs, consequences and code/tests that enforce it.
4. Reconcile the chronology with `CHANGELOG.md`, `docs/compatibility.md` and historical source versions. Do not mark any decision Accepted solely because its PR was merged; historical evidence and current status are different concepts.

## Second pass: additional historical decision candidates

This pass screened titles of the remaining historical merged PRs in numbered ranges 26–322 and selected explicit rationale from PR summaries already retrieved. The list is not a claim that all reviews and diffs have been inspected.

| Topic | Decision / evolution | Evidence | Class | Suggested ADR |
|---|---|---|---|---|
| Git output provenance | Add local commits and file/PR output signals to the calendar and summaries, with later adjustments to which metrics count as outcomes. | [#27](https://github.com/MichinaoShimizu/kiroku/pull/27), [#28](https://github.com/MichinaoShimizu/kiroku/pull/28), [#112](https://github.com/MichinaoShimizu/kiroku/pull/112) | Evolution | Output attribution |
| Measurement vs coaching | Add observations and interventions, then remove the action buttons and prescribed follow-up UI. | [#41](https://github.com/MichinaoShimizu/kiroku/pull/41), [#53](https://github.com/MichinaoShimizu/kiroku/pull/53), [#57](https://github.com/MichinaoShimizu/kiroku/pull/57), [#81](https://github.com/MichinaoShimizu/kiroku/pull/81) | Reversed | Product philosophy |
| Cache granularity | Move from full history rereads toward changed-file loading and affected-period recalculation. | [#69](https://github.com/MichinaoShimizu/kiroku/pull/69), [#129](https://github.com/MichinaoShimizu/kiroku/pull/129), [#258](https://github.com/MichinaoShimizu/kiroku/pull/258) | Evolution | Performance |
| History preservation | Add `archive` to preserve history agents may delete; handle symlinked folders and deleted subagent copies. | [#75](https://github.com/MichinaoShimizu/kiroku/pull/75), [#86](https://github.com/MichinaoShimizu/kiroku/pull/86) | Explicit | Retention |
| Yearly reflection | Add annual exposure and shareable image, introduce grades, hide the feature, later restore it without grading. | [#76](https://github.com/MichinaoShimizu/kiroku/pull/76), [#78](https://github.com/MichinaoShimizu/kiroku/pull/78), [#88](https://github.com/MichinaoShimizu/kiroku/pull/88), [#320](https://github.com/MichinaoShimizu/kiroku/pull/320) | Reversed | Product philosophy |
| Locale policy | Introduce English support, then English-only UI, later restore Japanese README. | [#46](https://github.com/MichinaoShimizu/kiroku/pull/46), [#110](https://github.com/MichinaoShimizu/kiroku/pull/110), [#317](https://github.com/MichinaoShimizu/kiroku/pull/317) | Evolution | Documentation |
| Security hardening | Tighten file writes, parser boundaries, serving and Git execution; add CSP and escape hostile strings. | [#159](https://github.com/MichinaoShimizu/kiroku/pull/159), [#161](https://github.com/MichinaoShimizu/kiroku/pull/161), [#162](https://github.com/MichinaoShimizu/kiroku/pull/162) | Explicit | Security |
| Serve access | Require a key for access to history from `serve`; separately block DNS rebinding. | [#172](https://github.com/MichinaoShimizu/kiroku/pull/172), [#127](https://github.com/MichinaoShimizu/kiroku/pull/127) | Explicit | Local serving |
| Source fidelity | Represent missing agent metrics as “not recorded” rather than zero, and explain per-agent coverage. | [#214](https://github.com/MichinaoShimizu/kiroku/pull/214), [#215](https://github.com/MichinaoShimizu/kiroku/pull/215) | Explicit | Measurement semantics |
| Traceable breakdowns | Let users inspect contributing records behind summary figures instead of relying only on totals. | [#253](https://github.com/MichinaoShimizu/kiroku/pull/253), [#259](https://github.com/MichinaoShimizu/kiroku/pull/259) | Evolution | Explainability |
| Report-generation boundary | Replace generated report drafts with prompts for a user-selected agent; later standardize the prompt format and evaluation. | [#292](https://github.com/MichinaoShimizu/kiroku/pull/292), [#298](https://github.com/MichinaoShimizu/kiroku/pull/298), [#299](https://github.com/MichinaoShimizu/kiroku/pull/299) | Explicit | External AI boundary |
| No implicit network fetch | Avoid Git fetching remote data while reading commits. | [#293](https://github.com/MichinaoShimizu/kiroku/pull/293) | Explicit | Security/privacy |
| Source audit discipline | Establish repeatable agent source auditing and agent add/drop and pricing update skills. | [#201](https://github.com/MichinaoShimizu/kiroku/pull/201), [#243](https://github.com/MichinaoShimizu/kiroku/pull/243), [#245](https://github.com/MichinaoShimizu/kiroku/pull/245) | Explicit | Maintenance |

### Important distinctions

- `#110` concerns UI/documentation language, while `#317` restores Japanese **README**, not necessarily Japanese in-product localization.
- `#172` and `#127` should be read together to distinguish loopback defaults from access controls on deliberately exposed servers.
- `#214` is a measurement contract, not a display-only detail: unknown/missing is not zero.
- A PR title alone is weaker evidence than its body, review and diff. Validate the rationale for second-pass entries before marking their proposed ADRs Accepted.
