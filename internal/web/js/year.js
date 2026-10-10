// 1 年の露光（ふりかえり）
/* ── 1 年の露光（ふりかえり） ──
   1 年ぶんの作業していた区間を、横に日付・縦に時刻（朝 6 時から翌朝 6 時）の光の筋として描く。同時に動いていたほど明るく写る。
   シェア用の 1 枚もこのブラウザの中で描くだけで、どこにも送らない。画像に載せるのは集計した数字と光の筋だけで、
   プロンプト・プロジェクト名・ブランチ・ファイル・目安コストは載せない */
const PLATE = ["#3a9be0","#f07a2b","#1fbf8f","#e08fbd","#f2b53a","#7cc6f0","#9c84e6","#c9b51c"], PLATE_OTHER = "#6b7180";
const FONT = { mincho: '"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Hiragino Mincho ProN","Noto Serif JP",serif',
  sans: '"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Hiragino Sans","Noto Sans JP",system-ui,sans-serif',
  mono: 'ui-monospace,"SFMono-Regular","JetBrains Mono",Menlo,Consolas,monospace' };
const yr = { y: null, x: null, L: null, G: null, opt: Object.assign({out: true, type: true, agents: true}, store.get("yrOpt", {})) };
function yearsOf(){ return [...new Set(Object.keys(MONTHS).map(k => +k.slice(0, 4)))].sort((a, b) => a - b); }
function yearData(y){
  const ms = Object.keys(MONTHS).filter(k => k.startsWith(y + "-")).sort().map(k => MONTHS[k]);
  const sum = f => ms.reduce((a, m) => a + (f(m) || 0), 0);
  const active = sum(m => m.active), fixP = sum(m => m.fixRate == null ? 0 : m.prompts);
  const by = {}; ms.forEach(m => ((m.shares || {}).source || []).forEach(s => by[s.key] = (by[s.key] || 0) + s.minutes));
  const agents = Object.keys(by).filter(k => by[k] > 0).sort((a, b) => by[b] - by[a]).map(k => ({k, min: by[k], c: PLATE[agIdx(k)] || PLATE_OTHER})); // 画面と同じエージェントの色
  const aMin = agents.reduce((a, g) => a + g.min, 0); agents.forEach(g => g.pct = aMin ? g.min / aMin * 100 : 0);
  const color = Object.fromEntries(agents.map(g => [g.k, g.c]));
  // 作業した日
  const on = new Set(); ms.forEach(m => (m.days || []).forEach((d, i) => { if (d.active > 0){ const [Y, M, D] = m.start.split("-").map(Number); on.add(key(new Date(Y, M-1, D+i))); } }));
  // 光の筋：[列（日）, 上端, 下端（列の中の 0〜1）, 色]。列は朝 6 時で区切るので、深夜の作業は前の日の列の下のほうに写る
  const y0 = new Date(y, 0, 1)/1000, y1 = new Date(y+1, 0, 1)/1000, nd = Math.round((y1 - y0)/86400), day0 = new Date(y, 0, 1);
  const lines = [], hist = new Array(24).fill(0), slots = new Set(); let segMin = 0, nses = 0, nprompts = 0; // slots: 動いていた 5 分枠（並列を二重に数えない）
  DATA.forEach(s => {
    const mine = s.start >= y0 && s.start < y1; if (mine){ nses++; nprompts += s.nPrompts || 0; }
    s.segs.forEach(([a, b]) => {
      if (mine) segMin += (b - a)/60;
      a = Math.max(a, y0); b = Math.min(b, y1); if (b <= a) return;
      for (let i = Math.floor(a/300), j = Math.ceil(b/300); i < j; i++) slots.add(i);
      while (a < b){
        const t = new Date(a*1000), d = new Date(t.getFullYear(), t.getMonth(), t.getDate() - (t.getHours() < 6 ? 1 : 0)), c0 = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 6)/1000, c1 = new Date(d.getFullYear(), d.getMonth(), d.getDate()+1, 6)/1000, e = Math.min(b, c1);
        const col = Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
        if (col >= 0 && col < nd) lines.push([col, (a - c0)/(c1 - c0), (e - c0)/(c1 - c0), color[s.source] || PLATE_OTHER]);
        a = e;
      }
    });
  });
  slots.forEach(i => hist[new Date(i*300000).getHours()] += 5);
  const ht = hist.reduce((a, v) => a + v, 0) || 1, now = today0();
  return { y, nd, active, lines, agents, sessions: nses,
    morningPct: (hist[5] + hist[6] + hist[7] + hist[8])/ht*100, peak: hist.indexOf(Math.max(...hist)),
    fix: fixP ? sum(m => m.fixRate == null ? 0 : m.fixRate * m.prompts)/fixP : null,
    avgMin: nses ? segMin/nses : 0, perSes: nses ? nprompts/nses : 0,
    commits: sum(m => m.git && m.git.commits), prs: sum(m => m.outputs && m.outputs.prs),
    days: on.size, prompts: nprompts,
    partial: now.getFullYear() === y ? now : null };
}
/* 光の名前：使い方の傾向を写真の言葉で表す（良し悪しではない）。上から順に、最初に当てはまったもの */
const LIGHTS = [
  {k: "dawn", en: "Daybreak", m: "morning", t: x => x.morningPct >= 25,
   de: "You get things done while the morning is fresh.",
   re: "25% or more of active time is between 5:00 and 9:00"},
  {k: "multi", en: "Multiple Exposure", m: "agents", t: x => x.agents.filter(g => g.pct >= 10).length >= 3,
   de: "You pick a partner for each job and layer them.",
   re: "3 or more agents, each with 10% or more of active time"},
  {k: "long", en: "Long Exposure", m: "avg", t: x => x.avgMin >= 45 && x.perSes <= 10,
   de: "You hand over big tasks and let them run.",
   re: "45 minutes or more of active time per session, with 10 prompts or fewer on average"},
  {k: "burst", en: "Burst", m: "per", t: x => x.perSes >= 15,
   de: "You ask small and often, and move fast.",
   re: "15 or more prompts per session on average"},
  {k: "focus", en: "Bracketing", m: "fix", t: x => x.fix != null && x.fix >= 15,
   de: "You try a few takes and keep the best one.",
   re: "15% or more of prompts look like a follow-up correction or came after an interruption (guessed from the wording)"},
  {k: "day", en: "Daylight", m: "peak", t: () => true,
   de: "You work with AI steadily through the day.",
   re: "None of the above"}];
function lightMetric(k, x){
  return ({ morning: ["Work between 5:00 and 9:00", `${Math.round(x.morningPct)}%`],
    agents: ["Agents with 10%+ of time", String(x.agents.filter(g => g.pct >= 10).length)],
    avg: ["Active time per session", dur(x.avgMin)],
    per: ["Prompts per session", x.perSes.toFixed(1)],
    fix: ["Prompts with corrections", x.fix == null ? "—" : `${Math.round(x.fix)}%`],
    peak: ["Busiest hour", `${x.peak}:00`] })[k];
}
function seeded(a){ return () => (a = (a * 16807) % 2147483647) / 2147483647; }
function exposure(g, X, Y, W, H, x, p, grid, c0 = 0, c1 = x.nd){ // 列 [c0, c1) だけを描く
  g.save(); g.beginPath(); g.rect(X, Y, W, H); g.clip();
  g.fillStyle = "#06070a"; g.fillRect(X, Y, W, H);
  const r = seeded(7); g.fillStyle = "#ffffff";
  for (let i = 0; i < W*H/6000; i++){ g.globalAlpha = r()*.18; g.fillRect(X + r()*W, Y + r()*H, p, p); }
  g.globalAlpha = 1;
  if (grid){ g.strokeStyle = "rgba(255,255,255,.07)"; g.lineWidth = p; [.25, .5, .75].forEach(t => { g.beginPath(); g.moveTo(X, Y + t*H); g.lineTo(X + W, Y + t*H); g.stroke(); }); }
  g.globalCompositeOperation = "lighter"; g.lineCap = "round";
  const cw = W/(c1 - c0), lw = Math.min(3.5*p, Math.max(p*.9, cw*.7)), glow = Math.min(lw*6, Math.max(lw*2.2, cw*.9));
  x.lines.forEach(([c, a, b, col]) => {
    if (c < c0 || c >= c1) return;
    const px = X + (c - c0 + .5)*cw, y0 = Y + a*H, y1 = Math.max(y0 + lw*.5, Y + b*H);
    g.strokeStyle = col;
    g.globalAlpha = .1; g.lineWidth = glow; g.beginPath(); g.moveTo(px, y0); g.lineTo(px, y1); g.stroke();
    g.globalAlpha = .75; g.lineWidth = lw*.8; g.beginPath(); g.moveTo(px, y0); g.lineTo(px, y1); g.stroke();
  });
  g.restore();
}
function drawPlate(){
  const cv = $("#yrplate"); if (!cv || !yr.x) return;
  const r = cv.getBoundingClientRect(), p = Math.min(2, devicePixelRatio || 1), W = Math.round(r.width*p), H = Math.round(r.height*p);
  if (!W || !H) return; cv.width = W; cv.height = H; exposure(cv.getContext("2d"), 0, 0, W, H, yr.x, p, true);
}
function drawCard(){
  const cv = $("#yrcard"); if (!cv || !yr.x) return;
  const x = yr.x, L = yr.L, o = yr.opt, W = 1600, H = 900, g = cv.getContext("2d");
  const ink = "#ece8df", ink2 = "#a9a69e", amber = "#ffae57", top = 430;
  const ls = v => { if ("letterSpacing" in g) g.letterSpacing = v; };
  g.clearRect(0, 0, W, H); g.fillStyle = "#06070a"; g.fillRect(0, 0, W, H);
  // 使い始めたばかりでも光が端に寄らないよう、画像は記録のある日から（短ければ 4 週ぶん）だけを写す
  const last = x.partial ? Math.round((new Date(x.partial.getFullYear(), x.partial.getMonth(), x.partial.getDate()) - new Date(x.y, 0, 1))/864e5) + 1 : x.nd;
  const first = x.lines.reduce((a, l) => Math.min(a, l[0]), last), c0 = Math.max(0, Math.min(first, last - 28));
  exposure(g, 0, top, W, H - top, x, 2, false, c0, last);
  const fade = g.createLinearGradient(0, top, 0, H);
  fade.addColorStop(0, "rgba(6,7,10,1)"); fade.addColorStop(.3, "rgba(6,7,10,0)"); fade.addColorStop(.72, "rgba(6,7,10,0)"); fade.addColorStop(1, "rgba(6,7,10,.92)");
  g.fillStyle = fade; g.fillRect(0, top, W, H - top);
  // 画像だけを見た人にも読めるよう、光の読み方を添える
  g.textAlign = "left"; g.textBaseline = "alphabetic"; g.fillStyle = ink2; g.globalAlpha = .8; g.font = `500 16px ${FONT.mono}`; ls("2px");
  g.fillText("EACH STREAK = A STRETCH OF ACTIVE TIME   ·   ACROSS: DATE   ·   DOWN: 6:00 → 6:00", 80, top + 40); ls("0px"); g.globalAlpha = 1;
  g.textAlign = "left"; g.textBaseline = "alphabetic";
  const pt = x.partial;
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; ls("4px");
  const d0 = new Date(x.y, 0, 1 + c0), d1 = new Date(x.y, 0, last);
  g.fillText(c0 || pt ? `KIROKU — ${dSpan(d0, d1, true).toUpperCase()}` : `KIROKU — ${x.y} EXPOSURE`, 80, 110); ls("0px");
  const hrs = x.active/60, hs = commas(hrs >= 10 ? Math.round(hrs) : Math.round(hrs*10)/10);
  g.fillStyle = ink; g.font = `800 184px ${FONT.mincho}`; g.fillText(hs, 72, 300);
  const hw = g.measureText(hs).width;
  g.font = `600 54px ${FONT.mincho}`; g.fillText("hours", 72 + hw + 16, 300);
  g.fillStyle = ink2; g.font = `500 30px ${FONT.sans}`;
  g.fillText(pt ? "This year's light, with AI." : "A year of light, with AI.", 80, 360);
  const items = [[x.sessions, "sessions"]];
  if (o.out){ if (x.commits) items.push([x.commits, "commits"]); if (x.prs) items.push([x.prs, "PR" + (("s"))]); }
  let cx = 80;
  items.forEach(([n, l]) => { g.font = `500 26px ${FONT.mono}`; g.fillStyle = ink; const t = commas(n); g.fillText(t, cx, 412); cx += g.measureText(t).width + 10;
    g.font = `500 24px ${FONT.sans}`; g.fillStyle = ink2; g.fillText(l, cx, 412); cx += g.measureText(l).width + 40; });
  if (o.type && L){
    g.textAlign = "right"; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("YOUR LIGHT", W - 80, 110); ls("0px");
    g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36; g.fillStyle = amber; g.font = `800 64px ${FONT.mincho}`; g.fillText(L.en, W - 80, 190);
    g.shadowBlur = 0; g.textAlign = "left"; g.textBaseline = "alphabetic";
  }
  if (o.agents){
    let lx = 80; g.font = `500 24px ${FONT.sans}`;
    x.agents.slice(0, 4).forEach(a => { g.fillStyle = a.c; g.shadowColor = a.c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8, 842, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0;
      const t = `${(a.k)} ${Math.round(a.pct)}%`; g.fillStyle = ink; g.fillText(t, lx + 26, 850); lx += 26 + g.measureText(t).width + 36; });
  }
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; g.textAlign = "right"; ls("2px"); g.fillText("kiroku", W - 80, 850); ls("0px"); g.textAlign = "left";
}
function renderYear(){
  const ys = yearsOf(), dlg = $("#yr"); if (!ys.length) return;
  if (!ys.includes(yr.y)) yr.y = ys.includes(today0().getFullYear()) ? today0().getFullYear() : ys[ys.length-1];
  const x = yr.x = yearData(yr.y), L = yr.L = LIGHTS.find(l => l.t(x)), pt = x.partial;
  const pos = d => (Math.round((d - new Date(x.y, 0, 1))/864e5)/x.nd*100).toFixed(2);
  const months = Array.from({length: 12}, (_, i) => `<span style="left:${pos(new Date(x.y, i, 1))}%">${MON[i]}</span>`).join("");
  const hours = [["6", 0], ["12", 25], ["18", 50], ["0", 75], ["6", 100]].map(([h, t]) => `<span style="top:${t}%">${`${h}:00`}</span>`).join("");
  const ev = [L.m, ...["morning", "avg", "per"].filter(k => k !== L.m)].slice(0, 3).map(k => lightMetric(k, x));
  const vt = t => esc(t);
  const name = l => l.en, opt = (k, l) => `<label><input type="checkbox" data-o="${k}"${yr.opt[k] ? " checked" : ""}>${l}</label>`;
  dlg.innerHTML = `<div class="yrhd"><div><div class="eyebrow">Year in review</div><h2 id="yrh">${`${x.y} exposure`}</h2></div>
    <label><span class="sr">Year</span><select id="yrsel">${ys.map(v => `<option value="${v}"${v === x.y ? " selected" : ""}>${v}</option>`).join("")}</select></label>
    <form method="dialog"><button class="iconbtn" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form></div>
  <p class="yrlead">Your active time with AI, drawn like a year-long exposure. Across is the date, down is the time of day (6:00 to 6:00 the next morning). Each stretch of active time is a streak of light, brighter where sessions overlapped.${pt ? ` Recorded up to ${dMD(pt)}.` : ""}</p>
  <div class="yrscroll"><div class="yrchart"><div class="yrax" aria-hidden="true">${hours}</div><canvas class="yrplate" id="yrplate" role="img" aria-label="${esc(`Active time in ${x.y} by date and time of day. Active time ${dur(x.active)}`)}"></canvas><div class="yrx" aria-hidden="true">${months}</div></div></div>
  <div class="yrleg">${x.agents.map(a => `<span><i style="background:${a.c}"></i>${esc((a.k))} ${Math.round(a.pct)}%</span>`).join("")}</div>
  <div class="yrhow"><div><b>Length = how long a session was active</b>Each streak is one stretch of active time in a session, estimated from the timestamps in the history. It includes time the AI ran on its own.</div>
    <div><b>Color = agent</b>Colors that start to mix show when you began using more than one.</div>
    <div><b>Dark bands = time off</b>Holidays and days off show up dark.</div></div>

  <h3>An image to share</h3>
  <canvas class="yrcard" id="yrcard" width="1600" height="900" role="img" aria-label="${esc(`Image to share: ${dur(x.active)} with AI in ${x.y}`)}"></canvas>
  <div class="yropts">${opt("out", "Commits and PRs")}${opt("type", "Your light")}${opt("agents", "Agent breakdown")}</div>
  <div class="yract"><button class="pill" id="yrsave">Save as PNG</button>${copyBtn("Copy image", `id="yrcopy"`)}</div>
  <p class="note">The image shows only totals such as active time and sessions, and the streaks of light. Prompts, project names, branches, files and estimated cost are never included. It is made in this browser and sent nowhere.</p>

  <h3>Your light</h3>
  <div class="yrtype"><div class="yrtn"><span>${vt(name(L))}</span></div><div>
    <p class="yrtd">${L.de}</p>
    <dl class="yrev">${ev.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>
    <p class="note">${`Your light describes the shape of your year in photography terms. It is not a verdict, and its thresholds are rough guides. Why: ${L.re}`}${x.active < 600 ? " (based on little history, so take it lightly)" : ""}</p>
    <details class="yrtypes"><summary>All six and how they are chosen</summary><ul>${LIGHTS.map(l => `<li><b>${name(l)}</b> — ${l.re}</li>`).join("")}</ul>
      <p class="note">The first one that matches, from the top, is chosen.</p></details></div></div>`;
  $("#yrsel").onchange = e => { yr.y = +e.target.value; renderYear(); };
  dlg.querySelectorAll("[data-o]").forEach(c => c.onchange = () => { yr.opt[c.dataset.o] = c.checked; store.set("yrOpt", yr.opt); drawCard(); });
  const blob = () => new Promise(res => $("#yrcard").toBlob(res, "image/png"));
  $("#yrsave").onclick = async () => { const b = await blob(); if (!b) return toast("Couldn't make the image");
    const a = document.createElement("a"); a.href = URL.createObjectURL(b); a.download = `kiroku-${x.y}.png`; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000); toast("Saved the PNG"); };
  $("#yrcopy").onclick = async () => { try { await navigator.clipboard.write([new ClipboardItem({"image/png": blob()})]); toast("Copied the image"); copied($("#yrcopy")); }
    catch(e){ toast("Couldn't copy the image. Save it as a PNG instead", 2600); } };
  drawPlate(); drawCard();
  const sc = dlg.querySelector(".yrscroll"); sc.scrollLeft = sc.scrollWidth; // 狭い画面では、新しい記録の側を見せる
}
function openYear(){ const d = $("#yr"); if (!d.open) d.showModal(); renderYear(); }

