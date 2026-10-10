// 1 年の露光（ふりかえり）
/* ── 1 年の露光（ふりかえり） ──
   1 年ぶんの作業していた区間を、横に日付・縦に時刻（朝 6 時から翌朝 6 時）の光の筋として描く。同時に動いていたほど明るく写る。
   シェア用の 1 枚もこのブラウザの中で描くだけで、どこにも送らない。画像に載せるのは集計した数字と光の筋だけで、
   プロンプト・プロジェクト名・ブランチ・ファイル・目安コストは載せない */
const PLATE = ["#3a9be0","#f07a2b","#1fbf8f","#e08fbd","#f2b53a","#7cc6f0","#9c84e6","#c9b51c"], PLATE_OTHER = "#6b7180";
// プロジェクトの色：となりあう順位の色が似ないよう（青と水色など）、色相と明るさの離れた順に並べる
const PLATE_PROJ = ["#3a9be0","#e05c8a","#1fbf8f","#f2b53a","#9c84e6","#f07a2b","#e8e4d8","#7cc6f0"];
const FONT = { mincho: '"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Hiragino Mincho ProN","Noto Serif JP",serif',
  sans: '"Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Hiragino Sans","Noto Sans JP",system-ui,sans-serif',
  mono: 'ui-monospace,"SFMono-Regular","JetBrains Mono",Menlo,Consolas,monospace' };
const yr = { y: null, x: null, L: null, opt: Object.assign({totals: true, type: true, agents: true, late: false, breaks: false, color: "auto"}, store.get("yrOpt", {})) };
// 画像に載せるものは画像の上のチェックで選ぶ。深夜・週末の割合といちばん長い休みは、見せたくない人もいるので初めは載せない
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
  const projects = Object.keys(pm).filter(k => pm[k] > 0).sort((a, b) => pm[b] - pm[a]).map((k, i) => ({k, min: pm[k], c: PLATE_PROJ[i] || PLATE_OTHER}));
  const pMin = projects.reduce((a, p) => a + p.min, 0); projects.forEach(p => p.pct = pMin ? p.min / pMin * 100 : 0);
  // 作業した日
  const on = new Set(); ms.forEach(m => (m.days || []).forEach((d, i) => { if (d.active > 0){ const [Y, M, D] = m.start.split("-").map(Number); on.add(key(new Date(Y, M-1, D+i))); } }));
  // 光の筋：[列（日）, 上端, 下端（列の中の 0〜1）, エージェント, プロジェクト]。色は描くときに決める（yr.color）。列は朝 6 時で区切るので、深夜の作業は前の日の列の下のほうに写る
  const y0 = new Date(y, 0, 1)/1000, y1 = new Date(y+1, 0, 1)/1000, nd = Math.round((y1 - y0)/86400), day0 = new Date(y, 0, 1);
  const lines = [], hist = new Array(24).fill(0), slots = new Set(), mine = []; let segMin = 0, nses = 0, nprompts = 0; // slots: 動いていた 5 分枠（並列を二重に数えない）
  // エージェントを初めて使った時刻は、すべての記録から（前の年から使っていたエージェントを「この年に始めた」としない）
  const firstAll = {}; let t0All = Infinity;
  DATA.forEach(s => { if (!(s.source in firstAll) || s.start < firstAll[s.source]) firstAll[s.source] = s.start; t0All = Math.min(t0All, s.start); });
  const firstOf = Object.fromEntries(Object.entries(firstAll).filter(([, t]) => t >= y0 && t < y1));
  DATA.forEach(s => {
    const own = s.start >= y0 && s.start < y1; if (own){ nses++; nprompts += s.nPrompts || 0; mine.push(s); }
    s.segs.forEach(([a, b]) => {
      if (own) segMin += (b - a)/60;
      plateCols(a, b, y).forEach(c => lines.push([...c, s.source, s.project]));
      a = Math.max(a, y0 + 21600); b = Math.min(b, y1 + 21600); if (b <= a) return; // 集計も光と同じく、1/1 の 6 時から翌年 1/1 の 6 時まで
      for (let i = Math.floor(a/300), j = Math.ceil(b/300); i < j; i++) slots.add(i);
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
  // 記録のある期間（光の最初の列から最後の列まで）。8 週に満たなければ、前半と後半・光の名前は出さない（言えることが少ない）
  x.c0 = lines.reduce((a, l) => Math.min(a, l[0]), nd); x.c1 = lines.reduce((a, l) => Math.max(a, l[0]), -1); // 光のある最初と最後の列
  x.span = lines.length ? x.c1 - x.c0 + 1 : 0;
  x.short = x.span < 56;
  x.firstYear = yearsOf()[0] === y;
  x.hi = highlights(x, ms, on, firstOf, day0, t0All);
  x.halves = halves(mine);
  return x;
}
/* 区間 [a, b)（秒）を、y 年の光の列に分ける。列は朝 6 時から翌朝 6 時までの 1 日で、列 0 は 1/1 の 6 時から。
   1/1 の 0〜6 時は前の年の最後の列（大晦日の夜）に写る。返すのは [列, 上端, 下端]（列の中の 0〜1） */
function plateCols(a, b, y){
  const out = [], day0 = new Date(y, 0, 1), nd = Math.round((new Date(y+1, 0, 1) - day0)/864e5);
  a = Math.max(a, new Date(y, 0, 1, 6)/1000); b = Math.min(b, new Date(y+1, 0, 1, 6)/1000);
  while (a < b){
    const t = new Date(a*1000), d = new Date(t.getFullYear(), t.getMonth(), t.getDate() - (t.getHours() < 6 ? 1 : 0));
    const c0 = new Date(d.getFullYear(), d.getMonth(), d.getDate(), 6)/1000, c1 = new Date(d.getFullYear(), d.getMonth(), d.getDate()+1, 6)/1000, e = Math.min(b, c1);
    const col = Math.round((d - day0)/864e5);
    if (col >= 0 && col < nd) out.push([col, (a - c0)/(c1 - c0), (e - c0)/(c1 - c0)]);
    if (e <= a) break; // 時計が戻る日（夏時間の終わりなど）でも止まらないように
    a = e;
  }
  return out;
}
/* 見どころ：データから見つけた出来事。シェア画像には光の上に書き込む（列 c0〜c1 があるもの）。良し悪しは付けず、事実だけを書く */
function highlights(x, ms, on, firstOf, day0, t0){
  const H = [], col = d => Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()) - day0)/864e5);
  const pcol = d => col(new Date(d.getFullYear(), d.getMonth(), d.getDate() - (d.getHours() < 6 ? 1 : 0))); // 光と同じ、朝 6 時で区切った列
  const act = ms.filter(m => m.active > 0);
  if (act.length >= 3 && !x.short){ // いちばん動いていた月（記録が 8 週以上・3 か月以上あり、ほかの月の平均より 3 割以上多く、10 時間以上のときだけ。比べる月が少なければ、どの月も同じなら言うことがない）
    const m = act.reduce((a, b) => b.active > a.active ? b : a), [Y, M] = m.start.split("-").map(Number);
    const rest = (act.reduce((a, b) => a + b.active, 0) - m.active)/(act.length - 1);
    if (m.active >= rest*1.3 && m.active >= 600){ const t = `Busiest month: ${MON[M-1]} · ${Math.round(m.active/60)}h`; H.push({k: "month", c0: col(new Date(Y, M-1, 1)), c1: col(new Date(Y, M, 0)), t, img: t}); }
  }
  const ds = [...on].sort(); // いちばん長い休み（記録の最初と最後の間で、7 日以上。週末だけの人の平日のような、いつもの間は出さない）
  if (ds.length >= 2){
    let best = null;
    for (let i = 1; i < ds.length; i++){
      const a = new Date(ds[i-1] + "T00:00"), b = new Date(ds[i] + "T00:00"), n = Math.round((b - a)/864e5) - 1;
      if (n >= 7 && (!best || n > best.n)) best = {n, a: addDays(a, 1), b: addDays(b, -1)};
    }
    // いちばん長い連続（動いた日が 14 日以上）。土日の空きは途切れにしない（平日だけ働く人も続いていれば数える）
    const weekendOnly = (p, q) => { for (let d = addDays(p, 1); d < q; d = addDays(d, 1)) if (d.getDay() !== 0 && d.getDay() !== 6) return false; return true; };
    let run = null, from = 0;
    for (let i = 1; i <= ds.length; i++){
      const cont = i < ds.length && weekendOnly(new Date(ds[i-1] + "T00:00"), new Date(ds[i] + "T00:00"));
      if (!cont){ const n = i - from; if (n >= 14 && (!run || n > run.n)) run = {n, a: new Date(ds[from] + "T00:00"), b: new Date(ds[i-1] + "T00:00")}; from = i; }
    }
    // 連続は画面の一覧にだけ出す（画像に印を付けると、続けることが目標のように読める）
    if (run) H.push({k: "streak", c0: col(run.a), c1: col(run.b), t: `Most consecutive active days: ${run.n} (a weekend off doesn't break a run)`});
    // 休みは、画像では日付を書かない（画像に載せるかどうかも選べる。初めは載せない）
    if (best) H.push({k: "break", c0: col(best.a), c1: col(best.b), t: `Longest stretch without AI: ${plural(best.n, "day")} (${best.a.getMonth() === best.b.getMonth() ? `${dMD(best.a)}–${best.b.getDate()}` : `${dMD(best.a)}–${dMD(best.b)}`})`, img: `Longest stretch without AI: ${plural(best.n, "day")}`});
  }
  // 途中から使い始めたエージェント：初めて使ったのがこの年で、すべての記録の最初から 2 週間より後
  if (t0 == null) t0 = Math.min(...Object.values(firstOf));
  Object.entries(firstOf).filter(([, t]) => t - t0 > 14*86400).sort((a, b) => a[1] - b[1]).slice(0, 1).forEach(([k, t]) => {
    const d = new Date(t*1000), s = `First ${k}: ${dMD(d)}`; H.push({k: "first", c0: pcol(d), c1: pcol(d), t: s, img: s}); });
  if (x.late >= 10){ const t = `Late nights (22:00–5:00): ${Math.round(x.late)}% of active time`; H.push({k: "late", t, img: t}); }
  if (x.weekend >= 10){ const t = `Weekends: ${Math.round(x.weekend)}% of active time`; H.push({k: "weekend", t, img: t}); }
  return H;
}
/* 前半と後半：記録のある期間を日付で半分に分け、働き方の変わったところ（大きく動いたものだけ）を並べる。
   q は画面だけに出す問いかけ（シェア画像には載せない） */
function halves(ss){
  if (!ss.length) return [];
  const t0 = Math.min(...ss.map(s => s.start)), t1 = Math.max(...ss.map(s => s.end));
  if (t1 - t0 < 56*86400) return []; // 8 週に満たなければ、半分ずつでは揺れのほうが大きい
  const mid = (t0 + t1)/2;
  // 並列の割合は「2 つ以上のセッションが実際に重なって動いていた時間 ÷ どれかが動いていた時間」（0〜100%。3 つ同時でも 100% を超えない）。
  // 続けて動かしただけ（重ならない）のセッションは数えない。深夜の割合は、動いていた 5 分枠（重ねて数えない）で
  const half = xs => { const u = new Set(), ev = []; let sum = 0, late = 0, pr = 0;
    xs.forEach(s => { pr += s.nPrompts || 0;
      mergeSegs(s.segs).forEach(([a, b]) => { sum += b - a; ev.push([a, 1], [b, -1]); for (let i = Math.floor(a/300), j = Math.ceil(b/300); i < j; i++) u.add(i); }); });
    u.forEach(i => { const h = new Date(i*300000).getHours(); if (h >= 22 || h < 5) late += 5; });
    ev.sort((p, q) => p[0] - q[0] || p[1] - q[1]); // 同じ時刻なら終わりを先に（つながっているだけのものを重なりとしない）
    let depth = 0, prev = 0, any = 0, two = 0;
    ev.forEach(([t, d]) => { if (depth >= 1) any += t - prev; if (depth >= 2) two += t - prev; depth += d; prev = t; });
    return {n: xs.length, per: xs.length ? sum/60/xs.length : 0, par: any ? two/any*100 : 0, late: u.size ? late/(u.size*5)*100 : 0, pp: xs.length ? pr/xs.length : 0}; };
  const a = half(ss.filter(s => s.start < mid)), b = half(ss.filter(s => s.start >= mid));
  if (a.n < 5 || b.n < 5) return [];
  const out = [], r = b.per/(a.per || 1);
  if (r >= 1.25) out.push({k: "len", t: `sessions ${Math.round((r - 1)*100)}% longer`, q: "Sessions got longer. Bigger tasks handed over, or more back-and-forth?"});
  else if (r <= .8) out.push({k: "len", t: `sessions ${Math.round((1 - r)*100)}% shorter`, q: "Sessions got shorter. Smaller, clearer asks, or more interruptions?"});
  if (Math.abs(b.par - a.par) >= 5) out.push(b.par > a.par
    ? {k: "par", t: `2+ sessions at once ${Math.round(a.par)}% → ${Math.round(b.par)}%`, q: "You ran more sessions at the same time. Did it save you time, or add rework and cost?"}
    : {k: "par", t: `2+ sessions at once ${Math.round(a.par)}% → ${Math.round(b.par)}%`, q: "You ran fewer sessions at the same time. Was that on purpose?"});
  if (Math.abs(b.late - a.late) >= 5) out.push(b.late > a.late
    ? {k: "late", t: `late nights (22:00–5:00) ${Math.round(a.late)}% → ${Math.round(b.late)}%`, q: "More of your AI time fell between 22:00 and 5:00 than before. Was that the time you'd choose?"}
    : {k: "late", t: `late nights (22:00–5:00) ${Math.round(a.late)}% → ${Math.round(b.late)}%`});
  const rp = b.pp/(a.pp || 1); // 依頼の記録がほとんどない履歴（0.0 → 0.0 など）では比べない
  if (a.pp >= 1 && b.pp >= 1 && a.pp.toFixed(1) !== b.pp.toFixed(1) && (rp >= 1.25 || rp <= .8)) out.push(rp > 1
    ? {k: "pp", t: `prompts per session ${a.pp.toFixed(1)} → ${b.pp.toFixed(1)}`, q: "More prompts per session. Harder tasks, or first prompts that needed more context?"}
    : {k: "pp", t: `prompts per session ${a.pp.toFixed(1)} → ${b.pp.toFixed(1)}`, q: "Fewer prompts per session. Clearer first prompts?"});
  return out;
}
/* 1 つのセッションの区間を、重なりなくつなげる（同じセッションを並列と数えないように） */
function mergeSegs(segs){
  const out = [];
  [...segs].sort((p, q) => p[0] - q[0]).forEach(([a, b]) => { const l = out[out.length - 1]; if (l && a <= l[1]) l[1] = Math.max(l[1], b); else out.push([a, b]); });
  return out;
}
/* シェア画像の右の列：前半と比べた後半（1 つ 1 行、2 つまで）と、光の上に書き込めない見どころ（深夜・週末）。合わせて 3 行まで。
   深夜・週末は o.late のときだけ。前半後半で深夜が動いたなら、深夜の割合の行は重ねて出さない */
function sideLines(x, o){
  // 深夜・週末を選んだなら、深夜の変化を先に（2 行で切ったときに落ちないように）
  const hs = x.halves.filter(h => h.k !== "late" || o.late).sort((p, q) => (q.k === "late") - (p.k === "late")), lateMoved = hs.some(h => h.k === "late");
  return [...hs.slice(0, 2).map(h => `2nd half vs 1st: ${h.t}`),
    ...x.hi.filter(h => h.c0 == null && h.img && o.late && !(lateMoved && h.k === "late")).map(h => h.img)].slice(0, 3);
}
/* 画像の光の上に書き込む見どころ（期間か日のあるもの）。休みは o.breaks のときだけ */
function cardMarks(x, o){
  return x.hi.filter(h => h.c0 != null && h.img && (h.k !== "break" || o.breaks));
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
  {k: "dawn", en: "Daybreak", m: "morning", t: x => x.morningPct >= 25, sub: "a lot of AI time early in the morning",
   de: "Much of your work with AI happens early in the morning.",
   ask: "A quarter or more of your AI time was between 5:00 and 9:00. Is that when your prompts come out clearest?",
   re: "25% or more of active time is between 5:00 and 9:00"},
  {k: "night", en: "Night Sky", m: "late", t: x => x.late >= 25, sub: "a lot of AI time late at night",
   de: "Much of your work with AI happens late at night.",
   ask: "A quarter or more of your AI time was between 22:00 and 5:00. Is that the time you'd choose, or the time that was left?",
   re: "25% or more of active time is between 22:00 and 5:00 (parallel sessions count once)"},
  {k: "multi", en: "Multiple Exposure", m: "agents", t: x => x.agents.filter(g => g.pct >= 10).length >= 3, sub: "several agents, each used a fair share",
   de: "You used several agents, each for a fair share of your time.",
   ask: "You used several agents. Which job did each one do best, and would one have been enough?",
   re: "3 or more agents, each with 10% or more of active time"},
  {k: "long", en: "Long Exposure", m: "avg", t: x => x.avgMin >= 45 && x.perSes <= 10, sub: "long sessions with few prompts",
   de: "Your sessions ran long, with few prompts.",
   ask: "Your sessions ran long with few prompts. How often did the first result need a second round?",
   re: "45 minutes or more of active time per session, with 10 prompts or fewer on average"},
  {k: "burst", en: "Burst", m: "per", t: x => x.perSes >= 15, sub: "many short prompts in each session",
   de: "Your sessions had many short prompts.",
   ask: "Your sessions had many prompts. Would some of them have worked better as one clear request?",
   re: "15 or more prompts per session on average"},
  {k: "focus", en: "Bracketing", m: "fix", t: x => x.fix != null && x.fix >= 15, sub: "many prompts that correct or retry",
   de: "Many of your prompts corrected or retried an earlier answer.",
   ask: "Many prompts corrected or retried an answer. What was missing from the first prompt?",
   re: "15% or more of prompts look like a follow-up correction or came after an interruption (guessed from the wording)"},
  {k: "day", en: "Daylight", m: "peak", t: () => true, sub: "steady through the day",
   de: "You work with AI steadily through the day.",
   ask: "Your year was steady. What would you hand to AI next year that you did by hand this year?",
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
/* 写す列の範囲 [c0, last)：最初の記録の日から今日（その年の終わり）まで。
   記録が 8 週に満たなければ、最後の記録まで（空いた右側や、記録の始まる前が「使わなかった日」に見えないように）。画面の図とシェア画像で同じ範囲 */
function yrRange(x){
  const end = x.partial ? Math.round((new Date(x.partial.getFullYear(), x.partial.getMonth(), x.partial.getDate()) - new Date(x.y, 0, 1))/864e5) + 1 : x.nd;
  if (!x.lines.length) return {c0: Math.max(0, end - 28), last: end};
  // 4 週に広げるのは右（記録の後）へだけ。今日や年の終わりで広げられなければ、4 週に満たなくてもそのまま（記録の前の日を写さない）
  const c0 = Math.min(x.c0, end - 1);
  return {c0, last: x.short ? Math.min(end, Math.max(x.c1 + 1, c0 + 28)) : end};
}
function drawPlate(){
  const cv = $("#yrplate"); if (!cv || !yr.x) return;
  const r = cv.getBoundingClientRect(), p = Math.min(2, devicePixelRatio || 1), W = Math.round(r.width*p), H = Math.round(r.height*p), R = yrRange(yr.x);
  if (!W || !H) return; cv.width = W; cv.height = H; exposure(cv.getContext("2d"), 0, 0, W, H, yr.x, p, true, R.c0, R.last);
}
/* 画像の見出し：記録の長さに合わせる（短い記録に「1 年」と書かない） */
function yrTitle(x){
  if (x.short){ // 2 週に満たなければ日で、それ以上は週で数える。光が 1 本もない年は数を言わない
    const n = x.span < 14 ? plural(x.span, "day") : plural(Math.round(x.span/7), "week");
    if (!x.span) return x.firstYear ? "Your first days with AI." : "A few days with AI.";
    if (x.span === 1) return x.firstYear ? "Your first day with AI." : "A day with AI.";
    return x.firstYear ? `Your first ${n} with AI.` : `${n} with AI.`; }
  return x.partial ? "This year with AI, so far." : "A year with AI.";
}
function drawCard(){
  const cv = $("#yrcard"); if (!cv || !yr.x) return;
  const x = yr.x, L = yr.L, o = yr.opt, W = 1600, H = 900, g = cv.getContext("2d");
  const ink = "#ece8df", ink2 = "#a9a69e", amber = "#ffae57";
  // 割り付け：上に見出しと小さな合計、右に光の名前と見どころ、その下に読み方と見どころの見出しの帯、その下に光（主役）、いちばん下に凡例と kiroku の場所。
  // 光は左右に余白を取り（PX〜PX+PW）、時刻の目盛りはその左の余白に書く。光の上に文字を重ねない（朝の筋は上端に写るので、重ねると朝型の人の光が隠れる）
  const PX = 80, PW = W - 160, PT = 492, PB = 792;
  const ls = v => { if ("letterSpacing" in g) g.letterSpacing = v; };
  const fit = (t, w, size, font) => { let fs = size; g.font = font(fs); while (fs > 14 && g.measureText(t).width > w){ fs--; g.font = font(fs); } return fs; };
  g.clearRect(0, 0, W, H); g.fillStyle = "#06070a"; g.fillRect(0, 0, W, H);
  // 使い始めたばかりでも光が端に寄らないよう、画像は記録のある日から（短ければ 4 週ぶん）だけを写す
  const {c0, last} = yrRange(x);
  exposure(g, PX, PT, PW, PB - PT, x, 2, false, c0, last);
  // 目盛り：下に月、左の余白に時刻（6・12・18・0・6 時）。どこが何月・何時かを画像だけで読めるように
  g.save(); g.fillStyle = ink2; g.globalAlpha = .75; g.font = `500 14px ${FONT.mono}`; g.textBaseline = "alphabetic"; g.textAlign = "left";
  for (let mo = 0; mo < 12; mo++){
    const c = Math.round((new Date(x.y, mo, 1) - new Date(x.y, 0, 1))/864e5);
    if (c < c0 || c >= last) continue;
    const px = PX + (c - c0)*PW/(last - c0);
    g.fillRect(px, PB + 4, 1, 8); if (px < PX + PW - 40) g.fillText(MON[mo].toUpperCase(), px + 5, PB + 22);
  }
  // 範囲に月の初めがなければ（1 か月の中に収まる）、最初の日を書く
  if (!Array.from({length: 12}, (_, mo) => Math.round((new Date(x.y, mo, 1) - new Date(x.y, 0, 1))/864e5)).some(c => c >= c0 && c < last))
    g.fillText(dMD(new Date(x.y, 0, 1 + c0)).toUpperCase(), PX + 5, PB + 22);
  g.textAlign = "right";
  [["6", 0], ["12", .25], ["18", .5], ["0", .75], ["6", 1]].forEach(([h, f]) => { const py = PT + f*(PB - PT); g.fillRect(PX - 8, py, 6, 1); g.fillText(h, PX - 12, py + 5); });
  g.restore();
  // 画像だけを見た人にも読めるよう、光の読み方を添える
  g.textAlign = "left"; g.textBaseline = "alphabetic"; g.fillStyle = ink2; g.globalAlpha = .8; g.font = `500 16px ${FONT.mono}`; ls("2px");
  g.fillText("EACH STREAK = A STRETCH OF ACTIVE TIME   ·   ACROSS: DATE   ·   DOWN: 6:00 → 6:00", PX, 412); ls("0px"); g.globalAlpha = 1;
  // 見どころ：光の上端に括弧（期間）か目印（1 日）を付け、見出しはその上の帯に置く。重なる見出しは上の段に上げ、2 段とも埋まっていれば書かない
  const cw = PW/(last - c0), cx0 = c => PX + (c - c0)*cw;
  const rows = [[], []]; x.drawn = []; // 実際に描いた見どころ（読み上げ用の説明に使う）
  cardMarks(x, o).filter(h => h.c1 >= c0 && h.c0 < last).slice(0, 3).sort((p, q) => p.c0 - q.c0).forEach(h => {
    const xa = Math.max(PX, cx0(h.c0)), xb = Math.min(PX + PW, cx0(h.c1 + 1));
    g.font = `600 19px ${FONT.sans}`; const tw = g.measureText(h.img).width;
    const tx = Math.max(PX, Math.min(PX + PW - tw, (xa + xb)/2 - tw/2));
    const r = rows.findIndex(row => row.every(([p, q]) => tx > q + 24 || tx + tw < p - 24));
    if (r < 0) return; // 2 段とも重なるなら書かない（画面の What stands out には出ている）
    rows[r].push([tx, tx + tw]); x.drawn.push(h.img);
    const ty = PT - 20 - r*28, ly = PT - 8;
    g.strokeStyle = amber; g.globalAlpha = .9; g.lineWidth = 2;
    g.beginPath();
    if (xb - xa > 6){ g.moveTo(xa + 1, ly + 8); g.lineTo(xa + 1, ly); g.lineTo(xb - 1, ly); g.lineTo(xb - 1, ly + 8); }
    else { g.moveTo((xa + xb)/2, ly); g.lineTo((xa + xb)/2, ly + 14); }
    g.stroke(); g.globalAlpha = 1;
    g.fillStyle = amber; g.textAlign = "left"; g.fillText(h.img, tx, ty);
  });
  // 見出し：期間はいつも日付で書く（写した範囲）
  g.textAlign = "left"; g.textBaseline = "alphabetic";
  g.fillStyle = ink2; g.font = `500 22px ${FONT.mono}`; ls("4px");
  g.fillText(`KIROKU — ${dSpan(new Date(x.y, 0, 1 + c0), new Date(x.y, 0, last), true).toUpperCase()}`, PX, 90); ls("0px");
  const title = yrTitle(x);
  g.fillStyle = ink; fit(title, 760, 64, fs => `700 ${fs}px ${FONT.mincho}`); g.fillText(title, PX, 172);
  // 合計は小さく（主役は光。人と比べる数字にしない）。時間が何を数えたかを、すぐ下に書く
  if (o.totals){
    const hrs = x.active/60, items = [[commas(hrs >= 10 ? Math.round(hrs) : Math.round(hrs*10)/10), "hours with a session active"], [commas(x.sessions), "sessions"]];
    if (x.commits) items.push([commas(x.commits), "commits"]); if (x.prs) items.push([commas(x.prs), "PRs"]);
    let cx = PX;
    items.forEach(([t, l], i) => { if (cx > 820) return; g.font = `500 26px ${FONT.mono}`; g.fillStyle = ink; g.fillText(t, cx, 232); cx += g.measureText(t).width + 10;
      g.font = `500 22px ${FONT.sans}`; g.fillStyle = ink2; g.fillText(l, cx, 232); cx += g.measureText(l).width + (i < items.length - 1 ? 34 : 0); });
    g.font = `500 18px ${FONT.sans}`; g.fillStyle = ink2; g.globalAlpha = .8;
    g.fillText("Hours count parallel sessions once and include time the AI ran on its own.", PX, 266); g.globalAlpha = 1;
  }
  if (o.type && L){
    g.textAlign = "right"; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("YOUR LIGHT", W - PX, 90); ls("0px");
    g.shadowColor = "rgba(255,174,87,.5)"; g.shadowBlur = 36; g.fillStyle = amber; g.font = `800 60px ${FONT.mincho}`; g.fillText(L.en, W - PX, 156);
    g.shadowBlur = 0; g.fillStyle = ink2; g.font = `500 22px ${FONT.sans}`; g.fillText(L.sub, W - PX, 194);
    g.textAlign = "left";
  }
  // 右の列：前半と比べた後半（大きく動いたものだけ）と、光の上に書き込めない見どころ。合わせて 3 行まで。行は左の見出しにかからない幅 640 に収める
  const side = sideLines(x, o);
  if (side.length){
    const y0 = o.type && L ? 250 : 90;
    g.textAlign = "right"; g.fillStyle = ink2; g.font = `500 20px ${FONT.mono}`; ls("4px"); g.fillText("WHAT STANDS OUT", W - PX, y0); ls("0px");
    g.fillStyle = ink;
    side.forEach((t, i) => { fit(t, 640, 24, fs => `500 ${fs}px ${FONT.sans}`); g.fillText(t, W - PX, y0 + 40 + i*34); });
    g.textAlign = "left";
  }
  // いちばん下：左に凡例、右に kiroku の場所（画像だけが回ってきても、何で作ったかがわかるように）
  g.font = `500 18px ${FONT.mono}`; ls("1px"); const foot = `made with kiroku · ${REPO.replace(/^https:\/\//, "")}`, fw = g.measureText(foot).width;
  g.fillStyle = ink2; g.textAlign = "right"; g.fillText(foot, W - PX, 864); ls("0px"); g.textAlign = "left";
  const room = W - PX - fw - 48;
  if (o.agents && x.mode === "project"){ // プロジェクトの名前は載せない。色の数だけを示す
    let lx = PX; x.projects.slice(0, 8).forEach(p => { g.fillStyle = p.c; g.shadowColor = p.c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8, 856, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0; lx += 26; });
    g.font = `500 22px ${FONT.sans}`; g.fillStyle = ink; g.fillText(`Colored by project · ${plural(x.projects.length, "project")}`, lx + 10, 864);
  } else if (o.agents){
    let lx = PX; g.font = `500 22px ${FONT.sans}`;
    // 入りきらないエージェントは「Other」にまとめる（色の見えている線に、凡例がないことがないように）
    // 「of time」は最初の 1 つにだけ付ける。入りきらないものは「Other」にまとめ、まとめたエージェントの色の丸を並べる
    const shown = [], rest = [...x.agents], dots = n => 26 + (n - 1)*14;
    while (rest.length){ const a = rest[0], t = `${a.k} ${Math.round(a.pct)}%${shown.length ? "" : " of time"}`, w = dots(1) + g.measureText(t).width + 32;
      const left = rest.length > 1 ? dots(Math.min(3, rest.length - 1)) + g.measureText("Other 100%").width : 0;
      if (shown.length >= 4 || lx + w + left > room) break; shown.push([[a.c], t]); lx += w; rest.shift(); }
    if (rest.length) shown.push([rest.slice(0, 3).map(a => a.c), `Other ${Math.round(rest.reduce((v, a) => v + a.pct, 0))}%`]);
    lx = PX; shown.forEach(([cs, t]) => { cs.forEach((c, i) => { g.fillStyle = c; g.shadowColor = c; g.shadowBlur = 14; g.beginPath(); g.arc(lx + 8 + i*14, 856, 8, 0, Math.PI*2); g.fill(); g.shadowBlur = 0; });
      g.fillStyle = ink; g.fillText(t, lx + dots(cs.length), 864); lx += dots(cs.length) + g.measureText(t).width + 32; });
  }
}
/* 画像を描き、描いたものを読み上げ用の説明にする */
function drawCardLabeled(){ drawCard(); const c = $("#yrcard"); if (c && yr.x) c.setAttribute("aria-label", cardLabel(yr.x, yr.L, yr.opt)); }
/* 画像に載っているものを、読み上げ用の文にする */
function cardLabel(x, L, o){
  const parts = [yrTitle(x).replace(/\.$/, "")];
  if (o.totals) parts.push(`${dur(x.active)} with a session active, ${plural(x.sessions, "session")}`);
  if (o.type && L) parts.push(`Your light: ${L.en}, ${L.sub}`);
  const m = [...(x.drawn || []), ...sideLines(x, o)];
  if (m.length) parts.push(`What stands out: ${m.join("; ")}`);
  return `Image to share. ${parts.join(". ")}`;
}
function renderYear(){
  const ys = yearsOf(), dlg = $("#yr"); if (!ys.length) return;
  if (!ys.includes(yr.y)) yr.y = ys.includes(today0().getFullYear()) ? today0().getFullYear() : ys[ys.length-1];
  const x = yr.x = yearData(yr.y), L = yr.L = x.short ? null : LIGHTS.find(l => l.t(x)), pt = x.partial; // 8 週に満たなければ光の名前は付けない
  x.mode = yrColorMode(x); x.colorFn = yrColorOf(x, x.mode);
  const keys = x.mode === "project" ? x.projects : x.agents, qs = x.halves.filter(h => h.q);
  const R = yrRange(x), colOf = d => Math.round((d - new Date(x.y, 0, 1))/864e5), pos = c => ((c - R.c0)/(R.last - R.c0)*100).toFixed(2);
  let months = Array.from({length: 12}, (_, i) => colOf(new Date(x.y, i, 1))).map((c, i) => c >= R.c0 && c < R.last ? `<span style="left:${pos(c)}%">${MON[i]}</span>` : "").join("");
  if (!months) months = `<span style="left:0">${dMD(new Date(x.y, 0, 1 + R.c0))}</span>`; // 1 か月の中に収まるなら、最初の日を書く
  const from = new Date(x.y, 0, 1 + R.c0);
  const hours = [["6", 0], ["12", 25], ["18", 50], ["0", 75], ["6", 100]].map(([h, t]) => `<span style="top:${t}%">${`${h}:00`}</span>`).join("");
  const ev = L ? [L.m, ...["morning", "avg", "per"].filter(k => k !== L.m)].slice(0, 3).map(k => lightMetric(k, x)) : [];
  const asks = [...qs.map(h => h.q), ...(L ? [L.ask] : [])];
  const vt = t => esc(t);
  // その年に何も変えない項目は押せなくし、理由を添える
  const name = l => l.en, opt = (k, l, off) => `<label${off ? ` class="off" title="${off}"` : ""}><input type="checkbox" data-o="${k}"${yr.opt[k] && !off ? " checked" : ""}${off ? " disabled" : ""}>${l}${off ? ` <small>(${off})</small>` : ""}</label>`;
  const noLate = !x.hi.some(h => h.k === "late" || h.k === "weekend") && !x.halves.some(h => h.k === "late");
  const noBreak = !x.hi.some(h => h.k === "break" && h.c1 >= R.c0 && h.c0 < R.last);
  dlg.innerHTML = `<div class="yrhd"><div><div class="eyebrow">Year in review</div><h2 id="yrh">${`${x.y} exposure`}</h2></div>
    <label><span class="sr">Year</span><select id="yrsel">${ys.map(v => `<option value="${v}"${v === x.y ? " selected" : ""}>${v}</option>`).join("")}</select></label>
    <form method="dialog"><button class="iconbtn" aria-label="Close"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form></div>
  <p class="yrlead">Your active time with AI, drawn like a year-long exposure. Across is the date, down is the time of day (6:00 to 6:00 the next morning). Each stretch of active time is a streak of light, brighter where sessions overlapped. Shown: ${esc(dSpan(from, new Date(x.y, 0, R.last), true))}${R.c0 && R.c0 === x.c0 ? " (from the first day with history)" : ""}.</p>
  <div class="yrscroll" tabindex="0" role="group" aria-label="Exposure chart, scrolls sideways on narrow screens"><div class="yrchart"><div class="yrax" aria-hidden="true">${hours}</div><canvas class="yrplate" id="yrplate" role="img" aria-label="${esc(`Active time in ${x.y} by date and time of day. Active time ${dur(x.active)}`)}"></canvas><div class="yrx" aria-hidden="true">${months}</div></div></div>
  <div class="yrleg">${keys.map(a => `<span><i style="background:${a.c}"></i>${esc((a.k))} ${Math.round(a.pct)}%</span>`).join("")}</div>
  <div class="yrhow"><div><b>Length = how long a session was active</b>Each streak is one stretch of active time in a session, estimated from the timestamps in the history. It includes time the AI ran on its own.</div>
    ${x.mode === "project" ? `<div><b>Color = project</b>Shifts in color show when your time moved from one project to another. The image to share shows the colors, never the names.</div>` : `<div><b>Color = agent</b>Colors that start to mix show when you began using more than one.</div>`}
    <div><b>Dark bands = days without AI sessions</b>Days off, holidays and days spent on other work all look the same.</div></div>
  ${x.hi.length || x.halves.length ? `<h3>What stands out</h3>
  <ul class="yrhi">${x.hi.map(h => `<li>${esc(h.t)}</li>`).join("")}${x.halves.map(h => `<li>Second half vs first: ${esc(h.t)}</li>`).join("")}</ul>
  ${x.halves.length ? `<p class="note">"Second half vs first" splits the dates you have history for in two. "2+ sessions at once" is the share of active time when two or more sessions were running.</p>` : ""}` : ""}
  ${asks.length ? `<h3>To reflect on</h3>
  <ul class="yrhi">${asks.map(q => `<li>${esc(q)}</li>`).join("")}</ul>
  <p class="note">Questions, not judgments. They stay on this screen: they are never saved, sent or put on the image. To dig into a month, copy its report prompt from the month view.</p>` : ""}

  <h3>An image to share</h3>
  <div class="yropts" role="group" aria-label="On the image">${opt("totals", "Totals (hours, sessions, commits, PRs)")}${opt("type", "Your light", L ? "" : "needs 8 weeks of history")}${opt("agents", "Color key")}${opt("late", "Late nights and weekends", noLate ? "nothing to show this year" : "")}${opt("breaks", "Longest stretch without AI", noBreak ? "no stretch of 7 days or more" : "")}
    <label>Color by <select id="yrcolor">${[["auto", "Auto"], ["agent", "Agent"], ["project", "Project"]].map(([v, l]) => `<option value="${v}"${yr.opt.color === v ? " selected" : ""}>${l}</option>`).join("")}</select></label></div>
  <canvas class="yrcard" id="yrcard" width="1600" height="900" role="img"></canvas>
  <div class="yract"><button class="pill" id="yrsave">Save as PNG</button>${copyBtn("Copy image", `id="yrcopy"`)}</div>
  <p class="note">The image shows the date range, the streaks of light, what stands out, kiroku's address, and only what is ticked above. The longest stretch without AI has no dates written on it. Prompts, project names, branches, files and estimated cost are never included. It is made in this browser and sent nowhere.</p>

  <h3>Your light</h3>
  ${L ? `<div class="yrtype"><div class="yrtn"><span>${vt(name(L))}</span></div><div>
    <p class="yrtd">${L.de}</p>
    <dl class="yrev">${ev.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join("")}</dl>
    <p class="note">${`Your light describes the shape of your year in photography terms. It is not a verdict, and its thresholds are rough guides. Why: ${L.re}`}${x.active < 600 ? " (based on little history, so take it lightly)" : ""}</p>
    <details class="yrtypes"><summary>All ${LIGHTS.length} and how they are chosen</summary><ul>${LIGHTS.map(l => `<li><b>${name(l)}</b> — ${l.re}</li>`).join("")}</ul>
      <p class="note">The first one that matches, from the top, is chosen.</p></details></div></div>` : `<p class="note">Your light is named once you have 8 weeks of history. Until then there is too little to say.</p>`}`;
  $("#yrsel").onchange = e => { yr.y = +e.target.value; renderYear(); $("#yrsel").focus(); }; // 描き直しても、年の選択にフォーカスを残す
  $("#yrcolor").onchange = e => { yr.opt.color = e.target.value; store.set("yrOpt", yr.opt); renderYear(); };
  dlg.querySelectorAll("[data-o]").forEach(c => c.onchange = () => { yr.opt[c.dataset.o] = c.checked; store.set("yrOpt", yr.opt); drawCardLabeled(); });
  const blob = () => new Promise(res => $("#yrcard").toBlob(res, "image/png"));
  $("#yrsave").onclick = async () => { const b = await blob(); if (!b) return toast("Couldn't make the image");
    const a = document.createElement("a"); a.href = URL.createObjectURL(b); a.download = `kiroku-${x.y}.png`; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000); toast("Saved the PNG"); };
  $("#yrcopy").onclick = async () => { try { await navigator.clipboard.write([new ClipboardItem({"image/png": blob()})]); toast("Copied the image"); copied($("#yrcopy")); }
    catch(e){ toast("Couldn't copy the image. Save it as a PNG instead", 2600); } };
  drawPlate(); drawCardLabeled();
  const sc = dlg.querySelector(".yrscroll"); sc.scrollLeft = sc.scrollWidth; // 狭い画面では、新しい記録の側を見せる
}
function openYear(){ const d = $("#yr"); if (!d.open) d.showModal(); renderYear(); }

