// mdView を、要素だけを持つ小さな document で動かし、できた木を文字にして確かめる。
const out = node => node.nodeType === 3 ? JSON.stringify(node.textContent)
  : node.tag === "#frag" ? node.children.map(out).join("")
  : `<${node.tag}${node.className ? "." + node.className : ""}${node.start ? " start=" + node.start : ""}>${node.children.map(out).join("")}</${node.tag}>`;
let fail = 0;
const check = (name, md, want) => { const got = out(mdView(md)); if (got !== want){ fail++; console.log("FAIL " + name + "\n  got:  " + got + "\n  want: " + want); } };
check("見出しと段落", "# Title\nline 1\nline 2", '<h3>"Title"</h3><p>"line 1"<br></br>"line 2"</p>');
check("HTML は文字のまま", '<img src=x onerror=alert(1)> <script>alert(1)</script>', '<p>"<img src=x onerror=alert(1)> <script>alert(1)</script>"</p>');
check("リンクと画像は押せる形にしない", "[a](https://e.x/?q=1) ![i](https://e.x/i.png)", '<p><span.mdlink>"a"</span><span.mdurl>" (https://e.x/?q=1)"</span>" "<span.mdlink>"i"</span><span.mdurl>" (https://e.x/i.png)"</span></p>');
check("javascript: の URL も文字", "[x](javascript:alert(1))", '<p><span.mdlink>"x"</span><span.mdurl>" (javascript:alert(1)"</span>")"</p>');
check("強調・コード", "**b** *i* `c<d>` ~~s~~ snake_case_name", '<p><strong>"b"</strong>" "<em>"i"</em>" "<code>"c<d>"</code>" "<del>"s"</del>" snake_case_name"</p>');
check("入れ子の箇条書き", "- a\n  - b\n- [x] c", '<ul><li><span>"a"</span><ul><li><span>"b"</span></li></ul></li><li.task><span>"☑ ""c"</span></li></ul>');
check("番号付き", "3. a\n4. b", '<ol start=3><li><span>"a"</span></li><li><span>"b"</span></li></ol>');
check("表", "| A | B |\n|---|:-:|\n| 1 | **2** |", '<div.mdtable><table><thead><tr><th>"A"</th><th>"B"</th></tr></thead><tbody><tr><td>"1"</td><td><strong>"2"</strong></td></tr></tbody></table></div>');
check("コード", "```html\n<b>x</b>\n```", '<pre><code>"<b>x</b>"</code></pre>');
check("引用・区切り線", "> q\n\n---", '<blockquote><p>"q"</p></blockquote><hr></hr>');
check("深い入れ子でも止まらない", ">".repeat(50) + " x", out(mdView(">".repeat(50) + " x")));
// 他人が書けるような意地悪な行でも、時間がかからない（正規表現の後戻りや、閉じていない記号の探し直しで固まらない）
for (const [name, md] of [
  ["閉じない強調", "**a ".repeat(4999)],
  ["閉じない括弧", "[".repeat(19999)],
  ["空白だけの表の線", "a|b\n" + " ".repeat(19990) + "x"],
  ["後ろが空白の見出し", "# a" + " ".repeat(19990) + "x"],
  ["閉じない下線", "_a ".repeat(6000)],
  ["閉じないバッククォート", "`a ".repeat(6000) + "``b " .repeat(1000)],
  ["長い行がたくさん", (("**a " + "x".repeat(16)).repeat(900) + "\n").repeat(50)],
  ["1 MB の行", "*a ".repeat(350000)],
]) {
  const t0 = Date.now(); mdView(md); const ms = Date.now() - t0;
  if (ms > 1500){ fail++; console.log("SLOW " + name + ": " + ms + " ms"); }
}
check("見出しの後ろの #", "## Title ##", '<h4>"Title"</h4>');
check("後ろの # が語の一部なら残す", "# C#", '<h3>"C#"</h3>');
check("snake_case の中の _ は斜体にしない", "a_b_c and _em_", '<p>"a_b_c and "<em>"em"</em></p>');
if (fail) process.exit(1);
