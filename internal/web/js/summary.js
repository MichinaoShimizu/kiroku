// 週次・月次サマリー（プロジェクト別・配分）と、指標の説明 HELP
/* ── 週次・月次サマリー：① プロジェクト別 ② コストとアウトプット ③ 時間 ④ AI ⑤ かたち ⑥ 改善案を聞く。基準を超えた指標には、その場に印と推移を付ける ── */
function summary(){
  const {S:w, P:pw} = period(), R = $("#review"), M = st.mode === "month", unit = M ? "月" : "週";
  const head = `<div class="rvhead"><h2>${M ? "Monthly summary" : "Weekly summary"}</h2><p>Rough figures for reflecting on how you work. They are not for comparing people or for evaluations.</p>${w ? `<button class="pill rpt" id="rpttog" aria-expanded="${!!st.rpt}" aria-controls="rptbox">${`${M ? "Monthly" : "Weekly"} report draft`}</button><button class="pill rpt" id="pxtog" aria-expanded="${!!st.px}" aria-controls="pxbox">Export prompts</button>` : ""}</div>${w ? `<section class="panel rptbox" id="pxbox"${st.px ? "" : " hidden"} aria-label="Export prompts">
    <div class="askbar"><button class="pill" id="pxcopy">Copy</button><button class="pill" id="pxclose">Close</button>
      <span class="muted" style="font-size:var(--fs-xs)">${`Markdown with only the user prompts ${uThis(unit)} (commands included), by session in time order. Automatic notifications and summaries are left out.`}</span></div>
    <pre class="askpre" id="pxpre">${esc(periodPrompts())}</pre></section>` : ""}${w ? `<section class="panel rptbox" id="rptbox"${st.rpt ? "" : " hidden"} aria-label="${`${M ? "Monthly" : "Weekly"} report draft`}">
    <div class="askbar"><button class="pill" id="rptcopy">Copy</button><button class="pill" id="rptclose">Close</button>
      <span class="muted" style="font-size:var(--fs-xs)">Markdown with what you did, commits and pull requests for each project. Session names are the start of your prompts, so edit them before pasting.</span></div>
    <pre class="askpre" id="rptpre">${esc(reportText(w, M))}</pre></section>` : ""}`;
  if (!w){ R.innerHTML = `${head}<div class="rvgrid"><div class="panel"><p class="none">${`No records ${uThis(unit)}.`}</p>${foot()}</div></div>`; bindCopy(R); return; }
  const longest = Math.max(0, ...w.focus.map(b=>b.min));
  const stat = (k, v, s, h) => `<div class="stat"><div class="k">${k}${hb(h)}</div><div class="v">${v}</div>${s?`<div class="s">${s}</div>`:""}${hint(h)}</div>`;
  const ph = (n, t, s, h) => `<div class="ph"><span class="no">${n}</span><h3>${t}${hb(h)}</h3><span>${s}</span></div>${hint(h)}`;
  const pct = v => v == null ? "Unknown" : `${v}<small>%</small>`;
  const times = n => `${n}`;
  const F = findList(w, pw, unit);
  R.innerHTML = `${head}${keepNotice()}${flagSum(F)}<div class="rvgrid">
  ${projectPanel(w, ph, unit)}
  ${outcomePanel(w, pw, unit, ph, stat)}
  <section class="panel">${ph(3, "How you spent time", "When and how long sessions ran")}
    <div class="stats">
      ${stat("Focus blocks (60+ min)", times(w.focus.length), longest ? `Longest ${dur(longest)}` : "", "focus")}
      ${stat("Weekend", dur(w.weekend,true), "", "weekend")}
      ${stat("Total AI run time", dur(w.ai,true), "Includes parallel runs", "ai")}
      ${(() => { const {ws, we} = period(), H = limitHits(ws, we); return stat("Usage limit hits", times(H.length), H.length ? H.slice(-4).map(h => `${md(h.t)} ${hm(h.t)}`).join(", ") + (H.length > 4 ? ", …" : "") : "From Claude Code history", "limits"); })()}
    </div>
    <details class="moreS" id="moreS"${st.moreS ? " open" : ""}><summary>More metrics (includes estimates)</summary><div class="stats">
      ${stat("Prompts with corrections or interruptions", pct(w.fixRate), `n=${w.prompts}`, "fix")}
      ${(() => { const {ws, we} = period(), B = bigOf(ws, we); return stat("Oversized prompts", `${B.n}`, `${BIG_PROMPT.toLocaleString(LOC())}+ characters${B.n ? ` · longest ${B.max.toLocaleString(LOC())}` : ""}`, "bigPrompts"); })()}
      ${stat("Project switches per day", times(w.switchesAvg), `Max ${w.switchesMax}`, "switches")}
      ${stat("Parallel time", dur(w.parallel,true), `Up to ${w.maxConc} at once`, "parallel")}
      ${stat("Wait time (median)", secsH(w.waitMedian), `n=${w.waitCount} · 90th percentile ${secs(w.waitP90)}`, "wait")}
    </div></details>
    </section>
  <section class="panel">${ph(4, "How you used AI", "Cache, models and heavy sessions")}
    ${aiUsage(w, pw, unit) || `<p class="none">No token or credit records.</p>`}
    ${nativeSection(w)}</section>
  <section class="panel shape">${ph(5, M ? "Shape of the month" : "Shape of the week", "Focus and friction")}
    ${w.focus.length ? `<h3>Focus blocks${hb("focusList")}</h3>${hint("focusList")}` : ""}
    ${w.focus.length ? [...w.focus].sort((a,b)=>b.min-a.min).slice(0,5).map(b=>`<div class="focusrow" style="--c:${st.colorBy==="project"?colorOf(b.project):"var(--ink-3)"}"><span class="when">${md(b.t)} ${hm(b.t)}</span><span class="track2"><span style="width:${b.min/longest*100}%"></span></span><span class="len">${dur(b.min)}</span></div>`).join("") : ""}
    ${w.friction.length ? `<h3>Possible friction${hb("friction")}</h3>${hint("friction")}` : ""}
    ${w.friction.length ? w.friction.map(f=>`<button class="card" data-id="${esc(f.id)}"><span class="ti">${esc(f.title)}</span><span class="me">${md(f.start)} · ${esc(f.project)} ${whyOf(f).map(x=>`<span class="tagx">${esc(x)}</span>`).join("")}</span></button>`).join("") : ""}
    ${repeatsOf(period().ws, period().we).length ? `<h3>Repeated prompts${hb("repeats")}</h3>${hint("repeats")}` : ""}
    ${(() => { const {ws, we} = period(), RP = repeatsOf(ws, we); return RP.length ? RP.slice(0, 3).map(c => `<button class="card" data-id="${esc(c.id)}"><span class="ti">${esc(snipOf(c.text, 90))}</span><span class="me">${`${c.n} times in ${c.ids.size} sessions · last on ${md(c.last)}`}</span></button>`).join("") : ""; })()}
    ${w.focus.length || w.friction.length || repeatsOf(period().ws, period().we).length ? "" : `<p class="none">No work of 60+ minutes, no sessions with possible friction and no repeated prompts.</p>`}</section>
  <section class="panel ask">${ph(6, "Ask AI for suggestions", "A prompt that asks for suggestions based on this data")}
    <div class="askbar"><button class="pill" id="askcopy">Copy prompt</button>
      <span class="muted" style="font-size:var(--fs-xs)">Paste it into the AI agent you use. kiroku never calls an AI. It includes session names (parts of your prompts) and project names, so review it before sending.</span></div>
    <pre class="askpre" id="askpre">${esc(askPrompt(w, pw, M))}</pre></section>
  <section class="panel metap">${measure(w)}${foot()}</section></div>`;
  placeFlags(R, F);
  R.querySelectorAll(".card,.pses,.fses").forEach(c => c.onclick = () => select(c.dataset.id));
  R.querySelectorAll("[data-goto]").forEach(b => b.onclick = () => { // 見直す候補から、印の付いた指標へ移動して説明を開く
    const t = R.querySelector(`.panel .hb[data-help="${b.dataset.goto}"]`) || R.querySelector(`.panel.${b.dataset.goto}`); if (!t) return;
    const dt = t.closest("details"); if (dt && !dt.open){ dt.open = true; st.moreS = true; }
    t.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "center"});
    if (t.classList.contains("hb") && t.getAttribute("aria-expanded") !== "true") t.click();
    if (!t.matches("button")) t.tabIndex = -1; t.focus({preventScroll: true}); }); // フォーカスも移す（画面の外に残さない）
  const more = R.querySelector("#pmore"); if (more) more.onclick = () => { st.allProj = !st.allProj; summary(); };
  const ko = R.querySelector("#keepoff"); if (ko) ko.onclick = () => { store.set("keepNoticeOff", true); summary(); };
  const ka = R.querySelector("#keeparch"); if (ka) ka.onclick = () => { ka.disabled = true; keepArchive(); };
  const ms = R.querySelector("#moreS"); if (ms) ms.ontoggle = () => { if (ms.dataset.auto){ delete ms.dataset.auto; return; } st.moreS = ms.open; }; // 印のために自動で開いたときは、次の期間に持ち越さない
  const rt = R.querySelector("#rpttog"), rb = R.querySelector("#rptbox"), rpt = open => { st.rpt = open; rb.hidden = !open; rt.setAttribute("aria-expanded", open); if (open) rb.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest"}); else rt.focus(); };
  if (rt && rb){ rt.onclick = () => rpt(!st.rpt); R.querySelector("#rptclose").onclick = () => rpt(false); }
  const xt = R.querySelector("#pxtog"), xb = R.querySelector("#pxbox"), px = open => { st.px = open; xb.hidden = !open; xt.setAttribute("aria-expanded", open); if (open) xb.scrollIntoView({behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest"}); else xt.focus(); };
  if (xt && xb){ xt.onclick = () => px(!st.px); R.querySelector("#pxclose").onclick = () => px(false); R.querySelector("#pxcopy").onclick = () => copy($("#pxpre").textContent); }
  const rc = R.querySelector("#rptcopy"); if (rc) rc.onclick = () => copy($("#rptpre").textContent, "Copied. Session names are the start of your prompts, so edit them before pasting", 4500);

  const ac = R.querySelector("#askcopy"); if (ac) ac.onclick = () => copy($("#askpre").textContent);
  R.querySelectorAll("#useBy button").forEach(b => b.onclick = () => { st.use = b.dataset.v; store.set("use", st.use); summary(); });
  bindCopy(R); bindHelp(R);
}
/* 期間の言い方（unit は "週" か "月"） */
const uThis = unit => unit === "月" ? "this month" : "this week";
const uLast = unit => unit === "月" ? "last month" : "last week";
const uNext = unit => unit === "月" ? "next month" : "next week";
const whyOf = f => f.whyEn && f.whyEn.length === f.why.length ? f.whyEn : f.why; // こじれた理由（Go の英語の文）
/* 配分：作業時間・トークン・目安コスト・クレジットのそれぞれで、何割をどのくくりに使ったか。くくりは色分け（プロジェクト・ブランチ・エージェント）に合わせる。
   指標を縦に並べ、「時間の割にコストが多い」のようなずれを見比べられるようにする（上位 5 つ＋その他） */
function shareBlock(w, ps){
  const all = st.colorBy === "project" ? ps.map(p => ({key: p.project, minutes: p.minutes, tokens: p.tokens, cost: p.cost, credits: p.credits})) : ((w.shares || {})[st.colorBy] || []);
  const head = `<h3 class="shh">${`Share by ${CB()[st.colorBy].toLowerCase()}`}</h3><p class="shn">Follows the Project / Branch / Agent switch at the top</p>`;
  if (!all.length) return "";
  if (all.length < 2) return head + `<p class="none">${`Only "${esc(all[0].key)}" this period.`}</p>`;
  const N = 5, top = all.slice(0, N), rest = all.slice(N);
  const rows = top.map(p => ({name: p.key, c: colorOf(p.key), v: p}));
  if (rest.length) rows.push({name: `Other (${rest.length})`, c: "var(--other)", other: true,
    v: rest.reduce((a, p) => ({minutes: a.minutes + p.minutes, tokens: a.tokens + p.tokens, cost: a.cost + p.cost, credits: a.credits + p.credits}), {minutes: 0, tokens: 0, cost: 0, credits: 0})});
  const ms = [["minutes", "Active time", dur], ["tokens", "Tokens", tok], ["cost", "Est. cost", usd], ["credits", "Credits", cr]]
    .map(([k, n, f]) => ({k, n, f, t: rows.reduce((t, r) => t + (r.v[k] || 0), 0)})).filter(m => m.t > 0);
  if (!ms.length) return "";
  const pc = (r, m) => Math.round((r.v[m.k] || 0) * 100 / m.t);
  const bars = ms.map(m => `<span class="k">${m.n}</span><div class="stack" role="img" aria-label="${esc(`${m.n}: ${rows.map(r => `${r.name} ${pc(r, m)}%`).join(", ")}`)}">${rows.filter(r => r.v[m.k] > 0).map(r => `<span style="flex:${r.v[m.k]};--c:${r.c}" ${tipAttr(r.name, `${m.n} ${m.f(r.v[m.k])}${` (${pc(r, m)}%)`}`)}></span>`).join("")}</div>`).join("");
  const tab = `<table class="shtab"><thead><tr><th scope="col">${CB()[st.colorBy]}</th>${ms.map(m => `<th scope="col">${m.n}</th>`).join("")}</tr></thead><tbody>${rows.map(r => `<tr style="--c:${r.c}"><td title="${esc(r.name)}"><i aria-hidden="true"></i>${esc(r.name)}</td>${ms.map(m => `<td>${r.v[m.k] ? pc(r, m) + "%" : "—"}</td>`).join("")}</tr>`).join("")}</tbody></table>`;
  return `${head}<div class="sharebox"><div class="share">${bars}</div>${tab}</div>`;
}
/* プロジェクト別：何に何時間、どれだけのトークン・コスト・クレジットを、どのモデル中心に使い、何が重かったか */
function projectPanel(w, ph, unit){
  const ps = w.projectStats || []; if (!ps.length) return "";
  const total = w.projects.reduce((t,[,v])=>t+v,0) || 1, LIMIT = 6, shown = st.allProj ? ps : ps.slice(0, LIMIT);
  const use = x => [x.cost >= 0.005 ? usd(x.cost) : "", x.credits ? cr(x.credits) : "", !x.cost && x.tokens ? tok(x.tokens)+" tok" : ""].filter(Boolean).join(" · ");
  const anyCr = ps.some(p => p.credits); // この期間にクレジットの記録がまったくなければ、「—」だけの欄は出さない
  const cards = shown.map(p => {
    const c = st.colorBy === "project" ? colorOf(p.project) : "var(--ink-3)", share = Math.round(p.minutes*100/total);
    const mt = p.models.reduce((t,m)=>t+m.tokens,0) || 1; // トークンのないモデル（Kiro など）は回数で出す
    return `<article class="pcard" style="--c:${c}">
      <div class="hd"><i></i><b title="${esc(p.project)}">${esc(p.project)}</b><span>${p.minutes ? dur(p.minutes) : "—"}${p.minutes ? ` · ${share}%` : ""}</span></div>
      <div class="bar2"><span style="width:${share}%"></span></div>
      <div class="kv"><div><div class="k">Sessions / prompts</div><div class="v">${p.sessions}/${p.prompts}</div></div>
        <div><div class="k">Tokens</div><div class="v">${p.tokens ? tok(p.tokens) : "—"}</div></div>
        <div><div class="k">Est. cost</div><div class="v">${p.cost >= 0.005 ? usd(p.cost) : "—"}</div></div>
        ${anyCr ? `<div><div class="k">Credits</div><div class="v">${p.credits ? cr(p.credits) : "—"}</div></div>` : ""}</div>
      ${p.git && p.git.commits ? `<div><div class="lab">Outputs</div><div class="pout">Git commits ${p.git.commits} · ${p.git.ai} by AI · <span>+${p.git.added} −${p.git.removed} lines</span></div></div>` : p.outputs && p.outputs.commits ? `<div><div class="lab">Outputs</div><div class="pout">AI commits ${p.outputs.commits}</div></div>` : ""}
      ${p.models.length ? `<div><div class="lab">Main models</div><div class="mods">${p.models.map(m=>`<span title="${esc((m.model))}">${esc((m.model))}<b>${m.tokens ? Math.round(m.tokens*100/mt)+"%" : plural(m.turns, "turn")}</b></span>`).join("")}</div></div>` : ""}
      <div><div class="lab">Top sessions</div>${p.top.map(t=>`<button class="pses" data-id="${esc(t.id)}"><span class="t">${esc(t.title)}</span><span class="m">${dur(t.minutes)}${use(t) ? " · "+use(t) : ""}</span></button>`).join("")}</div>
    </article>`; }).join("");
  return `<section class="panel projs">${ph(1, "By project", `Where your time and AI went ${uThis(unit)}`, "projects")}
    ${shareBlock(w, ps)}
    <div class="pgrid">${cards}</div>
    ${ps.length > LIMIT ? `<button class="pill pmore" id="pmore">${st.allProj ? "Show top projects only" : `Show ${plural(ps.length - LIMIT, "more project")}`}</button>` : ""}</section>`;

}
/* 指標の読み方：定義・言えること・言えないこと・打てる手。画面の「?」と、改善案プロンプトの前提に使う */
const HELP = {
  findings: {n: "Worth a look", d: "Metrics that crossed a fixed threshold. Each is marked with a warning triangle where it appears, with what was observed, an 8-period trend, the related sessions and the threshold; their names are listed here in priority order", c: "Which metric is worth reviewing first", x: "Whether something is good or bad. Thresholds are generic and may not fit how you work", a: "Pick one, try it next period, and check the change with the same metric"},
  projects: {n: "By project", d: "Time, usage, main models and top sessions per project. Time when several projects ran at once is split between them", c: "How you divided time and AI across projects", x: "How important or successful a project was", a: "If the split differs from what you intended, revisit how you work or your priorities"},
  active: {n: "Active time", d: "Time when any session was running (overlaps count once)", c: "Total time you worked together with AI", x: "Whether you were focused. Includes time you left a session idle", a: "Compare with the previous period to see how much more or less you rely on AI"},
  ai: {n: "Total AI run time", d: "Session run time added up, including sessions running in parallel", c: "How much work you gave to AI. The gap from active time shows how much ran in parallel", x: "Time saved or productivity", a: "If it is close to active time, you could do other work while waiting"},
  focus: {n: "Focus blocks", d: "Work that continued for 60 minutes or more. Gaps within a session up to the session gap (15 minutes by default) and gaps of up to 5 minutes between sessions are treated as continuous", c: "Whether you had long stretches of uninterrupted work", x: "The quality of the work in that time", a: "If your time is fragmented, group the work you hand to AI into blocks of time"},
  focusList: {n: "Focus blocks (list)", d: "The 5 longest", c: "When and in which project you worked for long stretches", x: "Outcomes", a: "Learn when you focus best and schedule heavy work then"},
  switches: {n: "Project switches per day", d: "How often the project changed between consecutive prompts (daily average and maximum)", c: "How much you moved between projects", x: "Whether switching is bad (you may just be using wait time well)", a: "If high, next period limit each day to 2–3 projects"},
  parallel: {n: "Parallel time", d: "Time when 2 or more sessions ran at once, and the most at once", c: "Whether you kept several sessions going in parallel", x: "What the parallel work achieved", a: "If low, give AI another task while it works"},
  wait: {n: "Wait time", d: "Median and 90th percentile of the time from an AI reply to your next prompt (up to 30 minutes)", c: "How quickly you responded to AI replies", x: "Shorter is not always better (you may be moving on without checking)", a: "If long, use notifications or batch your reviews"},
  weekend: {n: "Weekend", d: "Work time on Saturdays and Sundays", c: "How much you worked on weekends", x: "Whether you are overworking", a: "If not intended, pick a day off for next period"},
  fix: {n: "Prompts with corrections or interruptions", d: "Share estimated from the opening words of each prompt (such as 'No, that's wrong' or 'undo that') and interruptions. The first prompt of a conversation and words inside pasted code or logs are not counted", c: "Roughly how often a first prompt did not get your intent across", x: "Estimates can be wrong, and they don't show the cause", a: "If high, add background, constraints and done criteria to your prompts"},
  limits: {n: "Usage limit hits", d: "Count and times of usage limit errors (usage caps and rate limits) in Claude Code history. Hits within 1 minute count once", c: "When and during which work you hit a limit and had to stop", x: "How much headroom is left. Usage from other agents, the browser or the app", a: "Spread heavy work over time, use lighter models, and start new sessions for long conversations"},
  longctx: {n: "Long conversations", d: "Sessions where the input read per response (new input plus cache reads and writes) in the last quarter of the conversation was at least 4 times that of the first quarter, peaking at 100K tokens or more (estimated cost $0.5 or more; only agents that record tokens)", c: "Sessions where each response got heavier as the conversation went on", x: "Whether continuing the conversation was the right call (some work needs the earlier context)", a: "At a good stopping point, write down the key points and continue in a new session"},
  modelfit: {n: "Expensive models for light work", d: "Total for sessions that mainly used Opus-class models, had 3 or fewer prompts, edited no files and cost $0.3 or more", c: "How much went to short tasks on an expensive model", x: "Whether that model was needed (some research or design questions are hard)", a: "Try a lighter model first for research and questions, and switch if it falls short"},
  friction: {n: "Sessions with possible friction", d: "Up to 3 sessions started in the period with at least one correction or interruption, or 15 or more prompts, most first", c: "Sessions where rework piled up", x: "Why it went wrong", a: "Next period, split big requests into one step per prompt"},
  bigPrompts: {n: "Oversized prompts", d: "Number of prompts of 4,000+ characters in the period, and the length of the longest", c: "Whether you hand over large inputs at once, such as pasting whole logs or documents", x: "Whether that length was needed (some long prompts, like design explanations, are fine)", a: "Next period, put long logs or documents in a file and write only its path and the part to look at"},
  repeats: {n: "Repeated prompts", d: "Prompts of 12+ characters in the period, grouped by how similar their text is; up to 3 written in 3 or more sessions, most first", c: "Routine requests you type every time", x: "Whether those requests worked well", a: "Next period, write the most repeated one once as a custom command or in CLAUDE.md"},
  daily: {n: "Daily trend", d: "A bar chart per day that switches between tokens, credits, estimated cost, active time, sessions and prompts", c: "Which days you worked the most and used AI the most", x: "Whether that day's usage was appropriate", a: "Open the sessions on outlier days to see why they were heavy"},
  cost: {n: "Estimated cost (API pricing)", d: "Usage priced at public API rates. Uses the cost Claude Code records for itself when available (this also covers price changes, new models and calls not in the history, such as auto-compaction); otherwise, and where it records 0 for a model it did use (as on a subscription), tokens in the history times the kiroku price table. Shown as — when none of the models used are in the price table", c: "A rough way to compare how heavy usage was, in money", x: "What you are actually billed (subscriptions differ)", a: "Open the sessions behind the increase, and next period keep that kind of work in shorter conversations"},
  projection: {n: "Month-end cost (estimate)", d: "In a month in progress, the estimated cost from the 1st through today, divided by the days so far and multiplied by the days in the month. Not shown for the first 7 days or on the last day", c: "Roughly where this month's cost is heading at the current pace", x: "Your actual bill, or how you will work from now on (it is off if the pace changes)", a: "If it is too high, look at the heavy sessions and models"},
  projectionCr: {n: "Month-end credits (estimate)", d: "In a month in progress, the Kiro credits from the 1st through today, divided by the days so far and multiplied by the days in the month. Counted separately from the estimated cost, because tokens and credits are separate allowances. Not shown for the first 7 days or on the last day", c: "Roughly how many credits this month is heading for at the current pace", x: "What your account page will show, or how you will work from now on (it is off if the pace changes)", a: "If it is heading past your limit, spread the heavy work out or move some of it to another agent"},
  tokens: {n: "Tokens", d: "Input, output, cache reads and cache writes combined", c: "How much you consumed", x: "Whether more or less is good", a: "Look for skew by project and by day"},
  cache: {n: "Share of input read from cache", d: "The share of input tokens read from cache", c: "Whether the same context was reused", x: "Why it is low (it may just be many short sessions)", a: "If low, next period write long background once in a project file (such as CLAUDE.md) instead of pasting it every time"},
  models: {n: "By model", d: "Estimated cost and tokens per model", c: "Which models your usage leaned toward", x: "Whether that model was needed", a: "Next period, try lighter models for routine work (formatting, renames, adding tests)"},
  subagents: {n: "Subagents", d: "Number of Task / Agent calls, their types and total run time", c: "Whether you delegated research and similar work", x: "How much delegating helped", a: "Check that you use them to save the main conversation's context"},
  credits: {n: "Kiro credits", d: "Total credits recorded in Kiro history", c: "Credits actually consumed", x: "Differences from your account page (period boundaries or use on other machines)", a: "Track your pace against your limit"},
  costPerAsk: {n: "Estimated cost per prompt", d: "Estimated cost ÷ number of prompts", c: "How heavy a typical prompt was", x: "Differences in prompt size", a: "Watch the trend to see how the size of your prompts changes"},
  heavy: {n: "Heaviest sessions", d: "The 3 sessions with the highest estimated cost", c: "Sessions that drove usage up", x: "Whether the result was worth it", a: "Next period, restart long conversations in a new session, and carry work to a commit in small steps"},
  prompts: {n: "Prompts", d: "How many prompts you sent in the period. Slash commands and ! shell commands count as prompts; notifications, system reminders, hook and command output, automatic summaries and instructions from other agents do not", c: "How much you had to put in by hand to get the work done", x: "The size or quality of each prompt, or how much thought went into it", a: "Read it next to active time: many prompts in little time often means steering turn by turn instead of handing over a whole task"},
  outputs: {n: "Outputs", d: "Reads left to right: Cost (your time and AI usage, from every agent), what it left behind (commits, lines, files, pushes and pull requests) and Compared (cost divided by what it left behind). What was left behind is the amount produced, counted from git and from history. Work that leaves no commit, such as research, review or design, does not appear here, and the by-AI share, cost per commit and sessions that reached a commit come from Claude Code only, the one agent whose history records the commits it ran", c: "Whether the cost turned into work that left a trace", x: "Value, quality or productivity. Work that leaves nothing in git", a: "Put the two sides next to each other and look for usage that left nothing"},
  gitCommits: {n: "Git commits", d: "Your own commits (user.email) in the repositories agents worked in, including ones made by hand. Each mark on the right edge of a day in the calendar is one commit (filled = run by AI; touch it to see the short hash). The share by AI is recognised by matching the times of commits an agent ran with a tool, which only Claude Code records for now", c: "How much of your time with AI became recorded changes", x: "The value of the changes. Work outside the repositories or commits by others", a: "On days with much time or cost but few commits, check where the time went"},
  lines: {n: "Lines changed", d: "Lines added and removed by those commits, as git counts them, with the average per commit. Generated files, lockfiles and moved code count the same as code you wrote", c: "Roughly how much the period produced, without depending on how often you commit", x: "The value or difficulty of the changes. A big number can be one generated file", a: "Read it next to the number of commits to see your usual commit size"},
  files: {n: "Files changed", d: "How many different files those commits touched. kiroku keeps up to 40 files per commit, so a period with larger commits shows a lower bound, marked as at least", c: "How widely the work spread", x: "How much of each file changed, or whether the files are related", a: "If few files took much cost, open those sessions to see what the time went into"},
  pushes: {n: "Pushes", d: "Pushes made from this computer, read from the local git reflog of remote-tracking branches (update by push). Pushes from other computers are not there, and git keeps its reflog for 90 days by default, so older ones are gone", c: "How often the work left your machine", x: "Whether it was reviewed or merged. kiroku never asks GitHub", a: "If commits pile up without pushes, push in smaller steps so the work is shared sooner"},
  prs: {n: "Pull requests", d: "Pull requests an agent created and whose result was recorded (gh pr create, or GitHub tools in Claude Code). Ones you opened in a browser, and ones created by agents that do not record outputs, are not counted", c: "How often work reached a request for review", x: "Whether they were merged, their size or their quality. kiroku never asks GitHub", a: "Compare with commits to see how much work is waiting to be shared"},
  commits: {n: "Commits", d: "Number of git commits that AI ran successfully", c: "Roughly how often work reached a checkpoint", x: "The value or size of the changes. Commit size varies by person and task", a: "In periods with few commits for the cost, check where the time went"},
  outSessions: {n: "Sessions that reached a commit", d: "Number and share of sessions that made a commit or pull request in the period, counting only Claude Code sessions, the only agent whose outputs are recorded", c: "The share of sessions that left something behind", x: "The value of sessions not meant to commit, such as research or discussion", a: "If low, next period state at the start of each session what done looks like (when to commit)"},
  costPerCommit: {n: "Estimated cost per commit", d: "Estimated cost of Claude Code sessions ÷ number of commits AI ran in them (only Claude Code records outputs, so other agents' cost is left out). Commits you made by hand are not counted", c: "Roughly how heavy it was to reach a checkpoint", x: "Differences in commit size, or work that went into commits you made by hand", a: "Next period, keep each prompt to one change and commit often"},
  native: {n: "Agent-specific metrics", d: "Numbers each agent records in its history", c: "Trends within the same agent", x: "Comparisons between agents (definitions differ)", a: "Only look at changes over time for the same agent"},
};
const H = () => HELP;
const openHelp = new Set(); // 再描画（週の移動・自動更新）しても開いた説明は開いたまま
function hb(id){ return H()[id] ? `<button class="hb" data-help="${id}" aria-label="${`How to read ${H()[id].n}`}" aria-expanded="${openHelp.has(id)}">?</button>` : ""; }
function hint(id){ const h = H()[id]; if (!h) return "";
  return `<div class="hint" data-hint="${id}"${openHelp.has(id) ? "" : " hidden"}><p>${esc(h.d)}</p><dl><dt>Tells you</dt><dd>${esc(h.c)}</dd><dt>Doesn't tell you</dt><dd>${esc(h.x)}</dd><dt>What to try</dt><dd>${esc(h.a)}</dd></dl></div>`; }

const helpOrder = []; // 説明を開いた順（Esc で新しいものから閉じる）。{id, b}
function bindHelp(root){ root.querySelectorAll(".hb").forEach(b => b.onclick = e => { e.stopPropagation();
  let t = null; // 同じ指標の説明が画面に 2 か所あるので、押したボタンにいちばん近いものを開く
  for (let a = b.parentElement; a && !t; a = a === root ? null : a.parentElement) t = a.querySelector(`[data-hint="${b.dataset.help}"]`);
  if (!t) return; t.hidden = !t.hidden; t.hidden ? openHelp.delete(b.dataset.help) : openHelp.add(b.dataset.help); b.setAttribute("aria-expanded", String(!t.hidden));
  const i = helpOrder.findIndex(x => x.id === b.dataset.help); if (i >= 0) helpOrder.splice(i, 1);
  if (!t.hidden) helpOrder.push({id: b.dataset.help, b}); }); }
function closeHint(){ // Esc：いちばん新しく開いた説明を閉じて、その ? へフォーカスを戻す。閉じたら true
  if (st.sel) return false; // 詳細を開いているときは、詳細を閉じるほう
  while (helpOrder.length){ const {id, b} = helpOrder[helpOrder.length-1];
    const btn = b.isConnected && b.getAttribute("aria-expanded") === "true" ? b : $(`#review .hb[data-help="${id}"][aria-expanded="true"]`); // 描き直していたら、同じ指標の ? を探す
    if (!btn || !openHelp.has(id)){ helpOrder.pop(); continue; }
    btn.click(); btn.focus(); return true; }
  return false; }
