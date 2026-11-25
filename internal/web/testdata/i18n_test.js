// i18n.js の JA_TEXT を Node で確かめる（script_test.go の TestI18nCatalog）。KEYS は Go が js/ から集めた t("…") の鍵
const bad = [];
for (const [k, v] of Object.entries(JA_TEXT)){
  if (typeof v !== "string" || !v.trim()) bad.push(`empty: ${k}`);
  if (/[<>]/.test(v)) bad.push(`markup in value: ${k}`); // t() は文字だけを返す。HTML は呼ぶ側のテンプレートに
  const ph = s => (s.match(/\{\w+\}/g) || []).sort().join(",");
  if (ph(k) !== ph(v)) bad.push(`placeholders differ: ${k} → ${v}`);
}
// t() の働き：日本語では辞書を引き、{name} を埋める。英語と、辞書にない文はそのまま
const one = Object.keys(JA_TEXT)[0];
if (one && t(one) !== JA_TEXT[one]) bad.push("t() doesn't look up the catalog");
if (t("no such key {x}", {x: "<b>"}) !== "no such key <b>") bad.push("t() fallback or vars");
if (bad.length){ console.error(bad.join("\n")); process.exitCode = 1; }
const missing = KEYS.filter(k => !(k in JA_TEXT));
if (missing.length) console.log(`MISSING ${missing.length}\n` + missing.join("\n"));
// 記録が足りないときの一言は、週と月で名詞と数え方を分ける（「過去のか月が 4 要ります」としない）
{
  const want = [["To compare with your usual range, kiroku needs {min} past months with records (have {n}).", "いつもの範囲と比べるには、記録のある過去の月が 4 か月分要ります（いまは 2 か月分）。"],
    ["To compare with your usual range, kiroku needs {min} past weeks with records (have {n}).", "いつもの範囲と比べるには、記録のある過去の週が 4 週分要ります（いまは 2 週分）。"]];
  for (const [k, v] of want) if (t(k, {min: 4, n: 2}) !== v){ console.error(`${k} → ${t(k, {min: 4, n: 2})} (want ${v})`); process.exitCode = 1; }
}
// 用語をそろえる（2026-11-25 のメンテナーの決定）：by hand は「手動」、pull request は「PR」、so far は「ここまで」、
// range は「範囲」、ばらつき（月末の推定の幅のもとになる、終わった日の値のばらつき）と四分位範囲（spread：上位25%点 − 下位25%点）は分ける。古い言い方（手作業・手で・プルリクエスト・これまで・（途中）・幅・（コミットか PR）まで届いた。「至った」にそろえる）を値に戻さない。
// 期間の日数のうち今日までの日（「12 / 14 日（今日まで）」・経過日数）は「ここまで」ではなく「今日まで」。合計の「ここまで」とは分ける。
// 「途中」は進行中（途中の週）と「途中から記録が始まる」（partial）の意味でだけ使う
for (const [k, v] of Object.entries(JA_TEXT)) for (const w of ["手作業", "手で", "プルリクエスト", "これまで", "（途中）", "幅", "まで届い"])
  if (v.includes(w)){ console.error(`old term ${w} in: ${k} → ${v}`); process.exitCode = 1; }
{
  const want = {"So far": "ここまで", " (so far)": "（ここまで）", "of {n} so far": "/ {n}（今日まで）", "Days so far": "経過日数", "Range": "範囲", "spread": "四分位範囲", "Pull requests": "PR", "By hand (or another tool)": "手動（またはほかのツール）"};
  // 「ばらつき」は月末の推定の説明だけ。四分位範囲（spread）には使わない（1 つの言葉に 1 つの意味）
  for (const [k, v] of Object.entries(JA_TEXT)) if (v.includes("ばらつき") && !/varied/.test(k)){ console.error(`ばらつき is for the projection's variation only: ${k} → ${v}`); process.exitCode = 1; }
  for (const k of ["{days} of the {n} so far (this month is still in progress)", "{days} of the {n} so far (this week is still in progress)"])
    if (!JA_TEXT[k].startsWith("今日までの")){ console.error(`days until today should say 今日まで: ${k} → ${JA_TEXT[k]}`); process.exitCode = 1; }
  for (const [k, v] of Object.entries(want)) if (JA_TEXT[k] !== v){ console.error(`${k} → ${JA_TEXT[k]} (want ${v})`); process.exitCode = 1; }
}
// 見出し・図の名前は「推定費用」（Estimated cost）。「費用」（Cost）はスイッチだけ（ガイドの決まり）
for (const k of ["Estimated cost and output over time", "Claude Code estimated cost per AI commit", "Sessions with the highest estimated cost per commit", "Sessions with the highest estimated cost per prompt", "Share of estimated cost with no commit or PR",
  "Estimated cost per AI commit", "Estimated cost per session hour", "Estimated cost per 100 AI-edited lines"]) // 単価のグラフの縦軸の名前も
  if (!(k in JA_TEXT) || !JA_TEXT[k].includes("推定費用")){ console.error(`heading should say 推定費用: ${k} → ${JA_TEXT[k]}`); process.exitCode = 1; }
