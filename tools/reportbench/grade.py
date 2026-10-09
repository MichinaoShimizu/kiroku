#!/usr/bin/env python3
"""報告プロンプトに AI が書いた報告を、機械で確かめられるところだけ採点する（/report-eval のスキル用）。

  python3 -I tools/reportbench/grade.py <作業ディレクトリ>

読むもの（作業ディレクトリの中）:
  prompts/{day,week,month}.md と prompts/sizes.json（run.sh が作る）
  answers/<scope>-<n>.md（AI が書いた報告。scope は day・week・month）
  answers/runs.json（あれば。{"week-1": {"tokens": 55234, "tool_uses": 3, "files": ["…"]}} の形。スキルの手順で書く）
書くもの: grade.json と grade.md。分かりやすさ（伝わりやすさ）は機械では測れないので、スキルの手順で別に採点する。
"""
import json, os, re, sys

W = os.path.abspath(sys.argv[1])
truth = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "truth.json")))
sizes = json.load(open(os.path.join(W, "prompts", "sizes.json")))
runs_p = os.path.join(W, "answers", "runs.json")
runs = json.load(open(runs_p)) if os.path.exists(runs_p) else {}
SECTIONS = ["summary", "numbers", "by project", "advice from an expert"]
norm = lambda s: re.sub(r"\s+", " ", s).strip()


def table_rows(prompt):  # Facts の「## Numbers」の表（見出しの行を除く）
    m = re.search(r"^## Numbers\n((?:\|.*\n)+)", prompt, re.M)
    return [norm(l) for l in m.group(1).splitlines()[2:]] if m else []


def grade(scope, prompt, ans, run):
    lo = ans.lower(); out = {}
    heads = [h.strip().lower() for h in re.findall(r"^#{2,4}\s+(.+?)\s*$", ans, re.M)]
    pos = [next((i for i, h in enumerate(heads) if h.startswith(s)), -1) for s in SECTIONS]
    out["format"] = all(p >= 0 for p in pos) and pos == sorted(pos)
    rows = table_rows(prompt); body = norm(ans)
    out["numbers_copied"] = f"{sum(r in body for r in rows)}/{len(rows)}"
    urls = lambda t: {u.rstrip(".,;:!?'\"") for u in re.findall(r"https?://[^\s)>\]]+", t)}  # 文の終わりの記号は URL に入れない
    urls_p, urls_a = urls(prompt), urls(ans)
    out["invented_links"] = sorted(urls_a - urls_p)
    hx_p, hx_a = set(re.findall(r"\b[0-9a-f]{12}\b", prompt)), set(re.findall(r"\b[0-9a-f]{12}\b", ans))
    out["invented_hashes"] = sorted(hx_a - hx_p)
    t = truth[scope]
    miss = [g for g in t["must"] if not any(w.lower() in lo for w in g)]
    out["recall"] = f"{len(t['must']) - len(miss)}/{len(t['must'])}"; out["missing"] = [g[0] for g in miss]
    m = re.search(r"^#{2,4}\s+advice from an expert\s*$(.*)", ans, re.M | re.S | re.I); adv = m.group(1).lower() if m else ""
    out["advice_missing"] = [g[0] for g in t.get("advice", []) if not any(w.lower() in adv for w in g)]
    out["violations"] = [w for w in t["must_not"] if w.lower() in lo] + ([W] if W in ans else []) + (["/home/"] if "/home/" in ans else [])
    out["should_not"] = [w for w in t["should_not"] if re.search(r"\b" + re.escape(w.lower()) + r"\b", lo)]
    allowed = {os.path.join(W, "prompts", scope + ".md")} | set(re.findall(r"History file: (\S+)", prompt))
    files = run.get("files", [])
    out["history_files_opened"] = len([f for f in files if f in allowed and not f.endswith(".md")])
    out["files_outside"] = [f for f in files if f not in allowed and not any(f.startswith(a[:-6]) for a in allowed if a.endswith(".jsonl"))]
    out["agent_tokens"] = run.get("tokens"); out["tool_uses"] = run.get("tool_uses")
    out["answer_chars"] = len(ans)
    out["pass"] = out["format"] and out["numbers_copied"].split("/")[0] == out["numbers_copied"].split("/")[1] and not out["invented_links"] \
        and not out["invented_hashes"] and not out["missing"] and not out["advice_missing"] and not out["violations"] and not out["files_outside"]
    return out


res = {}
adir = os.path.join(W, "answers")
for f in sorted(os.listdir(adir)) if os.path.isdir(adir) else []:
    m = re.match(r"(day|week|month)-(\w+)\.md$", f)
    if not m: continue
    scope = m.group(1)
    res[f[:-3]] = grade(scope, open(os.path.join(W, "prompts", scope + ".md")).read(), open(os.path.join(adir, f)).read(), runs.get(f[:-3], {}))
json.dump({"sizes": sizes, "answers": res}, open(os.path.join(W, "grade.json"), "w"), indent=2)
L = ["| answer | pass | format | numbers | recall | missing | advice missing | violations | should_not | invented | files outside | history files | agent tokens | tool uses |",
     "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|"]
for k, r in res.items():
    L.append(f"| {k} | {'✓' if r['pass'] else '✗'} | {'✓' if r['format'] else '✗'} | {r['numbers_copied']} | {r['recall']} | {', '.join(r['missing']) or '—'} | {', '.join(r['advice_missing']) or '—'} | {', '.join(r['violations']) or '—'} | {', '.join(r['should_not']) or '—'} | "
             f"{len(r['invented_links']) + len(r['invented_hashes'])} | {len(r['files_outside'])} | {r['history_files_opened']} | {r['agent_tokens'] or '—'} | {r['tool_uses'] or '—'} |")
L += ["", "| prompt | chars | approx tokens |", "|---|---|---|"] + [f"| {k} | {v['chars']} | {v['approxTokens']} |" for k, v in sizes.items()]
open(os.path.join(W, "grade.md"), "w").write("\n".join(L) + "\n")
print("\n".join(L))
