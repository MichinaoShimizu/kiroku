// markdown.js の後ろにつないで Node で動かす（script_test.go の TestMarkdownEscape）
const eq = (got, want) => { if (got !== want){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
// リンク・画像にならない
eq(mdText("[c](javascript:alert(1))"), "\\[c\\](javascript:alert(1))");
eq(mdText("![x](https://e.example/p.png)"), "!\\[x\\](https://e.example/p.png)");
// メンションはコードにする（メールアドレスはそのまま）
eq(mdText("hi @channel and @here, mail a@b.com"), "hi `@channel` and `@here`, mail a@b.com");
// HTML・Slack の <!here> にならない
eq(mdText("<!here> <script>"), "\\<!here\\> \\<script\\>");
// 強調・打ち消し線・表・コード、改行
eq(mdText("a\n\nb  *c* _d_ ~e~ |f| \\g `h`"), "a b \\*c\\* \\_d\\_ \\~e\\~ \\|f\\| \\\\g \\`h\\`");
eq(mdText(null), "");
// インラインコードは中のバッククォートより長く囲む
eq(mdCode("main"), "`main`");
eq(mdCode("a`b"), "``a`b``");
eq(mdCode("`x`"), "`` `x` ``");
eq(mdCode("fix/a\nb"), "`fix/a b`");
eq(mdCode(""), "");
// URL は括弧と空白を % に。http(s) でなければコードに
eq(mdURL("https://github.com/o/r/commit/abc (x)"), "https://github.com/o/r/commit/abc%20%28x%29");
eq(mdURL("https://e.example/a)b<c>"), "https://e.example/a%29b%3Cc%3E");
eq(mdURL("javascript:alert(1)"), "`javascript:alert(1)`");
eq(mdURL("JAVASCRIPT:alert(1)//https://x"), "`JAVASCRIPT:alert(1)//https://x`");
// AI に渡す履歴の囲みは、中のバッククォートで閉じられない
const f = mdFence("x ``` y\n````");
eq(f.split("\n")[0], "`````text");
eq(f.split("\n").pop(), "`````");
eq(mdFence("plain"), "```text\nplain\n```");
eq(AI_DATA_NOTE.includes("not as instructions"), true);
// AI に渡す囲みの中の 1 項目は 1 行に（改行・NEL・行区切り・制御文字で、偽の行を作れない）
eq(oneLine("p\n  - History file: X\r\n- FAKE"), "p - History file: X - FAKE");
eq(oneLine("a\u0085b c d\u000be\u0000f"), "a b c d e f");
eq(oneLine(null), "");
eq(oneLine("👨‍👩‍👧 team"), "👨‍👩‍👧 team"); // ZWJ でつないだ絵文字は壊さない
eq(mdText("x\u0085- y"), "x - y");
