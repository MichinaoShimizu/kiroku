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
// 正規表現は、どの行でも行の長さに比例する時間で終わる形だけにする（返事は他人が書いた文字を含みうる）
const MD_FENCE = /^ {0,3}(`{3,}|~{3,})/, MD_HEAD = /^ {0,3}(#{1,6})(?:[ \t]+(.*))?$/, MD_HR = /^ {0,3}([-*_])(?:[ \t]*\1){2,}[ \t]*$/;
const MD_ITEM = /^([ \t]*)([-*+]|\d{1,9}[.)])[ \t]+(.*)$/, MD_QUOTE = /^ {0,3}>[ \t]?(.*)$/;
const MD_LONG = 20000; // これより長い行は、書き方を読まずに文字のまま出す
function mdTableSep(l){ // 表の見出しの下の線（| --- | :-: |）
  const s = l.trim(); if (!s.includes("-") || s.length > MD_LONG) return false;
  return s.replace(/^\|/, "").replace(/\|$/, "").split("|").every(c => /^:?-+:?$/.test(c.trim()));
}
function mdHeadText(t){ // 見出しの後ろの # を除く（「# 題 ##」→「題」）
  t = (t || "").trimEnd(); let k = t.length;
  while (k > 0 && t[k - 1] === "#") k--;
  return k === 0 ? "" : k < t.length && (t[k - 1] === " " || t[k - 1] === "\t") ? t.slice(0, k).trimEnd() : t;
}
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
      const h = mdEl("h" + Math.min(6, m[1].length + 2)); mdInline(mdHeadText(m[2]), h); parent.append(h); i++; continue;
    }
    if (MD_HR.test(line)){ parent.append(mdEl("hr")); i++; continue; }
    if (line.includes("|") && i + 1 < lines.length && mdTableSep(lines[i + 1])){ // 表
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
// mdInline は 1 行の中の書き方（コード・強調・斜体・打ち消し・リンク）を要素にして parent に足す。
// 先頭から 1 度だけ読み、閉じる記号は indexOf で探す。閉じる記号がもうない記号は覚えておき、探し直さない
// （閉じていない記号がたくさんあっても、行の長さにほぼ比例する時間で終わるように）
function mdInline(text, parent, depth = 0){
  const s = String(text);
  if (s.length > MD_LONG || depth > 6){ parent.append(document.createTextNode(s)); return; }
  const none = new Set(); // 閉じる記号が、もう後ろにないもの
  let buf = "", i = 0;
  const flush = () => { if (buf){ parent.append(document.createTextNode(buf)); buf = ""; } };
  const closeAt = (d, from) => { if (none.has(d)) return -1; const k = s.indexOf(d, from); if (k < 0) none.add(d); return k; };
  const word = c => c !== undefined && /[\p{L}\p{N}_]/u.test(c);
  // validClose は、from より後ろで最初の、閉じる記号として使える d の位置（なければ -1）。読む位置 i は前にしか進まないので、
  // 記号ごとに「どこまで調べたか」と「見つけた位置」を覚え、同じところを調べ直さない
  const memo = {};
  const okClose = (d, k) => !/\s/.test(s[k - 1]) && !(d.length === 1 && (s[k + 1] === d || s[k - 1] === d)) && !((d === "_" || d === "__") && word(s[k + d.length]));
  const okTick = (d, k) => s[k + d.length] !== "`" && s[k - 1] !== "`"; // インラインコードは、ちょうど同じ数のバッククォートで閉じる
  const validClose = (d, from, ok = okClose, key = d) => {
    const m = memo[key] || (memo[key] = {k: -1, at: 0});
    if (m.k >= from) return m.k;
    if (m.k === -2) return -1;
    let k = Math.max(from, m.at);
    while ((k = s.indexOf(d, k)) >= 0 && !ok(d, k)) k += 1;
    m.at = k < 0 ? s.length : k + 1; m.k = k < 0 ? -2 : k;
    return k;
  };
  while (i < s.length){
    const c = s[i];
    if (c === "\\" && i + 1 < s.length && "\\`*_{}[]()#+-.!|~>".includes(s[i + 1])){ buf += s[i + 1]; i += 2; continue; }
    if (c === "`"){ // インラインコード: 同じ数のバッククォートで閉じる
      let n = 1; while (s[i + n] === "`") n++;
      const d = "`".repeat(n), k = validClose(d, i + n, okTick, "code" + n); // 調べた位置を覚え、閉じられない候補を探し直さない
      if (k < 0){ buf += d; i += n; continue; }
      flush(); const e = mdEl("code"); e.textContent = s.slice(i + n, k).replace(/^ (.*) $/, "$1"); parent.append(e); i = k + n; continue;
    }
    const pair = (d, tag) => { // 強調・打ち消しなど: 中身が空白で始まらず終わらないときだけ
      if (!s.startsWith(d, i) || /\s/.test(s[i + d.length] || " ")) return false;
      if (d === "_" || d === "__"){ if (word(s[i - 1])) return false; }
      const k = validClose(d, i + d.length + 1);
      if (k < 0) return false;
      flush(); const e = mdEl(tag); mdInline(s.slice(i + d.length, k), e, depth + 1); parent.append(e); i = k + d.length; return true;
    };
    if ((c === "*" || c === "_") && (pair(c + c, "strong") || pair(c, "em"))) continue;
    if (c === "~" && pair("~~", "del")) continue;
    if (c === "[" || (c === "!" && s[i + 1] === "[")){ // リンクと画像: 押せる形にせず、文字と URL を並べる（画像も読み込まない）
      const o = c === "!" ? i + 1 : i, rb = closeAt("]", o + 1);
      if (rb > o + 1 && s[rb + 1] === "("){
        const rp = s.indexOf(")", rb + 2);
        if (rp > 0){
          flush(); const a = mdEl("span", "mdlink"); mdInline(s.slice(o + 1, rb), a, depth + 1); parent.append(a);
          const url = s.slice(rb + 2, rp).trim().replace(/^<|>$/g, "").split(/\s+/)[0];
          if (url){ const u = mdEl("span", "mdurl"); u.textContent = " (" + url + ")"; parent.append(u); }
          i = rp + 1; continue;
        }
      }
    }
    buf += c; i++;
  }
  flush();
}
