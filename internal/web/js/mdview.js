// AI の返事（Markdown）を、読みやすく整えて見せる（kiroku serve --ai の「AI 分析」）。
/* 返事は履歴を読んだ AI が書いたもので、履歴の文字をそのまま含みうる。HTML としては決して入れず、
   要素を作って文字は textContent で入れる。画像は読み込まず（外へ通信しないように）、リンクは押せる形にせず
   文字と URL を並べる。対応するのは見出し・段落・箇条書き（入れ子・チェック）・番号付き・引用・コード・表・区切り線と、
   強調・斜体・打ち消し・インラインコード。ほかの書き方は文字のまま出す */
function mdView(src){
  const frag = document.createDocumentFragment();
  mdBlocks(String(src ?? "").replace(/\r\n?/g, "\n").split("\n"), frag, 0);
  return frag;
}
const MD_FENCE = /^ {0,3}(`{3,}|~{3,})/, MD_HEAD = /^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$/, MD_HR = /^ {0,3}([-*_])(\s*\1){2,}\s*$/;
const MD_ITEM = /^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$/, MD_QUOTE = /^ {0,3}>\s?(.*)$/, MD_TSEP = /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/;
function mdEl(tag, cls){ const e = document.createElement(tag); if (cls) e.className = cls; return e; }
// mdBlocks は lines を塊に分けて parent に足す。depth は入れ子の深さ（深すぎる入れ子で止まらないように）
function mdBlocks(lines, parent, depth){
  let i = 0;
  const startsBlock = l => MD_FENCE.test(l) || MD_HEAD.test(l) || MD_HR.test(l) || MD_ITEM.test(l) || MD_QUOTE.test(l);
  while (i < lines.length){
    const line = lines[i];
    if (!line.trim()){ i++; continue; }
    let m;
    if ((m = line.match(MD_FENCE))){ // コード
      const fence = m[1], body = [];
      for (i++; i < lines.length && !lines[i].trimStart().startsWith(fence); i++) body.push(lines[i]);
      i++;
      const pre = mdEl("pre"), code = mdEl("code"); code.textContent = body.join("\n"); pre.append(code); parent.append(pre);
      continue;
    }
    if ((m = line.match(MD_HEAD))){ // 見出し（ダイアログの見出しより下の段にする）
      const h = mdEl("h" + Math.min(6, m[1].length + 2)); mdInline(m[2], h); parent.append(h); i++; continue;
    }
    if (MD_HR.test(line)){ parent.append(mdEl("hr")); i++; continue; }
    if (line.includes("|") && i + 1 < lines.length && MD_TSEP.test(lines[i + 1]) && lines[i + 1].includes("-")){ // 表
      const cells = l => l.trim().replace(/^\|/, "").replace(/(?<!\\)\|$/, "").split(/(?<!\\)\|/).map(c => c.trim().replace(/\\\|/g, "|"));
      const wrap = mdEl("div", "mdtable"), table = mdEl("table"), thead = mdEl("thead"), tbody = mdEl("tbody"), hr = mdEl("tr");
      cells(line).forEach(c => { const th = mdEl("th"); mdInline(c, th); hr.append(th); });
      thead.append(hr);
      for (i += 2; i < lines.length && lines[i].includes("|") && lines[i].trim(); i++){
        const tr = mdEl("tr"); cells(lines[i]).forEach(c => { const td = mdEl("td"); mdInline(c, td); tr.append(td); }); tbody.append(tr);
      }
      table.append(thead, tbody); wrap.append(table); parent.append(wrap);
      continue;
    }
    if (MD_QUOTE.test(line)){ // 引用
      const body = [];
      for (; i < lines.length && (m = lines[i].match(MD_QUOTE)); i++) body.push(m[1]);
      const q = mdEl("blockquote");
      if (depth < 8) mdBlocks(body, q, depth + 1); else q.textContent = body.join("\n");
      parent.append(q);
      continue;
    }
    if ((m = line.match(MD_ITEM))){ // 箇条書き・番号付き（同じ字下げの項目を 1 つの並びに、深い字下げは中の並びに）
      const indent = m[1].length, ordered = /\d/.test(m[2]), list = mdEl(ordered ? "ol" : "ul");
      if (ordered && parseInt(m[2], 10) !== 1) list.start = parseInt(m[2], 10);
      while (i < lines.length){
        const im = lines[i].match(MD_ITEM);
        if (!im || im[1].length !== indent || /\d/.test(im[2]) !== ordered) break;
        const li = mdEl("li"), first = im[3], sub = [];
        for (i++; i < lines.length; i++){ // 続きの行（字下げした行・中の並び）
          const l = lines[i];
          if (!l.trim()){ if (i + 1 < lines.length && lines[i + 1].match(/^\s*/)[0].length > indent && lines[i + 1].trim()){ sub.push(""); continue; } break; }
          const lm = l.match(MD_ITEM);
          if (lm && lm[1].length <= indent) break;
          if (!lm && l.match(/^\s*/)[0].length <= indent && startsBlock(l)) break;
          sub.push(l.replace(new RegExp("^\\s{0," + (indent + 4) + "}"), ""));
        }
        const task = first.match(/^\[([ xX])\]\s+(.*)$/);
        const p = mdEl("span");
        if (task){ li.className = "task"; p.append(document.createTextNode(task[1] === " " ? "☐ " : "☑ ")); mdInline(task[2], p); }
        else mdInline(first, p);
        li.append(p);
        if (sub.length){ if (depth < 8) mdBlocks(sub, li, depth + 1); else li.append(document.createTextNode(" " + sub.join(" "))); }
        list.append(li);
      }
      parent.append(list);
      continue;
    }
    const para = []; // 段落（行の区切りはそのまま改行にする）
    for (; i < lines.length && lines[i].trim() && !(para.length && startsBlock(lines[i])); i++) para.push(lines[i].trim());
    const p = mdEl("p");
    para.forEach((l, k) => { if (k) p.append(mdEl("br")); mdInline(l, p); });
    parent.append(p);
  }
}
// mdInline は 1 行の中の書き方（コード・強調・斜体・打ち消し・リンク）を要素にして parent に足す
const MD_SPANS = [
  [/`+/, "code"],
  [/\*\*(?=\S)(.+?)(?<=\S)\*\*|__(?=\S)(.+?)(?<=\S)__/, "strong"],
  [/~~(?=\S)(.+?)(?<=\S)~~/, "del"],
  [/\*(?=[^\s*])(.+?)(?<=[^\s*])\*|(?<![\w\\])_(?=\S)(.+?)(?<=\S)_(?!\w)/, "em"],
  [/!?\[([^\]\n]+)\]\(\s*<?([^)\s>]*)>?(?:\s+"[^"]*")?\s*\)/, "link"],
];
function mdInline(text, parent, depth = 0){
  let s = String(text);
  while (s){
    let best = null;
    for (const [re, kind] of MD_SPANS){ const m = s.match(re); if (m && (!best || m.index < best.m.index)) best = {m, kind}; }
    if (!best || depth > 6){ parent.append(document.createTextNode(mdUnescape(s))); return; }
    const {m, kind} = best;
    if (m.index) parent.append(document.createTextNode(mdUnescape(s.slice(0, m.index))));
    let rest = s.slice(m.index + m[0].length);
    if (kind === "code"){ // 同じ数のバッククォートで閉じる。閉じていなければ文字のまま
      const close = rest.indexOf(m[0]);
      if (close < 0){ parent.append(document.createTextNode(m[0])); s = rest; continue; }
      const c = mdEl("code"); c.textContent = rest.slice(0, close).replace(/^ (.*) $/, "$1"); parent.append(c);
      s = rest.slice(close + m[0].length); continue;
    }
    if (kind === "link"){ // 押せる形にはせず、文字と URL を並べる（画像も読み込まない）
      const a = mdEl("span", "mdlink"); mdInline(m[1], a, depth + 1); parent.append(a);
      if (m[2] && m[2] !== m[1]){ const u = mdEl("span", "mdurl"); u.textContent = " (" + m[2] + ")"; parent.append(u); }
      s = rest; continue;
    }
    const e = mdEl(kind); mdInline(m[1] ?? m[2], e, depth + 1); parent.append(e);
    s = rest;
  }
}
const mdUnescape = s => s.replace(/\\([\\`*_{}[\]()#+\-.!|~>])/g, "$1");
