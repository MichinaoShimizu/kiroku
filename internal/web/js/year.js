// 1 年の露光（ふりかえり）
/* ── 1 年の露光（ふりかえり） ──
   1 年ぶんの作業していた区間を、横に日付・縦に時刻（朝 6 時から翌朝 6 時）の光の筋として描く。同時に動いていたほど明るく写る。
   シェア用の 1 枚もこのブラウザの中で描くだけで、どこにも送らない。画像に載せるのは集計した数字と光の筋だけで、
   プロンプト・プロジェクト名・ブランチ・ファイル・目安コストは載せない */
const PLATE = ["#3a9be0","#f07a2b","#1fbf8f","#e08fbd","#f2b53a","#7cc6f0","#9c84e6","#c9b51c"], PLATE_OTHER = "#6b7180";
const FONT = { mincho: '"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Hiragino Mincho ProN","Noto Serif JP",serif',
  sans: '"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Hiragino Sans","Noto Sans JP",system-ui,sans-serif',
  mono: 'ui-monospace,"SFMono-Regular","JetBrains Mono",Menlo,Consolas,monospace' };
const yr = { y: null, x: null, L: null, opt: Object.assign({out: true, type: true, agents: true, color: "auto"}, store.get("yrOpt", {})) };
function yearsOf(){ return [...new Set(Object.keys(MONTHS).map(k => +k.slice(0, 4)))].sort((a, b) => a - b); }
function yearData(y){
  const ms = Object.keys(MONTHS).filter(k => k.startsWith(y + "-")).sort().map(k => MONTHS[k]);
  const sum = f => ms.reduce((a, m) => a + (f(m) || 0), 0);
  const active = sum(m => m.active), fixP = sum(m => m.fixRate == null ? 0 : m.prompts);
  const by = {}; ms.forEach(m => ((m.shares || {}).source || []).forEach(s => by[s.key] = (by[s.key] || 0) + s.minutes));
  const agents = Object.keys(by).filter(k => by[k] > 0).sort((a, b) => by[b] - by[a]).map(k => ({k, min: by[k], c: PLATE[agIdx(k)] || PLATE_OTHER})); // 画面と同じエージェントの色
  const aMin = agents.reduce((a, g) => a + g.min, 0); agents.forEach(g => g.pct = aMin ? g.min / aMin * 100 : 0);
  // プロジェクト：色分けにだけ使う。シェア画像には名前を載せない
  const pm = {}; ms.forEach(m => (m.projectStats || []).forEach(p => pm[p.project] = (pm[p.project] || 0) + p.minutes));
  const projects = Object.keys(pm).filter(k => pm[k] > 0).sort((a, b) => pm[b] - pm[a]).map((k, i) => ({k, min: pm[k], c: PLATE[i] || PLATE_OTHER}));
  const pMin = projects.reduce((a, p) => a + p.min, 0); projects.forEach(p => p.pct = pMin ? p.min / pMin * 100 : 0);
  // 作業した日
  const on = new Set(); ms.forEach(m => (m.days || []).forEach((d, i) => { if (d.active > 0){ const [Y, M, D] = m.start.split("-").map(Number); on.add(key(new Date(Y, M-1, D+i))); } }));
  // 光の筋：[列（日）, 上端, 下端（列の中の 0〜1）, エージェント, プロジェクト]。色は描くときに決める（yr.color）。列は朝 6 時で区切るので、深夜の作業は前の日の列の下のほうに写る
  const y0 = new Date(y, 0, 1)/1000, y1 = new Date(y+1, 0, 1)/1000, nd = Math.round((y1 - y0)/86400), day0 = new Date(y, 0, 1);
  const lines = [], hist = new Array(24).fill(0), slots = new Set(), mine = []; let segMin = 0, nses = 0, nprompts = 0; // slots: 動いていた 5 分枠（並列を二重に数えない）
  const firstOf = {}; // エージェントを初めて使った時刻
  DATA.forEach(s => {
    const own = s.start >= y0 && s.start < y1; if (own){ nses++; nprompts += s.nPrompts || 0; mine.push(s); if (!(s.source in firstOf) || s.start < firstOf[s.source]) firstOf[s.source] = s.start; }
    s.segs.forEach(([a, b]) => {
      if (own) segMin += (b - a)/60;
      a = Math.max(a, y0); b = Math.min(b, y1); if (b <= a) return;
      for (let i = Math.floor(a/300), j = Math.ceil(b/300); i < j; i++) slots.add(i);
      while (a < b){
        const t = new Date(a*1000), d = new Date(t.getFullYear(), t.getMonth(), t.getDate() - (t.getHours() < 6 ? 1 : 0)), c0 = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 6)/1000, c1 = new Date(d.getFullYear(), d.getMonth(), d.getDate()+1, 6)/1000, e = Math.min(b, c1);
        const col = Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
        if (col >= 0 && col < nd) lines.push([col, (a - c0)/(c1 - c0), (e - c0)/(c1 - c0), s.source, s.project]);
        a = e;
      }
    });
  });
  slots.forEach(i => hist[new Date(i*300000).getHours()] += 5);
  const ht = hist.reduce((a, v) => a + v, 0) || 1, now = today0();
  const x = { y, nd, active, lines, agents, projects, sessions: nses,
    morningPct: (hist[5] + hist[6] + hist[7] + hist[8])/ht*100, peak: hist.indexOf(Math.max(...hist)),
    fix: fixP ? sum(m => m.fixRate == null ? 0 : m.fixRate * m.prompts)/fixP : null,
    avgMin: nses ? segMin/nses : 0, perSes: nses ? nprompts/nses : 0,
    commits: sum(m => m.git && m.git.commits), prs: sum(m => m.outputs && m.outputs.prs),
    days: on.size, prompts: nprompts,
    partial: now.getFullYear() === y ? now : null };
  x.late = (hist[22] + hist[23] + hist[0] + hist[1] + hist[2] + hist[3] + hist[4])/ht*100;
  let wk = 0; slots.forEach(i => { const w = new Date(i*300000).getDay(); if (w === 0 || w === 6) wk += 5; }); x.weekend = wk/ht*100;
  x.hi = highlights(x, ms, on, firstOf, day0);
  x.halves = halves(mine);
  return x;
}
/* 見どころ：データから見つけた出来事。シェア画像には光の上に書き込む（列 c0〜c1 があるもの）。良し悪しは付けず、事実だけを書く */
function highlights(x, ms, on, firstOf, day0){
  const H = [], col = d => Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
  const act = ms.filter(m => m.active > 0);
  if (act.length >= 2){ // いちばん動いていた月（ほかの月の平均より 3 割以上多いときだけ。どの月も同じなら言うことがない）
    const m = act.reduce((a, b) => b.active > a.active ? b : a), [Y, M] = m.start.split("-").map(Number);
    const rest = (act.reduce((a, b) => a + b.active, 0) - m.active)/(act.length - 1);
    if (m.active >= rest*1.3) H.push({c0: col(new Date(Y, M-1, 1)), c1: col(new Date(Y, M, 0)), t: `Busiest month: ${MON[M-1]} · ${Math.round(m.active/60)}h`});
  }
  const ds = [...on].sort(); // いちばん長い休み（記録の最初と最後の間で、7 日以上。週末だけの人の平日のような、いつもの間は出さない）
  if (ds.length >= 2){
    let best = null;
    for (let i = 1; i < ds.length; i++){
      const a = new Date(ds[i-1] + "T00:00"), b = new Date(ds[i] + "T00:00"), n = Math.round((b - a)/864e5) - 1;
      if (n >= 7 && (!best || n > best.n)) best = {n, a: addDays(a, 1), b: addDays(b, -1)};
    }
    if (best) H.push({c0: col(best.a), c1: col(best.b), t: `Longest break: ${best.a.getMonth() === best.b.getMonth() ? `${dMD(best.a)}–${best.b.getDate()}` : `${dMD(best.a)}–${dMD(best.b)}`} · ${plural(best.n, "day")}`});
  }
  const t0 = Math.min(...Object.values(firstOf)); // 途中から使い始めたエージェント（最初の記録から 2 週間より後）
  Object.entries(firstOf).filter(([, t]) => t - t0 > 14*86400).sort((a, b) => a[1] - b[1]).slice(0, 1).forEach(([k, t]) => {
    const d = new Date(t*1000); H.push({c0: col(d), c1: col(d), t: `First ${k}: ${dMD(d)}`}); });
  if (x.late >= 10) H.push({t: `Late nights (22:00–5:00): ${Math.round(x.late)}% of active time`});
  if (x.weekend >= 10) H.push({t: `Weekends: ${Math.round(x.weekend)}% of active time`});
  return H;
}
/* 前半と後半：記録のある期間を日付で半分に分け、働き方の変わったところ（大きく動いたものだけ）を並べる。
   q は画面だけに出す問いかけ（シェア画像には載せない） */
function halves(ss){
  if (!ss.length) return [];
  const t0 = Math.min(...ss.map(s => s.start)), t1 = Math.max(...ss.map(s => s.end));
  if (t1 - t0 < 28*86400) return [];
  const mid = (t0 + t1)/2;
  const half = xs => { const u = new Set(); let sum = 0, late = 0, pr = 0;
    xs.forEach(s => { pr += s.nPrompts || 0; s.segs.forEach(([a, b]) => { sum += (b - a)/60; for (let i = Math.floor(a/300), j = Math.ceil(b/300); i < j; i++) u.add(i); }); });
    u.forEach(i => { const h = new Date(i*300000).getHours(); if (h >= 22 || h < 5) late += 5; });
    const un = u.size*5 || 1; return {n: xs.length, per: xs.length ? sum/xs.length : 0, par: Math.max(0, (sum - un)/un*100), late: late/un*100, pp: xs.length ? pr/xs.length : 0}; };
  const a = half(ss.filter(s => s.start < mid)), b = half(ss.filter(s => s.start >= mid));
  if (a.n < 5 || b.n < 5) return [];
  const out = [], r = b.per/(a.per || 1);
  if (r >= 1.25) out.push({t: `sessions ${Math.round((r - 1)*100)}% longer`, q: "Sessions got longer. Bigger tasks handed over, or more back-and-forth?"});
  else if (r <= .8) out.push({t: `sessions ${Math.round((1 - r)*100)}% shorter`, q: "Sessions got shorter. Smaller, clearer asks, or more interruptions?"});
  if (Math.abs(b.par - a.par) >= 5) out.push(b.par > a.par
    ? {t: `parallel ${Math.round(a.par)}% → ${Math.round(b.par)}%`, q: "More running in parallel. Did it save you time, or add rework?"}
    : {t: `parallel ${Math.round(a.par)}% → ${Math.round(b.par)}%`, q: "Less running in parallel. Was that on purpose?"});
  if (Math.abs(b.late - a.late) >= 5) out.push(b.late > a.late
    ? {t: `late nights ${Math.round(a.late)}% → ${Math.round(b.late)}%`, q: "More late nights than before. By choice?"}
    : {t: `late nights ${Math.round(a.late)}% → ${Math.round(b.late)}%`});
  const rp = b.pp/(a.pp || 1);
  if (rp >= 1.25 || rp <= .8) out.push(rp > 1
    ? {t: `prompts per session ${a.pp.toFixed(1)} → ${b.pp.toFixed(1)}`, q: "More prompts per session. Harder tasks, or first prompts that needed more context?"}
    : {t: `prompts per session ${a.pp.toFixed(1)} → ${b.pp.toFixed(1)}`, q: "Fewer prompts per session. Clearer first prompts?"});
  return out;
}
/* 色分け：auto は、1 つのエージェントが 9 割以上ならプロジェクト（エージェントでは色に差が出ないため）、そうでなければエージェント */
function yrColorMode(x){
  const m = yr.opt.color;
  if (m === "agent" || m === "project") return m;
  return (x.agents[0] && x.agents[0].pct >= 90 && x.projects.length >= 2) ? "project" : "agent";
}
function yrColorOf(x, mode){
  if (mode === "project"){ const c = Object.fromEntries(x.projects.map(p => [p.k, p.c])); return l => c[l[4]] || PLATE_OTHER; }
  const c = Object.fromEntries(x.agents.map(g => [g.k, g.c])); return l => c[l[3]] || PLATE_OTHER;
}
/* 光の名前：使い方の傾向を写真の言葉で表す（良し悪しではない）。上から順に、最初に当てはまったもの */
const LIGHTS = [
  {k: "dawn", en: "Daybreak", m: "morning", t: x => x.morningPct >= 25,
   de: "You get things done while the morning is fresh.",
   re: "25% or more of active time is between 5:00 and 9:00"},
  {k: "night", en: "Night Sky", m: "late", t: x => x.late >= 25,
   de: "You do your best work after dark.",
   re: "25% or more of active time is between 22:00 and 5:00 (parallel sessions count once)"},
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
    late: ["Work between 22:00 and 5:00", `${Math.round(x.late)}%`],
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
  const colorFn = x.colorFn || yrColorOf(x, "agent");
  x.lines.forEach(l => {
    const [c, a, b] = l, col = colorFn(l);
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
  const ink = "#ece8df", ink2 = "#a9a69e", amber = "#ffae57";
  // 縦の割り付け：上に数字、その下に文字の帯（読み方・見どころ）、その下に光（PT〜PB）、いちばん下に凡例。
  // 光の上に文字を重ねない（朝の筋は上端に写るので、重ねると朝型の人の光が隠れる）
  const PT = 552, PB = 798;
  const ls = v => { if ("letterSpacing" in g) g.letterSpacing = v; };
  g.clearRect(0, 0, W, H); g.fillStyle = "#06070a"; g.fillRect(0, 0, W, H);
  // 使い始めたばかりでも光が端に寄らないよう、画像は記録のある日から（短ければ 4 週ぶん）だけを写す
  const last = x.partial ? Math.round((new Date(x.partial.getFullYear(), x.partial.getMonth(), x.partial.getDate()) - new Date(x.y, 0, 1))/864e5) + 1 : x.nd;
  const first = x.lines.reduce((a, l) => Math.min(a, l[0]), last), c0 = Math.max(0, Math.min(first, last - 28));
  exposure(g, 0, PT, W, PB - PT, x, 2, false, c0, last);
  // 目盛り：下に月、左に時刻（6・12・18・0 時）。どこが何月・何時かを画像だけで読めるように
  g.save(); g.fillStyle = ink2; g.globalAlpha = .75; g.font = `500 14px ${FONT.mono}`; g.textBaseline = "alphabetic"; g.textAlign = "left";
  for (let mo = 0; mo < 12; mo++){
    const c = Math.round((new Date(x.y, mo, 1) - new Date(x.y, 0, 1))/864e5);
    if (c < c0 || c >= last) continue;
    const px = (c - c0)*W/(last - c0);
    g.fillRect(px, PB + 4, 1, 8); if (px < W - 40) g.fillText(MON[mo].toUpperCase(), px + 5, PB + 22);
  }
  [["6", 0], ["12", .25], ["18", .5], ["0", .75]].forEach(([h, f]) => { const py = PT + f*(PB - PT); g.fillRect(0, py, 8, 1); g.fillText(h, 12, py + 5); });
  g.restore();
  // 画像だけを見た人にも読めるよう、光の読み方を添える
  g.textAlign = "left"; g.textBaseline = "alphabetic"; g.fillStyle = ink2; g.globalAlpha = .8; g.font = `500 16px ${FONT.mono}`; ls("2px");
  g.fillText("EACH STREAK = A STRETCH OF ACTIVE TIME   ·   ACROSS: DATE   ·   DOWN: 6:00 → 6:00", 80, 466); ls("0px"); g.globalAlpha = 1;
  // 見どころ：光の上端に括弧（期間）か目印（1 日）を付け、見出しはその上の帯に置く。重なる見出しは上の段に上げる
  const cw = W/(last - c0), cx0 = c => (c - c0)*cw;
  let rows = [[], []];
  x.hi.filter(h => h.c0 != null && h.c1 >= c0 && h.c0 < last).slice(0, 3).sort((p, q) => p.c0 - q.c0).forEach(h => {
    const xa = Math.max(0, cx0(h.c0)), xb = Math.min(W, cx0(h.c1 + 1));
    g.font = `600 19px ${FONT.sans}`; const tw = g.measureText(h.t).width;
    const tx = Math.max(80, Math.min(W - 80 - tw, (xa + xb)/2 - tw/2));
    const r = rows[0].every(([p, q]) => tx > q + 24 || tx + tw < p - 24) ? 0 : 1; rows[r].push([tx, tx + tw]);
    const ty = 534 - r*28, ly = PT - 8;
    g.strokeStyle = amber; g.globalAlpha = .9; g.lineWidth = 2;
    g.beginPath();
    if (xb - xa > 6){ g.moveTo(xa + 1, ly + 8); g.lineTo(xa + 1, ly); g.lineTo(xb - 1, ly); g.lineTo(xb - 1, ly + 8); }
    else { g.moveTo((xa + xb)/2, ly); g.lineTo((xa + xb)/2, ly + 14); }
    g.stroke(); g.globalAlpha = 1;
    g.fillStyle = amber; g.textAlign = "left"; g.fillText(h.t, tx, ty);
  });
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
  // 右の列：前半と比べた後半（大きく動いたものだけ）と、光の上に書き込めない見どころ（深夜・週末）。合わせて 3 行まで
  const side = [...(x.halves.length ? [`Second half: ${x.halves.slice(0, 2).map(h => h.t).join(", ")}`] : []), ...x.hi.filter(h => h.c0 == null).map(h => h.t)].slice(0, 3);
  if (side.length){
    const y0 = o.type && L ? 262 : 110;
    g.textAlign = "right"; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("WHAT STANDS OUT", W - 80, y0); ls("0px");
    g.fillStyle = ink; g.font = `500 26px ${FONT.sans}`;
    side.forEach((t, i) => g.fillText(t, W - 80, y0 + 42 + i*38));
    g.textAlign = "left";
  }
  if (o.agents && x.mode === "project"){ // プロジェクトの名前は載せない。色の数だけを示す
    let lx = 80; x.projects.slice(0, 8).forEach(p => { g.fillStyle = p.c; g.shadowColor = p.c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8, 856, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0; lx += 26; });
    g.font = `500 24px ${FONT.sans}`; g.fillStyle = ink; g.fillText(`Colored by project · ${plural(x.projects.length, "project")}`, lx + 10, 864);
  } else if (o.agents){
    let lx = 80; g.font = `500 24px ${FONT.sans}`;
    x.agents.slice(0, 4).forEach(a => { g.fillStyle = a.c; g.shadowColor = a.c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8, 856, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0;
      const t = `${(a.k)} ${Math.round(a.pct)}%`; g.fillStyle = ink; g.fillText(t, lx + 26, 864); lx += 26 + g.measureText(t).width + 36; });
  }
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; g.textAlign = "right"; ls("2px"); g.fillText("kiroku", W - 80, 864); ls("0px"); g.textAlign = "left";
}
function renderYear(){
  const ys = yearsOf(), dlg = $("#yr"); if (!ys.length) return;
  if (!ys.includes(yr.y)) yr.y = ys.includes(today0().getFullYear()) ? today0().getFullYear() : ys[ys.length-1];
  const x = yr.x = yearData(yr.y), L = yr.L = LIGHTS.find(l => l.t(x)), pt = x.partial;
  x.mode = yrColorMode(x); x.colorFn = yrColorOf(x, x.mode);
  const keys = x.mode === "project" ? x.projects : x.agents, qs = x.halves.filter(h => h.q);
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
  <div class="yrleg">${keys.map(a => `<span><i style="background:${a.c}"></i>${esc((a.k))} ${Math.round(a.pct)}%</span>`).join("")}</div>
  <div class="yrhow"><div><b>Length = how long a session was active</b>Each streak is one stretch of active time in a session, estimated from the timestamps in the history. It includes time the AI ran on its own.</div>
    ${x.mode === "project" ? `<div><b>Color = project</b>Shifts in color show when your time moved from one project to another. The image to share shows the colors, never the names.</div>` : `<div><b>Color = agent</b>Colors that start to mix show when you began using more than one.</div>`}
    <div><b>Dark bands = time off</b>Holidays and days off show up dark.</div></div>
  ${x.hi.length || x.halves.length ? `<h3>What stands out</h3>
  <ul class="yrhi">${x.hi.map(h => `<li>${esc(h.t)}</li>`).join("")}${x.halves.length ? `<li>Second half vs first: ${esc(x.halves.map(h => h.t).join(" · "))}</li>` : ""}</ul>` : ""}
  ${qs.length ? `<h3>To reflect on</h3>
  <ul class="yrhi">${qs.map(h => `<li>${esc(h.q)}</li>`).join("")}</ul>
  <p class="note">Questions, not judgments. Only you see them; they are not on the image. To dig into a month, copy its report prompt from the month view.</p>` : ""}

  <h3>An image to share</h3>
  <canvas class="yrcard" id="yrcard" width="1600" height="900" role="img" aria-label="${esc(`Image to share: ${dur(x.active)} with AI in ${x.y}${L ? `. Your light: ${L.en}` : ""}${x.hi.length ? `. What stands out: ${x.hi.map(h => h.t).join("; ")}` : ""}${x.halves.length ? `. Second half vs first: ${x.halves.map(h => h.t).join(", ")}` : ""}`)}"></canvas>
  <div class="yropts">${opt("out", "Commits and PRs")}${opt("type", "Your light")}${opt("agents", "Color key")}
    <label>Color by <select id="yrcolor">${[["auto", "Auto"], ["agent", "Agent"], ["project", "Project"]].map(([v, l]) => `<option value="${v}"${yr.opt.color === v ? " selected" : ""}>${l}</option>`).join("")}</select></label></div>
  <div class="yract"><button class="pill" id="yrsave">Save as PNG</button>${copyBtn("Copy image", `id="yrcopy"`)}</div>
  <p class="note">The image shows only totals such as active time and sessions, the streaks of light and what stands out (dates and hours). Prompts, project names, branches, files and estimated cost are never included. It is made in this browser and sent nowhere.</p>

  <h3>Your light</h3>
  <div class="yrtype"><div class="yrtn"><span>${vt(name(L))}</span></div><div>
    <p class="yrtd">${L.de}</p>
    <dl class="yrev">${ev.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>
    <p class="note">${`Your light describes the shape of your year in photography terms. It is not a verdict, and its thresholds are rough guides. Why: ${L.re}`}${x.active < 600 ? " (based on little history, so take it lightly)" : ""}</p>
    <details class="yrtypes"><summary>All ${LIGHTS.length} and how they are chosen</summary><ul>${LIGHTS.map(l => `<li><b>${name(l)}</b> — ${l.re}</li>`).join("")}</ul>
      <p class="note">The first one that matches, from the top, is chosen.</p></details></div></div>`;
  $("#yrsel").onchange = e => { yr.y = +e.target.value; renderYear(); };
  $("#yrcolor").onchange = e => { yr.opt.color = e.target.value; store.set("yrOpt", yr.opt); renderYear(); };
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

