// 週次・月次サマリー（プロジェクト別・配分）と、指標の説明 HELP
/* ── 週次・月次サマリー：① プロジェクト別 ② コストとアウトプット ③ 時間 ④ AI ⑤ かたち ⑥ 改善案を聞く。基準を超えた指標には、その場に印と推移を付ける ── */
function summary(){
  const {S:w, P:pw} = period(), R = $("#review"), M = st.mode === "month", unit = M ? "月" : "週";
  const head = `<div class="rvhead"><h2>${M ? "Monthly summary" : "Weekly summary"}</h2><p>Rough figures for reflecting on how you work. They are not for comparing people or for evaluations.</p>${w ? copyBtn(`Copy ${M ? "monthly" : "weekly"} report prompt`, `id="rptcopy" title="A prompt that asks an AI agent on this computer to write your ${M ? "monthly" : "weekly"} report from your history"`, "pill rpt") : ""}</div>${w ? `<p class="note rpthint">${`The ${M ? "monthly" : "weekly"} report prompt is for an AI agent on this computer, such as Claude Code or Codex. It points the agent to this ${M ? "month" : "week"}'s history files, and the agent reads them. kiroku itself sends nothing.`}</p>` : ""}`;
  const WK = $("#worth"); $("#sres").hidden = true; $("#sres").innerHTML = "";
  if (!w){ WK.hidden = true; R.innerHTML = foot(); return; } // 記録がないことは、上の数字とカレンダーで言っている（3 度言わない）
  const longest = Math.max(0, ...w.focus.map(b=>b.min));
  const pct = v => v == null ? "Unknown" : `${v}<small>%</small>`;
  const times = n => `${n}`;
  const F = findList(w, pw, unit);
  WK.innerHTML = flagSum(F); WK.hidden = false; lastFlags = F; // 見直す候補は、カレンダーの上（期間の要点のすぐ下）に出す。押すと真ん中のダイアログで開く
  R.innerHTML = `${head}<div class="rvgrid">
  ${projectPanel(w, unit)}
  ${outcomePanel(w, pw, unit)}
  <section class="panel time">${panelH(3, "How you spent time", "When and how long sessions ran")}
    <div class="stats">
      ${statH("Focus blocks (60+ min)", times(w.focus.length), longest ? `Longest ${dur(longest)}` : "", "focus", w.focus.length ? "focus" : "")}
      ${(() => { const {ws, we} = period(), H = limitHits(ws, we), N = unrecorded(ws, we, "limits"); return N ? norecStat("Usage limit hits", N, "limits") : statH("Usage limit hits", times(H.length), H.length ? H.slice(-4).map(h => `${md(h.t)} ${hm(h.t)}${h.r ? ` (resets ${esc(h.r)})` : ""}`).join(", ") + (H.length > 4 ? ", …" : "") : "From Claude Code and Codex history", "limits", H.length ? "limits" : ""); })()}
      ${(() => { const {ws, we} = period(), C = compactionsOf(ws, we), n = new Set(C.map(c => c.s.id)).size, N = unrecorded(ws, we, "compactions"); return N ? norecStat("Compactions", N, "compactions") : statH("Compactions", times(C.length), C.length ? `In ${plural(n, "session")} · ${C.slice(-4).map(c => `${md(c.t)} ${hm(c.t)}`).join(", ")}${C.length > 4 ? ", …" : ""}` : "From Claude Code, Codex and Amazon Q / Kiro CLI (SQLite) history", "compactions", C.length ? "compactions" : ""); })()}
    </div>
    ${secH("More metrics (includes estimates)")}<div class="stats">
      ${statH("Prompts with corrections or interruptions", pct(w.fixRate), `n=${w.prompts}`, "fix", w.fixRate ? "fix" : "")}
      ${(() => { const {ws, we} = period(), B = bigOf(ws, we); return statH("Oversized prompts", `${B.n}`, `${commas(BIG_PROMPT)}+ characters${B.n ? ` · longest ${commas(B.max)}` : ""}`, "bigPrompts", B.n ? "bigPrompts" : ""); })()}
      ${statH("Project switches per day", times(w.switchesAvg), `Max ${w.switchesMax}`, "switches", "switches")}
      ${statH("Parallel time", dur(w.parallel,true), `Up to ${w.maxConc} at once`, "parallel", w.parallel ? "parallel" : "")}
      ${statH("Wait time (median)", secsH(w.waitMedian), `n=${w.waitCount} · 90th percentile ${secs(w.waitP90)}`, "wait", w.waitCount ? "wait" : "")}
      ${statH("Total AI run time", dur(w.ai,true), "Includes parallel runs", "ai", "ai")}
    </div>
    </section>
  <section class="panel">${panelH(4, "How you used AI", "Cache, models and heavy sessions")}
    ${aiUsage(w, pw, unit) || noneH("No token or credit records.")}
    ${nativeSection(w)}</section>
  <section class="panel shape">${panelH(5, M ? "Shape of the month" : "Shape of the week", "Friction and repeated prompts")}
    ${w.friction.length ? secH("Possible friction", "friction") : ""}
    ${w.friction.length ? w.friction.map(f => sesCardH(sesById(f.id, f.title), `${sesMeta(f)} ${whyOf(f).map(x=>`<span class="tagx">${esc(x)}</span>`).join("")}`)).join("") : ""}
    ${repeatsOf(period().ws, period().we).length ? secH("Repeated prompts", "repeats") : ""}
    ${(() => { const {ws, we} = period(), RP = repeatsOf(ws, we); return RP.length ? RP.slice(0, 3).map(c => sesCardH(sesById(c.id, snipOf(c.text, 90)), `${c.n} times in ${c.ids.size} sessions ${uThis(unit)} · last on ${md(c.last)}`)).join("") : ""; })()}
    ${w.friction.length || repeatsOf(period().ws, period().we).length ? "" : noneH("No sessions with possible friction and no repeated prompts.")}</section>
  <section class="panel ask">${panelH(6, "Ask AI for suggestions", "A prompt that asks for suggestions based on this data")}
    <div class="askbar">${copyBtn("Copy prompt", `id="askcopy"`)}
      <span class="muted">Paste it into the AI agent you use. kiroku never calls an AI. It includes session names (parts of your prompts) and project names, so review it before sending.</span></div>
    <details class="askd"><summary>Show the prompt</summary><pre class="askpre" id="askpre">${esc(askPrompt(w, pw, M))}</pre></details></section>
  <section class="panel metap">${measure(w)}${foot()}</section></div>`;
  placeFlags(R, F);
  R.querySelectorAll(".card,.fses").forEach(c => c.onclick = () => select(c.dataset.id));
  WK.querySelectorAll("[data-goto]").forEach(b => { b.setAttribute("aria-haspopup", "dialog"); b.onclick = () => openWorth(b.dataset.goto, b); });
  R.querySelectorAll(".fmark").forEach(b => b.onclick = () => openWorth(b.dataset.goto, b));
  R.querySelectorAll("[data-metric]").forEach(b => b.onclick = () => openMetric(b.dataset.metric, b)); // 上の帯と同じ内訳
  const more = R.querySelector("#pmore"); if (more) more.onclick = () => { st.allProj = !st.allProj; summary(); };
  R.querySelectorAll(".keeparch").forEach(b => b.onclick = () => { R.querySelectorAll(".keeparch").forEach(x => x.disabled = true); keepArchive(); });
  const rc = R.querySelector("#rptcopy"); if (rc) rc.onclick = () => copy(reportPrompt(w, M), `Copied. Paste it into an AI agent on this computer. It includes your prompts and history file paths, and the agent reads those files, so check it before sending`, 8000, rc);

  const ac = R.querySelector("#askcopy"); if (ac) ac.onclick = () => copy($("#askpre").textContent, "", 0, ac);
  bindCopy(R); bindHelp(R); bindHelp(WK);
  if (hpopFor && !hpopFor.isConnected) hideHint(); // 描き直しで、説明を開いた ? が消えた
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
  if (all.length < 2) return head + noneH(`Only "${esc(all[0].key)}" this period.`);
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
/* プロジェクト別：何に何時間、どれだけのトークン・コスト・クレジットを使い、何が残ったか（モデルと重いセッションは ④ に） */
function projectPanel(w, unit){
  const ps = w.projectStats || []; if (!ps.length) return "";
  const total = w.projects.reduce((t,[,v])=>t+v,0) || 1, LIMIT = 6, shown = st.allProj ? ps : ps.slice(0, LIMIT);
  const anyCr = ps.some(p => p.credits); // この期間にクレジットの記録がまったくなければ、「—」だけの欄は出さない
  const cards = shown.map(p => {
    const c = st.colorBy === "project" ? colorOf(p.project) : "var(--ink-3)", share = Math.round(p.minutes*100/total);
    return `<article class="pcard" style="--c:${c}">
      <div class="hd"><i></i><b title="${esc(p.project)}">${esc(p.project)}</b><span>${p.minutes ? dur(p.minutes) : "—"}${p.minutes ? ` · ${share}%` : ""}</span></div>
      <div class="bar2"><span style="width:${share}%"></span></div>
      <div class="kv"><div><div class="k">Sessions / prompts</div><div class="v">${p.sessions}/${p.prompts}</div></div>
        <div><div class="k">Tokens</div><div class="v">${p.tokens ? tok(p.tokens) : "—"}</div></div>
        <div><div class="k">Est. cost</div><div class="v">${p.cost >= 0.005 ? usd(p.cost) : "—"}</div></div>
        ${anyCr ? `<div><div class="k">Credits</div><div class="v">${p.credits ? cr(p.credits) : "—"}</div></div>` : ""}</div>
      ${p.git && p.git.commits ? `<div><div class="lab">Left behind</div><div class="pout">Git commits ${p.git.commits} · ${p.git.ai} by AI · <span>+${p.git.added} −${p.git.removed} lines</span></div></div>` : p.outputs && p.outputs.commits ? `<div><div class="lab">Left behind</div><div class="pout">AI commits ${p.outputs.commits}</div></div>` : ""}
    </article>`; }).join("");
  return `<section class="panel projs">${panelH(1, "By project", `Where your time and AI went ${uThis(unit)}`, "projects")}
    ${shareBlock(w, ps)}
    <div class="pgrid">${cards}</div>
    ${ps.length > LIMIT ? `<button class="pill pmore" id="pmore">${st.allProj ? "Show top projects only" : `Show ${plural(ps.length - LIMIT, "more project")}`}</button>` : ""}</section>`;

}
/* 指標の読み方：定義・言えること・言えないこと・打てる手。画面の「?」と、改善案プロンプトの前提に使う */
const HELP = {
  findings: {n: "Worth a look", d: "Metrics that crossed a fixed threshold. Each is marked with a warning triangle where it appears, with what was observed, an 8-period trend, the related sessions and the threshold; their names are listed here in priority order", c: "Which metric is worth reviewing first", x: "Whether something is good or bad. Thresholds are generic and may not fit how you work", a: "Pick one, try it next period, and check the change with the same metric"},
  projects: {n: "By project", d: "Time, usage and what was left behind per project. Time when several projects ran at once is split between them", c: "How you divided time and AI across projects", x: "How important or successful a project was", a: "If the split differs from what you intended, revisit how you work or your priorities"},
  active: {n: "Active time", d: "Time when any session was running (overlaps count once)", c: "Total time you worked together with AI", x: "Whether you were focused. Includes time you left a session idle", a: "Compare with the previous period to see how much more or less you rely on AI"},
  ai: {n: "Total AI run time", d: "Session run time added up, including sessions running in parallel", c: "How much work you gave to AI. The gap from active time shows how much ran in parallel", x: "Time saved or productivity", a: "If it is close to active time, you could do other work while waiting"},
  focus: {n: "Focus blocks", d: "Work that continued for 60 minutes or more. Gaps within a session up to the session gap (15 minutes by default) and gaps of up to 5 minutes between sessions are treated as continuous", c: "Whether you had long stretches of uninterrupted work", x: "The quality of the work in that time", a: "If your time is fragmented, group the work you hand to AI into blocks of time"},
  switches: {n: "Project switches per day", d: "How often the project changed between consecutive prompts (daily average and maximum)", c: "How much you moved between projects", x: "Whether switching is bad (you may just be using wait time well)", a: "If high, next period limit each day to 2–3 projects"},
  parallel: {n: "Parallel time", d: "Time when 2 or more sessions ran at once, and the most at once", c: "Whether you kept several sessions going in parallel", x: "What the parallel work achieved", a: "If low, give AI another task while it works"},
  wait: {n: "Wait time", d: "Median and 90th percentile of the time from an AI reply to your next prompt (up to 30 minutes)", c: "How quickly you responded to AI replies", x: "Shorter is not always better (you may be moving on without checking)", a: "If long, use notifications or batch your reviews"},
  fix: {n: "Prompts with corrections or interruptions", d: "Share estimated from the opening words of each prompt (such as 'No, that's wrong' or 'undo that') and interruptions. The first prompt of a conversation and words inside pasted code or logs are not counted", c: "Roughly how often a first prompt did not get your intent across", x: "Estimates can be wrong, and they don't show the cause", a: "If high, add background, constraints and done criteria to your prompts"},
  limits: {n: "Usage limit hits", d: "Count and times of usage limit errors (usage caps and rate limits) in Claude Code and Codex history. Hits within 1 minute count once", c: "When and during which work you hit a limit and had to stop", x: "How much headroom is left. Usage from other agents, the browser or the app", a: "Spread heavy work over time, use lighter models, and start new sessions for long conversations"},
  compactions: {n: "Compactions", d: "How many times a conversation was compacted (summarized to free context), automatically or with /compact, from Claude Code, Codex and the SQLite history of Amazon Q / Kiro CLI, and in how many sessions. For Amazon Q / Kiro CLI only the latest compaction of each conversation is kept", c: "Which conversations grew long enough to need summarizing", x: "Whether anything important was lost in the summary, or whether the conversation should have been split", a: "When one task keeps compacting, finish it at a good stopping point and start the next in a new session"},
  longctx: {n: "Long conversations", d: "Sessions where the input read per response (new input plus cache reads and writes) in the last quarter of the conversation was at least 4 times that of the first quarter, peaking at half the model's context window or more (100K tokens or more when the window is unknown; estimated cost $0.5 or more; only agents that record tokens)", c: "Sessions where each response got heavier as the conversation went on", x: "Whether continuing the conversation was the right call (some work needs the earlier context)", a: "At a good stopping point, write down the key points and continue in a new session"},
  modelfit: {n: "Expensive models for light work", d: "Total for sessions that mainly used Opus-class models, had 3 or fewer prompts, edited no files and cost $0.3 or more", c: "How much went to short tasks on an expensive model", x: "Whether that model was needed (some research or design questions are hard)", a: "Try a lighter model first for research and questions, and switch if it falls short"},
  friction: {n: "Sessions with possible friction", d: "Up to 3 sessions started in the period with at least one correction or interruption, or 15 or more prompts, most first", c: "Sessions where rework piled up", x: "Why it went wrong", a: "Next period, split big requests into one step per prompt"},
  bigPrompts: {n: "Oversized prompts", d: "Number of prompts of 4,000+ characters in the period, and the length of the longest", c: "Whether you hand over large inputs at once, such as pasting whole logs or documents", x: "Whether that length was needed (some long prompts, like design explanations, are fine)", a: "Next period, put long logs or documents in a file and write only its path and the part to look at"},
  repeats: {n: "Repeated prompts", d: "Prompts of 12+ characters in the period, grouped by how similar their text is; up to 3 written in 3 or more sessions, most first", c: "Routine requests you type every time", x: "Whether those requests worked well", a: "Next period, write the most repeated one once as a custom command or in CLAUDE.md"},
  cost: {n: "Estimated cost (API pricing)", d: "Usage priced at public API rates. Uses the cost Claude Code records for itself when available (this also covers price changes, new models and calls not in the history, such as auto-compaction); otherwise, and where it records 0 for a model it did use, tokens in the history times the kiroku price table, plus web searches at $10 per 1,000. Shown as — when none of the models used are in the price table", c: "A rough way to compare how heavy usage was, in money", x: "What you are actually billed (subscriptions differ)", a: "Open the sessions behind the increase, and next period keep that kind of work in shorter conversations"},
  projection: {n: "Month-end cost (estimate)", d: "In a month in progress, the estimated cost from the 1st through today, divided by the days so far and multiplied by the days in the month. Not shown for the first 7 days or on the last day", c: "Roughly where this month's cost is heading at the current pace", x: "Your actual bill, or how you will work from now on (it is off if the pace changes)", a: "If it is too high, look at the heavy sessions and models"},
  projectionCr: {n: "Month-end credits (estimate)", d: "In a month in progress, the Kiro credits from the 1st through today, divided by the days so far and multiplied by the days in the month. Counted separately from the estimated cost, because tokens and credits are separate allowances. Not shown for the first 7 days or on the last day", c: "Roughly how many credits this month is heading for at the current pace", x: "What your account page will show, or how you will work from now on (it is off if the pace changes)", a: "If it is heading past your limit, spread the heavy work out or move some of it to another agent"},
  tokens: {n: "Tokens", d: "Input, output, cache reads and cache writes combined", c: "How much you consumed", x: "Whether more or less is good", a: "Look for skew by project and by day"},
  cache: {n: "Share of input read from cache", d: "The share of input tokens read from cache", c: "Whether the same context was reused", x: "Why it is low (it may just be many short sessions)", a: "If low, next period write long background once in a project file (such as CLAUDE.md) instead of pasting it every time"},
  models: {n: "By model", d: "Estimated cost and tokens per model", c: "Which models your usage leaned toward", x: "Whether that model was needed", a: "Next period, try lighter models for routine work (formatting, renames, adding tests)"},
  subagents: {n: "Subagents", d: "Number of Task / Agent calls, their types and total run time", c: "Whether you delegated research and similar work", x: "How much delegating helped", a: "Check that you use them to save the main conversation's context"},
  credits: {n: "Kiro credits", d: "Total credits recorded in Kiro history", c: "Credits actually consumed", x: "Differences from your account page (period boundaries or use on other machines)", a: "Track your pace against your limit"},
  costPerAsk: {n: "Estimated cost per prompt", d: "Estimated cost ÷ number of prompts in sessions that record tokens (Claude Code, Codex, and Kiro Crew on backends other than kiro-cli); prompts to agents that record only credits are left out", c: "How heavy a typical prompt was", x: "Differences in prompt size", a: "Watch the trend to see how the size of your prompts changes"},
  heavy: {n: "Heaviest sessions", d: "The 3 sessions with the highest estimated cost", c: "Sessions that drove usage up", x: "Whether the result was worth it", a: "Next period, restart long conversations in a new session, and carry work to a commit in small steps"},
  prompts: {n: "Prompts", d: "How many prompts you sent in the period. Slash commands and ! shell commands count as prompts; notifications, system reminders, hook and command output, automatic summaries and instructions from other agents do not", c: "How much you had to put in by hand to get the work done", x: "The size or quality of each prompt, or how much thought went into it", a: "Read it next to active time: many prompts in little time often means steering turn by turn instead of handing over a whole task"},
  outputs: {n: "Cost and outputs", d: "Three parts, in this order (side by side on a wide screen, one under the other on a phone): Cost (your time, prompts and AI usage, from every agent), Left behind (commits, lines, files, pushes and pull requests) and Compared (cost divided by what it left behind). What was left behind is the amount produced, counted from git and from history. Work that leaves no commit, such as research, review or design, does not appear here, and the by-AI share, cost per commit and sessions that reached a commit or PR come from Claude Code only, the one agent whose history records the commits it ran", c: "Whether the cost turned into work that left a trace", x: "Value, quality or productivity. Work that leaves nothing in git", a: "Put the two sides next to each other and look for usage that left nothing"},
  gitCommits: {n: "Git commits", d: "Your own commits (user.email) in the repositories agents worked in, including ones made by hand. Each mark on the right edge of a day in the calendar is one commit (filled = run by AI; touch it to see the short hash); an up arrow is a push from this computer and the other mark a pull request. The share by AI is recognised by matching the times of commits an agent ran with a tool, which only Claude Code records for now", c: "How much of your time with AI became recorded changes", x: "The value of the changes. Work outside the repositories or commits by others", a: "On days with much time or cost but few commits, check where the time went"},
  lines: {n: "Lines changed", d: "Lines added and removed by those commits, as git counts them. The average divides them by every git commit in the period, including the ones you made by hand, while estimated cost per commit divides by the commits AI ran, so the two are not per the same commits. Generated files, lockfiles and moved code count the same as code you wrote", c: "Roughly how much the period produced, without depending on how often you commit", x: "The value or difficulty of the changes. A big number can be one generated file", a: "Read it next to the number of commits to see your usual commit size"},
  files: {n: "Files changed", d: "How many different files those commits touched. kiroku keeps up to 40 files per commit, so a period with larger commits shows a lower bound, marked as at least", c: "How widely the work spread", x: "How much of each file changed, or whether the files are related", a: "If few files took much cost, open those sessions to see what the time went into"},
  pushes: {n: "Pushes", d: "Pushes made from this computer, read from the local git reflog of remote-tracking branches (update by push). Pushes from other computers are not there, and git keeps its reflog for 90 days by default, so older ones are gone", c: "How often the work left your machine", x: "Whether it was reviewed or merged. kiroku never asks GitHub", a: "If commits pile up without pushes, push in smaller steps so the work is shared sooner"},
  prs: {n: "Pull requests", d: "Pull requests an agent created and whose result was recorded (gh pr create, or GitHub tools in Claude Code). Ones you opened in a browser, and ones created by agents that do not record outputs, are not counted", c: "How often work reached a request for review", x: "Whether they were merged, their size or their quality. kiroku never asks GitHub", a: "Compare with commits to see how much work is waiting to be shared"},
  commits: {n: "AI commits", d: "Number of git commits that AI ran successfully. It takes the place of Git commits when kiroku could not read any git repository, and lines, files and pushes, which are counted from git, are left out for the same reason", c: "Roughly how often work reached a checkpoint", x: "The value or size of the changes. Commit size varies by person and task", a: "In periods with few commits for the cost, check where the time went"},
  outSessions: {n: "Sessions that reached a commit or PR", d: "Number and share of sessions that made a commit or created a pull request in the period, counting only Claude Code sessions, the only agent whose outputs are recorded", c: "The share of sessions that left something behind", x: "The value of sessions not meant to commit, such as research or discussion", a: "If low, next period state at the start of each session what done looks like (when to commit)"},
  costPerCommit: {n: "Estimated cost per commit", d: "Estimated cost of Claude Code sessions ÷ number of commits AI ran in them (only Claude Code records outputs, so other agents' cost is left out). Commits you made by hand are not counted", c: "Roughly how heavy it was to reach a checkpoint", x: "Differences in commit size, or work that went into commits you made by hand", a: "Next period, keep each prompt to one change and commit often"},
  native: {n: "Agent-specific metrics", d: "Numbers each agent records in its history", c: "Trends within the same agent", x: "Comparisons between agents (definitions differ)", a: "Only look at changes over time for the same agent"},
};
const H = () => HELP;
// 期間のエージェントのどれも記録しない指標のカード（unrecorded）。数字の欄に「—」を太字で出すと 0 や読み込み中に見えるので、
// 数字の代わりに控えめな文で「記録されていない」と言い、点線の枠で数字のカードと見分けがつくようにする
function hb(id){ return H()[id] ? `<button class="hb" data-help="${id}" aria-label="${`How to read ${H()[id].n}`}" aria-expanded="false" aria-controls="hpop">?</button>` : ""; }
// hintBody は「?」の説明の中身（定義・わかること・わからないこと・次にやること・推移）。押した ? の近くの吹き出し（#hpop）に出す
function hintBody(id){ const h = H()[id]; if (!h) return "";
  const tr = MET[id] ? spark(id, true) : ""; // 推移を出せる指標は、印が付いていなくても説明の中で推移を見られる
  return `<p class="hpt">${esc(h.n)}</p><p>${esc(h.d)}</p><dl><dt>Tells you</dt><dd>${esc(h.c)}</dd><dt>Doesn't tell you</dt><dd>${esc(h.x)}</dd><dt>What to try</dt><dd>${esc(h.a)}</dd></dl>${tr ? `<div class="htrend">${tr}</div>` : ""}`; }

// 「?」の説明は、カードを広げずに、押した ? の近くの吹き出し（#hpop）に出す。開くのは 1 つだけ。
// 同じ ? をもう一度・Esc・吹き出しの外を押すと閉じる（Esc では ? にフォーカスを戻す）。描き直しで ? が消えたら閉じる
let hpopFor = null;
function bindHelp(root){ root.querySelectorAll(".hb").forEach(b => b.onclick = e => { e.stopPropagation(); hpopFor === b ? hideHint() : showHint(b); }); }
function showHint(b){ const P = $("#hpop"), body = hintBody(b.dataset.help); if (!body) return;
  hideHint(); P.innerHTML = body; P.setAttribute("aria-label", b.getAttribute("aria-label")); P.hidden = false; hpopFor = b; b.setAttribute("aria-expanded", "true");
  const r = b.getBoundingClientRect(), w = P.offsetWidth, h = P.offsetHeight, m = 12;
  const left = Math.min(Math.max(m, r.left + r.width/2 - w/2), innerWidth - w - m); // 画面の端からはみ出さない
  const below = r.bottom + 8 + h <= innerHeight - m || r.top - 8 - h < m; // 下に入らなければ上に
  P.style.left = `${left + scrollX}px`; P.style.top = `${(below ? r.bottom + 8 : r.top - 8 - h) + scrollY}px`; }
function hideHint(){ const P = $("#hpop"); if (P.hidden) return; P.hidden = true; P.innerHTML = "";
  if (hpopFor) hpopFor.setAttribute("aria-expanded", "false"); hpopFor = null; }
function closeHint(){ // Esc：開いている説明を閉じて、その ? へフォーカスを戻す。閉じたら true
  if (st.sel || !hpopFor) return false; // 詳細を開いているときは、詳細を閉じるほう
  const b = hpopFor; hideHint(); if (b.isConnected) b.focus(); return true; }
document.addEventListener("click", e => { if (hpopFor && !e.target.closest("#hpop")) hideHint(); });
addEventListener("resize", () => hideHint());
