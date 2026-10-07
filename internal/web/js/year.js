// 1 年の露光（ふりかえり）。いまは YEAR_ON で隠している
/* ── 1 年の露光（ふりかえり） ──
   1 年ぶんの作業していた区間を、横に日付・縦に時刻（朝 6 時から翌朝 6 時）の光の筋として描く。同時に動いていたほど明るく写る。
   シェア用の 1 枚もこのブラウザの中で描くだけで、どこにも送らない。画像に載せるのは集計した数字と光の筋だけで、
   プロンプト・プロジェクト名・ブランチ・ファイル・目安コストは載せない */
const PLATE = ["#3a9be0","#f07a2b","#1fbf8f","#e08fbd","#f2b53a","#7cc6f0","#9c84e6","#c9b51c"], PLATE_OTHER = "#6b7180";
const FONT = { mincho: '"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Hiragino Mincho ProN","Noto Serif JP",serif',
  sans: '"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Hiragino Sans","Noto Sans JP",system-ui,sans-serif',
  mono: 'ui-monospace,"SFMono-Regular","JetBrains Mono",Menlo,Consolas,monospace' };
const yr = { y: null, x: null, L: null, G: null, opt: Object.assign({out: true, type: true, grade: true, agents: true}, store.get("yrOpt", {})) };
function yearsOf(){ return [...new Set(Object.keys(MONTHS).map(k => +k.slice(0, 4)))].sort((a, b) => a - b); }
function yearData(y){
  const ms = Object.keys(MONTHS).filter(k => k.startsWith(y + "-")).sort().map(k => MONTHS[k]);
  const sum = f => ms.reduce((a, m) => a + (f(m) || 0), 0);
  const active = sum(m => m.active), fixP = sum(m => m.fixRate == null ? 0 : m.prompts);
  const by = {}; ms.forEach(m => ((m.shares || {}).source || []).forEach(s => by[s.key] = (by[s.key] || 0) + s.minutes));
  const agents = Object.keys(by).filter(k => by[k] > 0).sort((a, b) => by[b] - by[a]).map(k => ({k, min: by[k], c: PLATE[agIdx(k)] || PLATE_OTHER})); // 画面と同じエージェントの色
  const aMin = agents.reduce((a, g) => a + g.min, 0); agents.forEach(g => g.pct = aMin ? g.min / aMin * 100 : 0);
  const color = Object.fromEntries(agents.map(g => [g.k, g.c]));
  // いちばん長く続けて作業日数
  const on = new Set(); ms.forEach(m => (m.days || []).forEach((d, i) => { if (d.active > 0){ const [Y, M, D] = m.start.split("-").map(Number); on.add(key(new Date(Y, M-1, D+i))); } }));
  let streak = 0, run = 0; for (let d = new Date(y, 0, 1); d.getFullYear() === y; d = addDays(d, 1)){ run = on.has(key(d)) ? run + 1 : 0; streak = Math.max(streak, run); }
  // 光の筋：[列（日）, 上端, 下端（列の中の 0〜1）, 色]。列は朝 6 時で区切るので、深夜の作業は前の日の列の下のほうに写る
  const y0 = new Date(y, 0, 1)/1000, y1 = new Date(y+1, 0, 1)/1000, nd = Math.round((y1 - y0)/86400), day0 = new Date(y, 0, 1);
  const lines = [], hist = new Array(24).fill(0); let segMin = 0, nses = 0, nprompts = 0;
  DATA.forEach(s => {
    const mine = s.start >= y0 && s.start < y1; if (mine){ nses++; nprompts += s.nPrompts || 0; }
    s.segs.forEach(([a, b]) => {
      if (mine) segMin += (b - a)/60;
      a = Math.max(a, y0); b = Math.min(b, y1); if (b <= a) return;
      for (let t = a; t < b; t += 300) hist[new Date(t*1000).getHours()] += Math.min(300, b - t)/60;
      while (a < b){
        const t = new Date(a*1000), d = new Date(t.getFullYear(), t.getMonth(), t.getDate() - (t.getHours() < 6 ? 1 : 0)), c0 = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 6)/1000, c1 = new Date(d.getFullYear(), d.getMonth(), d.getDate()+1, 6)/1000, e = Math.min(b, c1);
        const col = Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
        if (col >= 0 && col < nd) lines.push([col, (a - c0)/(c1 - c0), (e - c0)/(c1 - c0), color[s.source] || PLATE_OTHER]);
        a = e;
      }
    });
  });
  const ht = hist.reduce((a, v) => a + v, 0) || 1, now = today0();
  // 腕前に使う数字（gradeOf）。割合は、記録のある期間・エージェントだけで出す
  const outBase = sum(m => m.outBase), cost = sum(m => m.usage && m.usage.cost), nY = Math.max(1, nses);
  const mods = {}; ms.forEach(m => ((m.usage || {}).models || []).forEach(([k, , t]) => mods[k] = (mods[k] || 0) + (t || 0)));
  const mT = Object.values(mods).reduce((a, v) => a + v, 0);
  // 上達：記録のある月が 4 か月以上なら、前半と後半で言い直しの割合と 1 コミットあたりのコストを比べる
  const half = ms.filter(m => m.prompts), hv = xs => { const p = xs.reduce((a, m) => a + (m.fixRate == null ? 0 : m.prompts), 0), c = xs.reduce((a, m) => a + (m.outputs ? m.outputs.commits : 0), 0);
    return {fix: p ? xs.reduce((a, m) => a + (m.fixRate == null ? 0 : m.fixRate * m.prompts), 0)/p : null, cpc: c >= 3 ? xs.reduce((a, m) => a + (m.costPerCommit == null ? 0 : m.costPerCommit * m.outputs.commits), 0)/c : null}; };
  const h1 = half.length >= 4 ? hv(half.slice(0, half.length >> 1)) : null, h2 = h1 && hv(half.slice(half.length >> 1));
  const grow = !!h1 && (h1.fix != null && h2.fix != null && h1.fix - h2.fix >= 3 || h1.cpc != null && h2.cpc != null && h2.cpc <= h1.cpc * .8);
  const first = DATA.find(s => s.start >= y0 && s.start < y1), span = Math.max(1, Math.round(((now.getFullYear() === y ? +now/1000 : y1) - (first ? Math.max(y0, first.start) : y0))/86400));
  return { y, nd, active, lines, agents, streak, sessions: nses,
    morningPct: (hist[5] + hist[6] + hist[7] + hist[8])/ht*100, peak: hist.indexOf(Math.max(...hist)),
    fix: fixP ? sum(m => m.fixRate == null ? 0 : m.fixRate * m.prompts)/fixP : null,
    avgMin: nses ? segMin/nses : 0, perSes: nses ? nprompts/nses : 0,
    commits: sum(m => m.git && m.git.commits), prs: sum(m => m.outputs && m.outputs.prs),
    days: on.size, prompts: nprompts, span, fixP, outBase, grow,
    reach: outBase >= 5 ? sum(m => m.outSessions)/outBase*100 : null,
    perCommit: outBase >= 5 && sum(m => m.outputs && m.outputs.commits) ? sum(m => m.outPrompts)/sum(m => m.outputs.commits) : null, // コミットを記録できる Claude Code の依頼だけ（ほかのエージェントの依頼を混ぜると多く出る）
    parPct: active ? sum(m => m.parallel)/active*100 : 0, subagents: sum(m => m.usage && m.usage.subagents),
    models: mT ? Object.values(mods).filter(v => v/mT >= .1).length : 0,
    longPct: longCtxOf(y0, y1).length/nY*100, idlePct: cost >= 1 ? idleOf(y0, y1).c/cost*100 : null, lightPct: cost >= 1 ? lightOf(y0, y1).c/cost*100 : null,
    limits: limitHits(y0, y1).length,
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
  {k: "focus", en: "Refocus", m: "fix", t: x => x.fix != null && x.fix >= 15,
   de: "You keep adjusting until it is just right.",
   re: "15% or more of prompts had a correction or interruption"},
  {k: "day", en: "Daylight", m: "peak", t: () => true,
   de: "You work with AI steadily through the day.",
   re: "None of the above"}];
/* 腕前：光の名前（型）とは別に、4 つのメーターで積み重ね・仕上げる力・使いこなしの幅・回り道を 0〜1 で測る。露光の画面だけの遊びで、週次・月次サマリーには使わない */
const TIERS = [ // 点数で決める 10 段階（10% ごと）
  "Novice", "Beginner", "Apprentice", "Competent", "Skilled",
  "Expert", "Master", "Virtuoso", "Grandmaster", "Legendary"].map((en, i) => ({en, u: i === 9 ? Infinity : (i + 1)/10}));
const OVERS = [ // 露出オーバー（働き方の危うさ・回り道が多すぎる）。上から順に、最初に当てはまったものを等級の代わりに冠にする
  {k: "blown", en: "Blown-Out", t: x => x.streak >= 30 && x.active/Math.max(1, x.days) >= 120,
   re: "30+ days in a row without a break at 2+ hours a day"},
  {k: "film", en: "Out-of-Film", t: x => x.limits >= 10,
   re: "Hit a usage limit 10 or more times"},
  {k: "noise", en: "Grainy", t: (x, G) => G.m.noise != null && G.m.noise >= .6,
   re: "Noise (detours) at 60% or more"}];
function gradeOf(x){
  const c = v => Math.max(0, Math.min(1, v)), avg = a => (a = a.filter(v => v != null)).length ? a.reduce((s, v) => s + v, 0)/a.length : null;
  const top2 = a => avg(a.slice().sort((p, q) => q - p).slice(0, 2)); // 使いこなしの幅は、どれか 2 つが得意なら高くなる
  const f = Math.max(.5, Math.min(1, x.span/365)); // 年の途中や使い始めの年は、経った日数のぶんだけ満点を下げる（使い始めてすぐ満点にならないよう、半分まで）
  const eff = 1/(x.agents.reduce((s, g) => s + (g.pct/100)**2, 0) || 1); // 使い分けの度合い（1 つだけなら 1、3 つを均等なら 3）
  const m = {
    shutter: avg([c(x.days/(180*f)), c(Math.log1p(x.prompts)/Math.log1p(3000*f)), c(Math.log1p(x.active/60)/Math.log1p(800*f))]),
    focus: avg([x.reach == null ? null : c(x.reach/50), x.perCommit == null ? null : c((20 - x.perCommit)/15)]), // コミットまで 50%・5 プロンプトで 1 コミットで満点
    multi: top2([c(x.parPct/30), c(Math.log1p(x.subagents)/Math.log1p(200*f)), c((eff - 1)/2), c((x.models - 1)/2)]),
    noise: avg([x.fix == null ? null : c((x.fix - 5)/20), c(x.longPct/15), x.idlePct == null ? null : c(x.idlePct/40), x.lightPct == null ? null : c(x.lightPct/20)]) };
  const base = avg([m.shutter, m.focus, m.multi]) || 0, score = Math.max(0, base - .4*(m.noise || 0));
  let ti = TIERS.findIndex(t => score < t.u);
  if (x.grow) ti = Math.min(TIERS.length - 1, ti + 1); // 上達していれば 1 段上げる
  if (ti === TIERS.length - 1 && !([m.shutter, m.focus ?? 1, m.multi].every(v => v >= .8) && (m.noise || 0) <= .2)) ti--; // 伝説は 3 つとも高く、ノイズが少ないときだけ
  const G = {m, score, tier: TIERS[ti], grow: x.grow};
  G.over = OVERS.find(o => o.t(x, G));
  G.exp = G.over ? "over" : score < .4 ? "under" : "ok";
  return G;
}
const pips = v => v == null ? 0 : Math.max(0, Math.min(10, Math.round(v*10))); // メーターの 10 段階
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
function cardMeters(g, X, Y, ink2, amber){ // シェア画像の腕前：4 つのメーターを 10 段階で
  const G = yr.G; g.save(); g.textAlign = "left"; g.textBaseline = "alphabetic";
  [["shutter", "SHUTTER"], ["focus", "FOCUS"], ["multi", "LAYERS"], ["noise", "NOISE"]].forEach(([k, n], r) => {
    const y = Y + r*38, p = pips(G.m[k]), on = k === "noise" ? "#ff7a5c" : amber;
    g.fillStyle = ink2; g.font = `500 ${(16)}px ${(FONT.mono)}`; if (("letterSpacing" in g)) g.letterSpacing = "3px"; g.fillText(n, X, y); if ("letterSpacing" in g) g.letterSpacing = "0px";
    for (let i = 0; i < 10; i++){ g.fillStyle = i < p ? on : "rgba(255,255,255,.12)"; g.shadowColor = on; g.shadowBlur = i < p ? 10 : 0;
      g.beginPath(); g.roundRect ? g.roundRect(X + 140 + i*16, y - 14, 12, 12, 4) : g.rect(X + 140 + i*16, y - 14, 12, 12); g.fill(); }
    g.shadowBlur = 0; });
  g.restore();
}
function drawCard(){
  const cv = $("#yrcard"); if (!cv || !yr.x) return;
  const x = yr.x, L = yr.L, o = yr.opt, W = 1600, H = 900, g = cv.getContext("2d");
  const ink = "#ece8df", ink2 = "#a9a69e", amber = "#ffae57", top = 430;
  const ls = v => { if ("letterSpacing" in g) g.letterSpacing = v; };
  g.clearRect(0, 0, W, H); g.fillStyle = "#06070a"; g.fillRect(0, 0, W, H);
  // 使い始めたばかりでも光が端に寄らないよう、画像は記録のある日から（短ければ 8 週ぶん）だけを写す
  const last = x.partial ? Math.round((new Date(x.partial.getFullYear(), x.partial.getMonth(), x.partial.getDate()) - new Date(x.y, 0, 1))/864e5) + 1 : x.nd;
  const first = x.lines.reduce((a, l) => Math.min(a, l[0]), last), c0 = Math.max(0, Math.min(first, last - 56));
  exposure(g, 0, top, W, H - top, x, 2, false, c0, last);
  const fade = g.createLinearGradient(0, top, 0, H);
  fade.addColorStop(0, "rgba(6,7,10,1)"); fade.addColorStop(.3, "rgba(6,7,10,0)"); fade.addColorStop(.72, "rgba(6,7,10,0)"); fade.addColorStop(1, "rgba(6,7,10,.92)");
  g.fillStyle = fade; g.fillRect(0, top, W, H - top);
  g.textAlign = "left"; g.textBaseline = "alphabetic";
  const pt = x.partial;
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; ls("4px");
  const d0 = new Date(x.y, 0, 1 + c0), d1 = new Date(x.y, 0, last);
  g.fillText(c0 || pt ? `KIROKU — ${dSpan(d0, d1, true).toUpperCase()}` : `KIROKU — ${x.y} EXPOSURE`, 80, 110); ls("0px");
  const hrs = x.active/60, hs = (hrs >= 10 ? Math.round(hrs) : Math.round(hrs*10)/10).toLocaleString(LOC());
  g.fillStyle = ink; g.font = `800 184px ${FONT.mincho}`; g.fillText(hs, 72, 300);
  const hw = g.measureText(hs).width;
  g.font = `600 54px ${FONT.mincho}`; g.fillText("hours", 72 + hw + 16, 300);
  g.fillStyle = ink2; g.font = `500 30px ${FONT.sans}`;
  g.fillText(pt ? "This year's light, with AI." : "A year of light, with AI.", 80, 360);
  const items = [[x.sessions, "sessions"]];
  if (o.out){ if (x.commits) items.push([x.commits, "commits"]); if (x.prs) items.push([x.prs, "PR" + (("s"))]); }
  let cx = 80;
  items.forEach(([n, l]) => { g.font = `500 26px ${FONT.mono}`; g.fillStyle = ink; const t = n.toLocaleString(LOC()); g.fillText(t, cx, 412); cx += g.measureText(t).width + 10;
    g.font = `500 24px ${FONT.sans}`; g.fillStyle = ink2; g.fillText(l, cx, 412); cx += g.measureText(l).width + 40; });
  if (o.type && L){
    const G = yr.G, pre = o.grade ? (G.over ? G.over.en : G.tier.en) : "", hot = "#ff7a5c", pc = o.grade && G.over ? hot : amber;
    g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36;
    {
      g.textAlign = "right"; g.shadowBlur = 0; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("YOUR LIGHT", W - 80, 110); ls("0px");
      if (pre){ g.shadowColor = pc; g.shadowBlur = 24; g.fillStyle = pc; g.font = `700 34px ${FONT.mincho}`; g.fillText(pre, W - 80, 160); }
      g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36; g.fillStyle = amber; g.font = `800 64px ${FONT.mincho}`; g.fillText(L.en, W - 80, pre ? 228 : 190);
      g.shadowBlur = 0; if (o.grade) cardMeters(g, W - 80 - 300, 280, ink2, amber);
    }
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
  const x = yr.x = yearData(yr.y), L = yr.L = LIGHTS.find(l => l.t(x)), G = yr.G = gradeOf(x), pt = x.partial;
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
  <div class="yrhow"><div><b>Length = how long you worked</b>Each streak is one stretch of active time in a session.</div>
    <div><b>Color = agent</b>Colors that start to mix show when you began using more than one.</div>
    <div><b>Dark bands = time off</b>Holidays and days off show up dark.</div></div>

  <h3>An image to share</h3>
  <canvas class="yrcard" id="yrcard" width="1600" height="900" role="img" aria-label="${esc(`Image to share: ${dur(x.active)} with AI in ${x.y}`)}"></canvas>
  <div class="yropts">${opt("out", "Commits and PRs")}${opt("type", "Your light")}${opt("grade", "Skill")}${opt("agents", "Agent breakdown")}</div>
  <div class="yract"><button class="pill" id="yrsave">Save as PNG</button><button class="pill" id="yrcopy">Copy image</button></div>
  <p class="note">The image shows only totals such as active time and sessions, and the streaks of light. Prompts, project names, branches, files and estimated cost are never included. It is made in this browser and sent nowhere.</p>

  <h3>Your light</h3>
  <div class="yrtype"><div class="yrtn"><span class="pre${G.over ? " over" : ""}">${vt(G.over ? G.over.en : G.tier.en)}</span><span>${vt(name(L))}</span></div><div>
    <p class="yrtd">${L.de}</p>
    <div class="yrgrade"><div class="yrexp ${G.exp}">${{under: "Underexposed: room to grow", ok: "Well exposed", over: `Overexposed: ${G.over ? G.over.re : ""}`}[G.exp]}</div>${G.grow ? `<div class="yrexp ok">↗ Improving: the second half beat the first (one level up)</div>` : ""}
      ${meters(x, G)}</div>
    <dl class="yrev">${ev.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>
    <p class="note">${`Your light describes the shape of your year in photography terms. It is not a verdict. Why: ${L.re}`}${x.active < 600 ? " (based on little history, so take it lightly)" : ""}</p>
    <details class="yrtypes"><summary>All six and how they are chosen</summary><ul>${LIGHTS.map(l => `<li><b>${name(l)}</b> — ${l.re}</li>`).join("")}</ul>
      <p class="note">The first one that matches, from the top, is chosen.</p></details>
    <details class="yrtypes"><summary>Skill levels and overexposure</summary>
      <ul>${TIERS.map((t, i) => `<li><b>${t.en}</b> — ${`Score ${i*10}–${i*10 + 10}%`}</li>`).join("")}
        ${OVERS.map(o => `<li><b>${o.en}</b> — ${o.re}</li>`).join("")}</ul>
      <p class="note">${"The score is the average of shutter count, focus and layers, minus 40% of noise. Legendary needs all three at 80% or more and noise at 20% or less. If corrections drop by 3 points or cost per commit falls by 20% from the first half of the year to the second, you are \"improving\" and go up one level. Shutter count comes from active days (180), prompts (3,000) and active time (800 hours); focus from sessions that reached a commit or pull request (50%) and few prompts per commit (5); layers from your best two of parallel time (30%), subagents (200 runs), and using several agents and models; noise from corrections, long conversations, cost with no commit and expensive models for light work (full marks in brackets, scaled down to as little as half for a year in progress). These are rough proxies that can move for reasons unrelated to skill, so treat it as a game."}</p></details></details></div></div>`;
  $("#yrsel").onchange = e => { yr.y = +e.target.value; renderYear(); };
  dlg.querySelectorAll("[data-o]").forEach(c => c.onchange = () => { yr.opt[c.dataset.o] = c.checked; store.set("yrOpt", yr.opt); drawCard(); });
  const blob = () => new Promise(res => $("#yrcard").toBlob(res, "image/png"));
  $("#yrsave").onclick = async () => { const b = await blob(); if (!b) return toast("Couldn't make the image");
    const a = document.createElement("a"); a.href = URL.createObjectURL(b); a.download = `kiroku-${x.y}.png`; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000); toast("Saved the PNG"); };
  $("#yrcopy").onclick = async () => { try { await navigator.clipboard.write([new ClipboardItem({"image/png": blob()})]); toast("Copied the image"); }
    catch(e){ toast("Couldn't copy the image. Save it as a PNG instead", 2600); } };
  drawPlate(); drawCard();
  const sc = dlg.querySelector(".yrscroll"); sc.scrollLeft = sc.scrollWidth; // 狭い画面では、新しい記録の側を見せる
}
function meters(x, G){
  const pct = v => v == null ? "—" : `${Math.round(v)}%`;
  const rows = [
    ["shutter", "Shutter count", "Practice", `${plural(x.days, "day")} · ${plural(x.prompts, "prompt")} · ${dur(x.active)}`],
    ["focus", "Focus", "Follow-through", `To commit ${pct(x.reach)} · ${x.perCommit == null ? "—" : x.perCommit.toFixed(1)} prompts per commit`],
    ["multi", "Layers", "Range", `Parallel ${pct(x.parPct)} · ${plural(x.subagents, "subagent run")} · ${plural(x.agents.length, "agent")} · ${plural(x.models, "model")}`],
    ["noise", "Noise", "Detours (fewer means a cleaner shot)", `Corrections ${pct(x.fix)} · long conversations ${pct(x.longPct)} · cost with no commit ${pct(x.idlePct)} · expensive models for light work ${pct(x.lightPct)}`]];
  return `<dl class="yrmeter">${rows.map(([k, n, sub, v]) => { const p = pips(G.m[k]);
    return `<div${k === "noise" ? ' class="neg"' : ""}><dt>${n}<small>${sub}</small></dt><dd><span class="pp" role="img" aria-label="${`${p} of 10`}">${"<i class=on></i>".repeat(p)}${"<i></i>".repeat(10 - p)}</span><span class="pv">${v}</span></dd></div>`; }).join("")}</dl>`;
}
function openYear(){ const d = $("#yr"); if (!d.open) d.showModal(); renderYear(); }

