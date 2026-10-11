# Reverse ADR — detailed audit artifacts

This reference is loaded only for comprehensive historical audits or when the user asks for an evidence matrix. Routine reconstruction does not require every artifact.

## Artifacts

Default paths if the repository has no existing convention:
- `docs/decision-log/README.md`: historical index;
- `docs/decision-log/NNNN-short-title.md`: historical observations and design evolution;
- `docs/adr/README.md`: explicit decision routing index;
- `docs/adr/NNNN-short-title.md`: Proposed or owner-approved decisions only;
- `docs/adr/decision-ledger.md`: chronology and candidate decisions (kiroku: `docs/adr/phase1-decision-ledger.md`);
- `docs/adr/evidence.yaml`: claim-by-claim references and statuses;
- `docs/adr/audit.md`: coverage, checks and gaps.

Evidence matrix example (replace placeholders with real evidence; do not copy them as facts):

```yaml
revision: "<commit-sha>"
claims:
  - id: ADR-0001-C1
    statement: "<one falsifiable claim>"
    status: historical-only
    historical:
      - "<PR or review permalink>"
    current: []
    tests: []
    notes: "Current behavior not inspected"
```

Keep source references stable, prefer permalinks or commit-pinned paths, and never include sensitive session content, secrets or private user histories in public ADRs.

