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
// 他人が書けるような意地悪な行でも、時間が行の長さに比例する（正規表現の後戻りや、閉じていない記号の探し直しで固まらない）。
// CI の機械の速さはまちまちなので、決まった秒数ではなく、入力を 4 倍にしたときに時間が何倍になるかで確かめる
// （比例なら約 4 倍。2 乗なら 16 倍、3 乗なら 64 倍）。短すぎて測れないときは、十分に速いとみなす
const best = f => { let m = Infinity; for (let k = 0; k < 3; k++){ const t0 = process.hrtime.bigint(); f(); m = Math.min(m, Number(process.hrtime.bigint() - t0) / 1e6); } return m; };
for (const [name, make, n] of [
  ["閉じない強調", n => "**a ".repeat(n), 4999],
  ["閉じない括弧", n => "[".repeat(n), 19999],
  ["空白だけの表の線", n => "a|b\n" + " ".repeat(n) + "x", 19990],
  ["後ろが空白の見出し", n => "# a" + " ".repeat(n) + "x", 19990],
  ["閉じない下線", n => "_a ".repeat(n), 6000],
  ["閉じないバッククォート", n => "`a ".repeat(n) + "``b ".repeat(n / 6), 6000],
  ["長い行がたくさん", n => (("**a " + "x".repeat(16)).repeat(900) + "\n").repeat(n), 48],
]) {
  const small = best(() => mdView(make(Math.floor(n / 4)))), big = best(() => mdView(make(n)));
  if (big > 20 && big / Math.max(small, 1) > 10){ fail++; console.log(`SLOW ${name}: ${small.toFixed(1)} ms → ${big.toFixed(1)} ms for 4× the input`); }
  if (big > 10000){ fail++; console.log(`SLOW ${name}: ${big.toFixed(0)} ms`); }
}
check("見出しの後ろの #", "## Title ##", '<h4>"Title"</h4>');
check("後ろの # が語の一部なら残す", "# C#", '<h3>"C#"</h3>');
check("snake_case の中の _ は斜体にしない", "a_b_c and _em_", '<p>"a_b_c and "<em>"em"</em></p>');
if (fail) process.exit(1);
