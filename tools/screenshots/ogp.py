"""デモページ（GitHub Pages）の HTML に、SNS のリンクカード用の情報（OGP）を入れる。

  python3 ogp.py <index.html> <公開する URL（末尾は /）>

利用者が手元で作る HTML には入れない（デモの URL や画像を指すため）。画像は docs/og.png を同じ場所に置く。
"""
import html, sys

path, base = sys.argv[1], sys.argv[2]
title = "kiroku — Your AI work history, visualized"
desc = ("Claude Code, Kiro, Amazon Q and Codex history as a calendar: what you asked, what it cost, "
        "and what turned into commits. Local-only, one HTML file. Live demo with dummy data.")
tags = [
    ("name", "description", desc),
    ("property", "og:type", "website"),
    ("property", "og:site_name", "kiroku"),
    ("property", "og:title", title),
    ("property", "og:description", desc),
    ("property", "og:url", base),
    ("property", "og:image", base + "og.png"),
    ("property", "og:image:width", "2880"),
    ("property", "og:image:height", "1508"),
    ("property", "og:image:alt", "kiroku's week calendar: sessions as bars with commits, pushes and pull requests beside them"),
    ("name", "twitter:card", "summary_large_image"),
    ("name", "twitter:title", title),
    ("name", "twitter:description", desc),
    ("name", "twitter:image", base + "og.png"),
]
meta = "".join(f'\n<meta {k}="{v}" content="{html.escape(c, quote=True)}">' for k, v, c in tags)
s = open(path, encoding="utf-8").read()
mark = '<meta charset="utf-8">'
if s.count(mark) < 1:
    sys.exit(f"{path} に {mark} がない")
s = s.replace(mark, mark + meta, 1)
s = s.replace("<title>kiroku</title>", f"<title>{html.escape(title)}</title>", 1)
open(path, "w", encoding="utf-8").write(s)
print(f"{path} に OGP を入れました")
