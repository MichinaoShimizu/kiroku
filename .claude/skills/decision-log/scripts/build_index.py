#!/usr/bin/env python3
"""Decision log のフォルダーの索引 README.md を、フォルダーの Decision log 全部から作り直す。

  build_index.py <フォルダー> [--repo NAME] [--lang ja|en]   README.md を書く
  build_index.py <フォルダー> --check         README.md が最新で、各 Decision log の形が崩れていないかを確かめる
                                             （どちらかが駄目なら終了コード 1）

Decision log は日本語でも英語でも読む（references/writing.md の「英語で書くとき」）。索引の言語は --lang、
なければいまの索引の言語、なければ多いほうの言語（同じなら日本語）。標準ライブラリだけを使う。
"""
import argparse
import pathlib
import re
import subprocess
import sys

# 正しいファイル名。ここから外れる名前も読むが、直すよう知らせる
STRICT_RE = re.compile(r"^\d{4,}(?:-[a-z0-9-]+)?\.md$")
# (category, 見出し, 中身)。references/categories.md の表と同じにする（テストで照らす）
CATEGORIES = [
    ("architecture", "設計・構成", "分割と統合、依存の向き、採用する技術、基盤の移行"),
    ("data", "データの意味・持ち方", "スキーマ、値の意味と単位、保存先、保持期間（個人情報は privacy）"),
    ("security", "認証・権限・セキュリティ", "認証方式、権限の境界、秘密情報、署名と検証、暗号化"),
    ("privacy", "個人情報", "個人情報を集めるか、どこに出す・送る・残すか（ログ・送信先・保持期間）、マスキング"),
    ("compatibility", "互換性・外部との約束", "公開 API、形式、バージョン、廃止"),
    ("reliability", "性能・信頼性", "負荷や障害のときの振る舞い。タイムアウト、リトライ、上限、キャッシュ、縮退"),
    ("release", "リリース・運用", "デプロイの手順、フラグ、環境、監視・アラートと通知先、ログ・トレースの出し方"),
    ("engineering", "開発の決まり", "CI の必須チェック、テストの方針、lint・コードの規約、ブランチの運用"),
    ("cost", "コスト", "インスタンスやプランの大きさ、サンプリングの割合、コストのための削減や上限"),
    ("product", "プロダクトの方針", "仕様の線引き、やらないと決めたこと"),
    ("other", "その他", "どれにも当たらないもの。多ければ分類を見直す"),
]
LABELS = {key: label for key, label, _ in CATEGORIES}
# 英語で書くときの (見出し, 中身)。references/categories.md の英語の表と同じにする（テストで照らす）
CATEGORIES_EN = {
    "architecture": ("Architecture", "Splitting and merging, direction of dependencies, adopted technology, platform migrations"),
    "data": ("Data meaning and storage", "Schemas, meaning and units of values, where data is stored, retention (personal data is privacy)"),
    "security": ("Authentication, permissions and security", "Authentication methods, permission boundaries, secrets, signing and verification, encryption"),
    "privacy": ("Personal data", "Whether personal data is collected, where it is shown, sent or kept (logs, destinations, retention), masking"),
    "compatibility": ("Compatibility and external contracts", "Public APIs, formats, versions, deprecation"),
    "reliability": ("Performance and reliability", "Behavior under load and failure: timeouts, retries, limits, caches, degradation"),
    "release": ("Release and operations", "Deployment steps, flags, environments, monitoring, alerts and who gets them, logging and tracing"),
    "engineering": ("Engineering rules", "Required CI checks, testing policy, lint and code conventions, branch workflow"),
    "cost": ("Cost", "Instance and plan sizes, sampling rates, cuts and limits made for cost"),
    "product": ("Product policy", "Where the spec draws the line, things decided not to do"),
    "other": ("Other", "Anything that fits none of the above; if there are many, revisit the categories"),
}
# 決まった言葉。日本語と英語のどちらの Decision log も読む
WORDS = {
    "ja": dict(happened="起きたこと", reason="理由", recat="**分類を更新**", retire="**終了**", learned="**分かった**",
               reasons=("記録あり", "推測", "不明"), unknown="不明"),
    "en": dict(happened="What happened", reason="Reason", recat="**Category updated**", retire="**Retired**",
               learned="**Answered**", reasons=("Documented", "Inferred", "Unknown"), unknown="Unknown"),
}
# 0001.md、0001-auth-pkce.md のほか、0001-Short_Title.md・0001-認証の変更.md と、DL- を付けた DL-1.md・DL-0001-x.md も読む
FILE_RE = re.compile(r"^(?:DL-(\d+)|(\d{4,})(?!-\d{2}-\d{2}))(?:[-_].*)?\.md$", re.I)
FRONT_RE = re.compile(r"\A---\n(.*?)\n---[ \t]*\n+", re.S)
APPEND_RE = re.compile(r"^## (?:追記|Addendum) \d{4}-\d{2}-\d{2}.*$", re.M)
CLAIM_DATE_RE = re.compile(r"^- C\d+ (\d{4}-\d{2}-\d{2}) ", re.M)


def field(text, key):
    m = re.search(rf"^{key}:\s*([^#\n]*?)\s*(#.*)?$", text, re.M)
    return m.group(1).strip("'\" ") if m else None


def section(body, heading):
    m = re.search(rf"^## {re.escape(heading)}\n(.*?)(?=^## |\Z)", body, re.S | re.M)
    return m.group(1) if m else ""


def number(name):
    m = FILE_RE.match(name)
    return int(m.group(1) or m.group(2))


def read_log(path):
    text = path.read_text(encoding="utf-8")
    m = FRONT_RE.match(text)
    front, rest = (m.group(1), text[m.end():]) if m else ("", text)
    title_m = re.match(r"# (.+)\n", rest)
    title = re.sub(r"^\[[^\]]+\]\s*", "", title_m.group(1).strip()) if title_m else path.stem
    parts = APPEND_RE.split(rest)
    body, appends = parts[0], [p.strip() for p in parts[1:]]
    lang = "en" if re.search(r"^## What happened$", body, re.M) else "ja"
    w = WORDS[lang]
    category = field(front, "category") or "other"
    retired = learned = False
    for note in appends:
        first = note.split("\n", 1)[0]
        if first.startswith(w["recat"]):
            category = field(note, "category") or category
        elif first.startswith(w["retire"]):
            retired = True
        elif first.startswith(w["learned"]):
            learned = True
    dates = CLAIM_DATE_RE.findall(section(body, w["happened"]))
    reason = next((l.strip() for l in section(body, w["reason"]).splitlines() if l.strip()), "")
    problems = []
    if path.name.upper().startswith("DL-"):
        problems.append("ファイル名に DL- を付けない（DL-1 は文中で指すときの書き方）。0001-<名前>.md に変える")
    elif not STRICT_RE.match(path.name):
        problems.append("ファイル名は 0001.md か 0001-<英小文字・数字・ハイフン>.md にする")
    if not front:
        problems.append("フロントマターがない")
    elif field(front, "kind") != "log":
        problems.append("フロントマターの kind が log でない")
    if (field(front, "category") or "other") not in LABELS:
        problems.append(f"category が分類の一覧にない: {field(front, 'category')}")
    if not title_m:
        problems.append("タイトル（# の行）がない")
    if not dates:
        problems.append(f"「## {w['happened']}」に `- C<番号> YYYY-MM-DD <状態>：` の行がない")
    if not reason.startswith(w["reasons"]):
        problems.append(f"「## {w['reason']}」の 1 行目が「{'」「'.join(w['reasons'])}」で始まっていない")
    return dict(problems=problems, number=number(path.name), file=path.name, title=title,
                category=category if category in LABELS else "other", retired=retired, lang=lang,
                date=min(dates) if dates else None,
                unknown=reason.startswith(w["unknown"]) and not learned)


def line(log, lang="ja"):
    date = log["date"] or ("no date" if lang == "en" else "日付なし")
    mark = (" · reason unknown" if lang == "en" else "・理由不明") if log["unknown"] else ""
    return f"- {date} [DL-{log['number']}]({log['file']}) {log['title']}{mark}"


def order(log):
    return (log["date"] or "9999-99-99", log["number"])


def read_logs(folder):
    return [read_log(p) for p in sorted(folder.glob("*.md")) if FILE_RE.match(p.name)]


def unindexed(folder):
    """番号を読めず、索引に入らない Markdown（README.md を除く）。"""
    return sorted(p.name for p in folder.glob("*.md") if p.name != "README.md" and not FILE_RE.match(p.name))


def build(logs, repo, lang="ja"):
    en = lang == "en"
    out = [f"# Decision logs ({repo})" if en else f"# Decision logs（{repo}）", "",
           "> Rebuilt by decision-log from each Decision log; don't edit it by hand. These are past observations and bind nothing."
           if en else "> decision-log が各 Decision log から作り直す索引。手で直さない。過去の観察であり、何も縛らない。"]
    for key, label, about in CATEGORIES:
        if en:
            label, about = CATEGORIES_EN[key]
        group = sorted((l for l in logs if l["category"] == key and not l["retired"]), key=order)
        if group:
            out += ["", f"## {label} ({key})" if en else f"## {label}（{key}）", "", about, ""] + [line(l, lang) for l in group]
    retired = sorted((l for l in logs if l["retired"]), key=order)
    if retired:
        out += ["", "## Retired" if en else "## 取り下げ", ""] + [line(l, lang) for l in retired]
    return "\n".join(out) + "\n"


def index_lang(folder, logs):
    """索引の言語。いまの索引の言語を保ち、なければ多いほうの言語（同じなら日本語）。"""
    index = folder / "README.md"
    if index.exists():
        title = index.read_text(encoding="utf-8").split("\n", 1)[0]
        if title.startswith("# Decision logs ("):
            return "en"
        if title.startswith("# Decision logs（"):
            return "ja"
    english = sum(l["lang"] == "en" for l in logs)
    return "en" if english > len(logs) - english else "ja"


def duplicates(logs):
    """同じ番号を持つファイル。ほかのブランチと同じ番号でマージされたときに起きる。"""
    seen = {}
    for log in logs:
        seen.setdefault(log["number"], []).append(log["file"])
    return [files for _, files in sorted(seen.items()) if len(files) > 1]


def repo_name(folder):
    """見出しに出すリポジトリ名。いまの索引にあればそれを使い、--repo の有無で索引が変わらないようにする。"""
    index = folder / "README.md"
    if index.exists():
        m = re.match(r"# Decision logs(?:（(.+)）| \((.+)\))\n", index.read_text(encoding="utf-8"))
        if m:
            return m.group(1) or m.group(2)
    try:
        url = subprocess.run(["git", "-C", str(folder), "remote", "get-url", "origin"],
                             capture_output=True, text=True, check=True).stdout.strip()
        return re.sub(r"\.git$", "", url.rstrip("/").split("/")[-1].split(":")[-1])
    except (OSError, subprocess.CalledProcessError):
        return folder.resolve().parent.name


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("folder", type=pathlib.Path)
    p.add_argument("--repo", help="見出しに出すリポジトリ名（既定はいまの索引の名前、なければ git の origin から）")
    p.add_argument("--lang", choices=("ja", "en"), help="索引の言語（既定はいまの索引の言語、なければ多いほうの言語）")
    p.add_argument("--check", action="store_true", help="書かずに、README.md が最新かを確かめる")
    args = p.parse_args()
    if not args.folder.is_dir():
        print(f"{args.folder} がない", file=sys.stderr)
        return 2
    logs = read_logs(args.folder)
    text = build(logs, args.repo or repo_name(args.folder), args.lang or index_lang(args.folder, logs))
    target = args.folder / "README.md"
    problems = [f"{l['file']}: {p}" for l in logs for p in l["problems"]]
    problems += [f"{name}: ファイル名の先頭に 4 桁の番号がない（0001.md か 0001-<名前>.md）。索引に入らない"
                 for name in unindexed(args.folder)]
    problems += [f"{', '.join(files)}: 番号が重なっている。あとから入れた側のファイルの番号を、いちばん大きい番号の次に振り直す"
                 for files in duplicates(logs)]
    for p in problems:
        print(p)
    if args.check:
        current = target.read_text(encoding="utf-8") if target.exists() else ""
        stale = current != text
        if stale:
            print(f"{target} が古い。build_index.py {args.folder} で作り直す")
        if stale or problems:
            return 1
        print(f"{target} は最新で、{len(logs)} 件の Decision log に形の崩れはない")
        return 0
    target.write_text(text, encoding="utf-8")
    print(f"{target} を作り直した（{len(logs)} 件、形の崩れ {len(problems)} 件）")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
