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
