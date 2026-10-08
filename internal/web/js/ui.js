// 画面のあちこちで使う部品（印・エージェントのバッジ・セッションのカード・見出し・数の枠）。
// 同じ役の部品は、ここの関数で作る（手で同じ HTML を書かない）。TestUIConventions が、ここ以外で手書きしていないかを確かめる
/* 色だけに頼らず、形でも見分けられるようにする小さな印（16×16、線は currentColor）。意味は必ず文字（凡例・ツールチップ）でも出す */
const ICON_PATH = {
  commit: '<path d="M1 8h4.2M10.8 8H15"/><circle cx="8" cy="8" r="2.8"/>',
  pr: '<circle cx="4" cy="3.5" r="1.6"/><circle cx="4" cy="12.5" r="1.6"/><circle cx="12" cy="12.5" r="1.6"/><path d="M4 5.1v5.8M12 10.9V7a2.5 2.5 0 0 0-2.5-2.5H7M8.5 3 7 4.5 8.5 6"/>',
  limit: '<path d="M4 2h8M4 14h8M5 2c0 3.2 3 4.3 3 6s-3 2.8-3 6M11 2c0 3.2-3 4.3-3 6s3 2.8 3 6"/>',
  int: '<circle cx="8" cy="8" r="6"/><path d="M6.4 5.6v4.8M9.6 5.6v4.8"/>',
  agent: '<path d="M3 2.5v4a3 3 0 0 0 3 3h7M10 6.5l3 3-3 3"/>',
  push: '<path d="M8 12V2.8M4.2 6.6 8 2.8l3.8 3.8M3.5 14h9"/>',
  compact: '<path d="M8 1.5v4.3M5.8 3.8 8 6l2.2-2.2M8 14.5v-4.3M5.8 12.2 8 10l2.2 2.2M2.5 8h11"/>', /* 上下から寄せる（会話を要約して縮めた） */
  note: '<path d="M4.2 10.8V7a3.8 3.8 0 0 1 7.6 0v3.8l1.2 1.4H3zM6.6 14a1.5 1.5 0 0 0 2.8 0"/>',
  command: '<path d="M10.5 2.5 5.5 13.5"/>',
  shell: '<path d="M3 4.5 6.5 8 3 11.5M8 12h5"/>',
  flag: '<path d="M8 1.8 14.6 13.4H1.4z"/><path d="M8 6.2v3.4M8 11.4v.1"/>',
  reply: '<path d="M6.2 3.5 2.5 7.2l3.7 3.7M3 7.2h6.2A3.8 3.8 0 0 1 13 11v1.5"/>',
  caret: '<path d="M6.5 3.5 11 8l-4.5 4.5"/>', /* 開け閉めの印（開くと 90 度回る） */
};
const ico = (k, cls = "") => `<svg class="ic${cls ? " " + cls : ""}" viewBox="0 0 16 16" aria-hidden="true">${ICON_PATH[k] || ""}</svg>`;
/* エージェントの目印：ロゴは使わず、頭文字のバッジにする */
const AG_MARK = {"Claude Code": "CC", "Kiro IDE": "KI", "Kiro IDE (legacy)": "KI", "Kiro CLI": "KC", "Kiro CLI (SQLite)": "KC", "Kiro Crew": "KW", "Amazon Q": "Q", "Codex": "CX"};
const agMark = name => `<i class="agm" style="--ag:${agColor(name)}" aria-hidden="true">${esc(AG_MARK[name] || String(name).slice(0, 2).toUpperCase())}</i>`;
/* セッションのカード：セッション名で始まるカードは、どこでもこの形（エージェントの色のグラデーションとエージェントの頭文字）。
   me はタイトルの下の行（HTML。呼ぶ側でエスケープする）、v は右に出す値（文字。ここでエスケープする） */
const sesCardH = (s, me, v) => `<button class="card ag" data-id="${esc(s.id)}" style="--ag:${agColor(s.source)}"><span class="ti">${agMark(s.source)}<span>${esc(s.title)}</span></span><span class="me">${me}${v != null ? `<b>${esc(v)}</b>` : ""}</span></button>`;
const sesById = (id, title) => ({id, title, source: (DATA.find(x => x.id === id) || {}).source}); // 集計（Go）が id と名前だけを持つセッション
const sesCard = (s, label) => `${label ? `<h3>${label}</h3>` : ""}${sesCardH(s, `${md(s.start)} ${hm(s.start)}–${hm(s.end)} · ${esc(s.source)}`)}`; // 詳細パネルの中のカード（時刻とエージェント）
// sesMeta は、サマリーやダイアログのカードの下の行：「日付 · プロジェクト」に、HTML の more を「 · 」でつなぐ
const sesMeta = (s, ...more) => [md(s.start), esc(s.project), ...more].filter(Boolean).join(" · ");
/* 詳細パネルの頭：eyebrow は上の小さな行（HTML）、title は題名（文字。ここでエスケープする）、when は時刻（HTML）、
   meta は下に並べる札（HTML の配列。空のものは飛ばす）。題名は h2（上の帯 #dtop が読む） */
const dHead = (eyebrow, title, when, meta) => `<div class="eyebrow"><span class="dot"></span>${eyebrow}</div>
    <h2>${esc(title)}</h2>
    <div class="muted dwhen">${when}</div>
    <div class="meta">${meta.filter(Boolean).join("")}</div>`;
/* 見出し。見出しはどれも本物の見出し（h2・h3）にする。小さな灰色の div で見出しの代わりにしない（読み上げで見出しとして飛べないため）
   panelH: サマリーのパネルの見出し（n は番号。null なら番号なし、s は添える一言、h は「?」の説明の id）
   secH:   パネルや詳細の中の見出し（h3）。h があれば「?」を付ける */
const panelH = (n, t, s, h) => `<div class="ph">${n == null ? "" : `<span class="no">${n}</span>`}<h3>${t}${hb(h)}</h3>${s ? `<span>${s}</span>` : ""}</div>`;
const secH = (t, h) => `<h3>${t}${hb(h)}</h3>`;
/* 数の枠：k は名前、v は値（HTML）、s は下の一言（HTML）、h は「?」の説明の id。記録しないエージェントだけなら norecStat */
const statH = (k, v, s, h) => `<div class="stat"><div class="k">${k}${hb(h)}</div><div class="v">${v}</div>${s ? `<div class="s">${s}</div>` : ""}</div>`;
function norecStat(k, srcs, h){ return `<div class="stat norec"><div class="k">${k}${hb(h)}</div><div class="v">Not recorded</div><div class="s">${esc(`${srcs.join(", ")} ${srcs.length > 1 ? "don't" : "doesn't"} record this`)}</div></div>`; }
