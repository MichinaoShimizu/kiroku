// 詳細パネルのセッション、プロンプトの流れ、詳細の開け閉め（フォーカスの戻し先）
/* ── drawer: session detail ── */
const FIXRE = /違う|ちがう|そうじゃな|やり直|戻して|元に戻|取り消|じゃなくて|\b(?:wrong|incorrect|nope|revert|undo|roll ?back|start over|not what|that's not|try again|(?:doesn't|does not|didn't|did not|still not|isn't|is not) work(?:ing)?|still (?:broken|failing|fails)|you broke)\b/i; // 言い直し・中断らしいプロンプト（プロンプトの流れで点の色を変える）
function detail(s){
  if (!s){ st.sel = null; return summary(); }
  const active = s.segs.reduce((t,[a,b])=>t+(b-a),0)/60, waits = s.waits.map(x=>x[1]).sort((a,b)=>a-b);
  const med = waits.length ? waits[Math.floor(waits.length/2)] : null, maxT = Math.max(1, ...s.tools.map(t=>t[1]));
  const sameDay = new Date(s.start*1000).toDateString() === new Date(s.end*1000).toDateString();
  const allTok = [s.usage, ...s.subagents.map(a=>a.usage)].reduce((t,u)=>t + (u ? u.in+u.out+u.cw+u.cw1h+u.cr : 0), 0);
  // 目安コスト。料金表にないモデルだけなら "—"（$0.00 と出すと、使っていないと読めてしまう）
  const sCost = costOf({cost: s.cost, unpriced: [s.usage, ...s.subagents.map(a=>a.usage)].reduce((t,u)=>t + (u ? u.unpriced||0 : 0), 0)});
  // Kiro のクレジット。Kiro Crew は Kiro 以外のバックエンドのトークンと、kiro-cli のクレジットの両方を持つことがある
  const credH = s.credits ? `<div><div class="k">Kiro credits</div><div class="v">${crN(s.credits)}</div></div><div><div class="k">Per prompt</div><div class="v">${s.nPrompts ? crN(s.credits/s.nPrompts) : "—"}<small> credits</small></div></div>` : "";
  const P = $("#panel");
  P.innerHTML = `<div style="--c:${colorOf(keyOf(s))}">
    ${dHead(agMark(s.source) + esc(s.source), s.title, `${md(s.start)} ${hm(s.start)} – ${sameDay ? "" : md(s.end)+" "}${hm(s.end)}`, [`<span>${esc(s.project)}</span>`, s.branch && `<span>${esc(s.branch)}</span>`])}
    <div class="dcols"><div class="dcol">
    <div class="mini tight">
      <div><div class="k">Active time</div><div class="v">${dur(active,true)}</div></div>
      <div><div class="k">Prompts</div><div class="v">${s.nPrompts}</div></div>
      <div><div class="k">Wait time (median)</div><div class="v">${med==null?"—":secsH(med)}</div></div>
      <div><div class="k">Corrections / interruptions</div><div class="v">${s.corrections + s.interrupts}</div></div>
      ${s.compactions && s.compactions.length ? `<div><div class="k">Compactions</div><div class="v">${s.compactions.length}</div><div class="k">${s.compactions.map(hm).join(", ")}</div></div>` : ""}
      ${s.limits && s.limits.length ? `<div><div class="k">Usage limit hits</div><div class="v wv">${s.limits.length}</div><div class="k">${s.limits.map(hm).join(", ")}</div></div>` : ""}
      ${s.source === "Claude Code" ? `<div><div class="k">Estimated cost${s.costReported ? " (from Claude Code)" : ""}</div><div class="v"${sCost == null ? ` title="${esc(NOPRICE)}"` : ""}>${usdH(sCost)}</div></div>
      <div><div class="k">Tokens</div><div class="v">${tok(allTok)}</div></div>
      ${(() => { const cs = commitsOf(s), ai = cs.filter(c => c.ai).length, o = s.outputs || {}; // 右の「このセッションの間のコミット」と同じ数え方（手でのコミットも入れ、うち AI を添える）
        if (!cs.length) return o.commits ? `<div><div class="k">AI commits</div><div class="v">${o.commits}</div></div>` : ""; // git を読めないときは、AI が実行した回数
        return `<div><div class="k">Git commits</div><div class="v">${cs.length}<small> · ${ai} by AI</small></div></div>${o.prs ? `<div><div class="k">Pull requests created</div><div class="v">${o.prs}</div></div>` : ""}`; })()}
` : allTok ? `<div><div class="k">Estimated cost</div><div class="v"${sCost == null ? ` title="${esc(NOPRICE)}"` : ""}>${usdH(sCost)}</div></div>
      <div><div class="k">Tokens</div><div class="v">${tok(allTok)}</div></div>${credH}
` : credH}
    </div>
    ${s.prompts.length ? promptFlow(s) : `<h3>Prompt flow</h3><p class="none">No prompts recorded.${s.source === "Kiro Crew" ? " Kiro Crew deletes conversation records after a while, so only the usage record remains for this conversation." : ""}</p>`}
    ${s.subagents.length ? `<h3>Subagents · ${s.subagents.length}</h3>${s.subagents.map(a=>{
        const span = Math.max(1, s.end - s.start), l = a.start ? Math.max(0,(a.start - s.start)/span*100) : 0, w = a.start && a.end ? Math.max(.8,(a.end - a.start)/span*100) : .8;
        const t = a.usage, tt = t.in + t.out + t.cw + t.cw1h + t.cr;
        return `<div class="sub"><div class="hd"><span class="ty">${esc(a.type)}</span><span class="ds">${esc(a.desc || "(no description)")}</span>${a.bg?`<span class="bg">Background</span>`:""}</div>
          <div class="lane"><span style="left:${l}%;width:${Math.min(w,100-l)}%"></span></div>
          <div class="ft">${a.start?`<span>${hm(a.start)}${a.end?"–"+hm(a.end):""}</span>`:""}${a.start&&a.end?`<span>${dur((a.end-a.start)/60)}</span>`:""}${tt?`<span>${tok(tt)} tokens</span>`:t.reportedTokens?`<span>${tok(t.reportedTokens)} tokens (reported)</span>`:""}${t.cost?`<span>${usd(t.cost)}</span>`:""}${a.model?`<span>${esc(a.model)}</span>`:""}${a.tools?`<span>${plural(a.tools, "tool call")}</span>`:""}</div></div>`; }).join("")}` : ""}
    </div><div class="dcol">
    ${sessionCommits(s)}
    ${s.prs && s.prs.length ? `<h3>Pull requests created · ${s.prs.length}</h3><ul class="files">${s.prs.map(u => `<li title="${esc(u)}"><span>${ext(u, esc(u.replace(/^https?:\/\//, "")))}</span></li>`).join("")}</ul>` : ""}
    <h3>Files changed${s.files.length || records(s.source, "files") ? ` · ${s.nFiles}` : ""}</h3>
    ${(() => { if (!s.files.length) return `<p class="none">${records(s.source, "files") ? "None" : esc(notRec([s.source]))}</p>`; const us = s.files.map(f => fileLink(s, f)); // 断り書きは、リンクになったファイルがあるときだけ
      return `<ul class="files">${s.files.map((f, i) => `<li title="${esc(f)}"><span>${us[i] ? ext(us[i], esc(f)) : esc(f)}</span></li>`).join("")}</ul>${us.some(Boolean) ? `<p class="note">Links open each file as of the commits made during this session.</p>` : ""}`; })()}
    <details class="moreS dmore" open><summary>${[s.models.length ? "Models" : "", "tools", s.native && s.native.length ? `${esc(s.source)} metrics` : ""].filter(Boolean).join(", ").replace(/, ([^,]+)$/, " and $1").replace(/^./, c => c.toUpperCase())}</summary>
    ${s.models.length ? `<h3>Models used</h3><div class="chips">${s.models.map(([m,n])=>`<span class="mono">${esc((m))}<b>${n}</b></span>`).join("")}</div>` : ""}
    <h3>Tools used</h3>
    ${s.tools.length ? s.tools.map(([k,v],i)=>`<div class="trow"><span class="nm">${esc(k)}</span><span class="track2"><span style="width:${v/maxT*100}%;background:var(--c${i % 8})"></span></span><span class="n">${v}</span></div>`).join("") : `<p class="none">None recorded</p>`}
    ${s.native && s.native.length ? `<h3>${`${esc((s.source))} metrics`}</h3>${nativeRows(s.native)}<p class="note">Numbers this agent records itself. Definitions differ from other agents.</p>` : ""}</details>
    ${s.file ? `<h3>History file</h3><div class="code"><code>${esc(s.file)}</code><a class="copy" href="${esc(LIVE ? "history?id=" + encodeURIComponent(s.id) : fileHref(s.file))}" target="_blank" rel="noopener">Open</a><button class="copy" data-copy="${esc(s.file)}">Copy</button></div>` : ""}
    ${s.resume ? `<h3>Resume</h3><div class="code resume"><code>${esc(s.resume)}</code><button class="copy" data-copy="${esc(s.resume)}">Copy</button></div>` : ""}

  </div></div></div>`;
  bindGitEvents(P);
  const sr = P.querySelector("#sreview"); if (sr) sr.onclick = () => copy(sessionPrompt(s, active, med), "Copied a prompt that asks an AI to review this session. It includes your prompts, so check it before sending", 5000);
  P.querySelectorAll(".pexp").forEach(b => b.onclick = () => { const li = b.closest("li"), open = b.getAttribute("aria-expanded") !== "true";
    li.querySelector(".pshort").hidden = open; li.querySelector(".pfull").hidden = !open; b.setAttribute("aria-expanded", open); b.textContent = open ? "Show less" : pexpLabel(li.dataset.full ? {} : s.prompts[b.dataset.i]); });
  P.querySelectorAll(".pload").forEach(b => b.onclick = async () => { // kiroku serve のときだけ、切ったプロンプトの全文をサーバーから読む
    b.disabled = true;
    try { const r = await fetch(`prompt?id=${encodeURIComponent(s.id)}&i=${b.dataset.i}`, {cache: "no-store"}); if (!r.ok) throw 0;
      const li = b.closest("li"); li.querySelector(".pfull").textContent = await r.text(); li.dataset.full = "1"; li.querySelector(".pexp").focus(); // 「閉じる」は残して、また畳めるように
    } catch { b.disabled = false; b.textContent = "Couldn't load. Try again"; } });
  P.querySelectorAll(".rload").forEach(b => b.onclick = async () => { // kiroku serve のときだけ、切った応答の全文をサーバーから読む
    b.disabled = true;
    try { const r = await fetch(`reply?id=${encodeURIComponent(s.id)}&i=${b.dataset.i}`, {cache: "no-store"}); if (!r.ok) throw 0;
      const li = b.closest("li.rp"); li.querySelector(".reptext").textContent = await r.text(); li.tabIndex = -1; li.focus(); // 読んでいた場所から離れないように
    } catch { b.disabled = false; b.textContent = "Couldn't load. Try again"; } });
  P.querySelectorAll("#flowBy button").forEach(b => b.onclick = () => { st.flowUser = b.dataset.v === "user"; store.set("flowUser", st.flowUser); P.querySelector(".tl").classList.toggle("only-user", st.flowUser); P.querySelector(".tlkey").classList.toggle("only-user", st.flowUser); P.querySelectorAll("#flowBy button").forEach(x => x.setAttribute("aria-pressed", String(x === b))); });
  const pc = P.querySelector("#pcopy"); if (pc) pc.onclick = () => copy(userPrompts(s), `Copied ${plural(s.prompts.length, "user prompt")}`);
  const pall = P.querySelector(".pall"); if (pall) pall.onclick = () => { const shown = [...P.querySelectorAll(".tl li[hidden]")]; shown.forEach(li => li.hidden = false); pall.remove();
    const first = shown.find(li => li.classList.contains("pr")); if (first) first.focus(); }; // 出した最初のプロンプトへ（フォーカスを失わないように）
  bindCopy(P);
}
/* プロンプトの流れ: プロンプトと、その間に起きたこと（コミット・PR・利用上限・中断・サブエージェント）を時刻の順に 1 本に並べる */
const FLOW_SHOW = 30, FLOW_GAP = 1800; // はじめに出すプロンプトの数 / これより長い待たせは「あいた」の区切りにする（待たせの中央値と同じ 30 分）
function span(v){ return v < 3600 ? secs(v) : dur(v / 60); } // 秒数を、1 時間からは時間と分で
function prName(url){ // GitHub の PR は「リポジトリ#番号」と短く（狭い画面でも番号が見えるように）
  const m = /github\.com\/[^/]+\/([^/]+)\/pull\/(\d+)/.exec(url || "");
  return m ? `${m[1]}#${m[2]}` : String(url || "").replace(/^https?:\/\//, "");
}
const NOTE_LABEL = () => ({reminder: "System note", notice: "Notification", hook: "Hook output", output: "Command output",
  compact: "Conversation summary", agent: "From another agent or schedule", meta: "Added by the agent", other: "Added automatically"});
const PKIND = () => ({command: "Command", shell: "Shell"}); // 人が打ったプロンプトのうち、ふつうの文でないもの
function userPrompts(s){ // 人が打ったプロンプトだけを、時刻つきの Markdown の箇条書きにする（書き出し用）
  return s.prompts.map(p => `- ${p.t ? `${md(p.t)} ${hm(p.t)}` : "--:--"}${p.kind ? ` (${mdText(PKIND()[p.kind] || p.kind)})` : ""} ${mdText(p.text)}${p.len > 0 ? ` (first ${PROMPT_RUNES} characters)` : ""}`).join("\n"); }
function flowEvents(s){ // l: 何が起きたか / d: 中身（狭い画面では d だけを省略する）
  const ev = [];
  commitsOf(s).forEach(c => ev.push({t: c.t, k: c.ai ? "commit ai" : "commit", l: c.ai ? "AI committed" : "Committed by hand", d: `<button class="evd" data-git="${esc(c.hash)}"><i class="gtag${c.ai ? " ai" : ""}">${GIT_ICON}${esc(c.hash.slice(0,7))}</i> ${esc(c.subject)}</button>`}));
  (META.push || []).filter(p => p.project === s.project && p.t >= s.start && p.t <= s.end + 600).forEach(p => ev.push({t: p.t, k: "push", l: "Pushed", d: `<button class="evd" data-push="${esc(pushKey(p))}"><span class="mono">${esc(p.ref)}</span>${p.commits ? ` · ${plural(p.commits, "commit")}` : ""}</button>`})); // この PC からの push（git reflog）
  (s.prAt || []).forEach(p => ev.push({t: p.t, k: "pr", l: "Created a pull request", d: `<button class="evd" data-pr="${esc(prKey(s, p))}">${esc(p.url ? prName(p.url) : "Pull request")}</button>`}));
  (s.limits || []).forEach((t, i) => { const r = limitReset(s, i); ev.push({t, k: "warn", l: "Hit a usage limit", d: r ? `<span class="evd">${esc(`resets ${r}`)}</span>` : ""}); }); // 解除の時刻はエラー文のまま（日付や時間帯がないこともある）
  (s.interruptsAt || []).forEach(t => ev.push({t, k: "int", l: "Interrupted"}));
  (s.compactions || []).forEach((t, i) => { const k = compactKind(s, i); ev.push({t, k: "cmp", l: "Conversation compacted", d: k ? `<span class="evd">${esc(k)}</span>` : ""}); });
  (s.notes || []).forEach(x => { if (x.t) ev.push({t: x.t, k: `note ${x.kind}`, l: NOTE_LABEL()[x.kind] || NOTE_LABEL().other, d: `<span class="evd">${esc(x.text)}</span>`}); }); // 人が打っていないもの（通知・要約など）
  s.subagents.forEach(a => { if (a.start) ev.push({t: a.start, k: "agent", l: "Subagent", d: `<span class="evd"><span class="mono">${esc(a.type)}</span>${a.desc ? ` · ${esc(a.desc)}` : ""}</span>`}); });
  return ev.sort((a, b) => a.t - b.t);
}
const EV_ICON = {commit: "commit", push: "push", pr: "pr", warn: "limit", int: "int", cmp: "compact", agent: "agent", note: "note"}; // 流れの出来事の種類 → 印
const pexpLabel = p => p.len > 0 ? `Read more (${p.len.toLocaleString(LOC())} characters)` : "Show all";
/* 依頼に対する応答（エージェントが人に返した最後の文）。依頼の次の発言として、同じ流れの中に出す */
function replyRow(r, i, hidden){
  const cut = r.len > 0;
  const more = cut ? `<span class="pcut"> ${`(first ${REPLY_RUNES} of ${r.len.toLocaleString(LOC())} characters)`}${LIVE ? ` <button class="rload" data-i="${i}">Load the full reply</button>` : ` Open with kiroku serve to read it in full.`}</span>` : "";
  return `<li class="rp"${hidden}><time>${r.t ? hm(r.t) : ""}</time>${ico("reply")}<p><span class="rpw">AI</span><span class="reptext">${esc(r.text)}${cut ? "…" : ""}${more}</span></p></li>`;
}
function promptFlow(s){
  const ev = flowEvents(s), rows = [];
  let e = 0, n = 0, hiddenEv = 0, gap = null; // gap: 前のプロンプトのあと、長くあいたところ {from: AI が最後に動いた時刻, v: 秒}
  const hide = () => n > FLOW_SHOW ? " hidden" : "";
  const flush = until => { for (; e < ev.length && ev[e].t < until; e++){ if (hide()) hiddenEv++;
    rows.push(`<li class="ev ${esc(ev[e].k)}"${hide()}><time>${hm(ev[e].t)}</time>${ico(EV_ICON[ev[e].k.split(" ")[0]])}<p><span class="evl">${ev[e].l}</span>${ev[e].d || ""}</p></li>`); } };
  s.prompts.forEach((p, i) => {
    if (p.t){ flush(p.t); // あいた間に起きたこと（手でのコミットなど）は、区切りの上に出す
      if (gap){ rows.push(`<li class="gap"${hide()}><p>${`${hm(gap.from)}–${hm(p.t)}: ${span(gap.v)} gap`}</p></li>`); gap = null; } }
    n++;
    const t = String(p.text || ""), long = t.length > 220, fix = !p.kind && FIXRE.test(t), cut = p.len > 0;
    const more = cut ? `<span class="pcut"> ${`(first ${PROMPT_RUNES} of ${p.len.toLocaleString(LOC())} characters)`}${LIVE ? ` <button class="pload" data-i="${i}">Load the full prompt</button>` : ` Open with kiroku serve to read it in full.`}</span>` : "";
    const meta = [p.work ? `AI worked ${span(p.work)}` : "", p.wait && p.wait <= FLOW_GAP ? `wait ${secs(p.wait)}` : ""].filter(Boolean).join(" · ");
    rows.push(`<li class="pr${fix ? " fix" : ""}${p.kind ? " " + esc(p.kind) : ""}" tabindex="-1"${hide()}><time>${p.t ? hm(p.t) : ""}</time><p>${fix ? `<span class="sr">Looks like a correction: </span>` : ""}${p.kind ? `<span class="pkind">${ico(p.kind)}${PKIND()[p.kind] || esc(p.kind)}</span>` : ""}${plen(p) >= BIG_PROMPT ? `<span class="pkind big">${`Long · ${plen(p).toLocaleString(LOC())} chars`}</span>` : ""}<span class="ptext">${long ? `<span class="pshort">${esc(t.slice(0,220))}…</span><span class="pfull" hidden>${esc(t)}${more}</span> <button class="pexp" aria-expanded="false" data-i="${i}">${pexpLabel(p)}</button>` : esc(t)}</span>${meta ? `<span class="pmeta">${meta}</span>` : ""}</p></li>`);
    if (p.reply){ // 依頼と応答の間に起きたこと（コミットなど）は、応答より上に出す
      if (p.reply.t) flush(p.reply.t);
      rows.push(replyRow(p.reply, i, hide()));
    }
    if (p.t && p.wait > FLOW_GAP) gap = {from: p.t + p.work, v: p.wait};
  });
  flush(Infinity);
  const fixes = s.prompts.some(p => !p.kind && FIXRE.test(String(p.text || ""))), kinds = new Set(ev.map(x => x.k.split(" ")[0])), cmds = s.prompts.some(p => p.kind);
  const key = [`<span><i class="kp"></i>User prompt</span>`,
    cmds ? `<span><i class="kp cmd"></i>Commands the user typed (/ or !)</span>` : "",
    kinds.has("note") ? `<span class="kev note">${ico("note")}Added automatically (notifications, summaries, hooks; not counted as prompts)</span>` : "",
    fixes ? `<span><i class="kp fix"></i>Looks like a correction (guessed from the wording)</span>` : "",
    s.prompts.some(p => p.reply) ? `<span class="kev rep">${ico("reply")}${"What the AI wrote back"}</span>` : "",
    ...[["commit", "Commit"], ["push", "Push"], ["pr", "Pull request"], ["agent", "Subagent"], ["int", "Interruption"], ["cmp", "Compaction"], ["warn", "Usage limit"]].filter(([k]) => kinds.has(k)).map(([k, l]) => `<span class="kev ${k}">${ico(EV_ICON[k])}${l}</span>`)].filter(Boolean).join("");
  const rest = s.prompts.length - FLOW_SHOW, also = hiddenEv ? ` (and ${plural(hiddenEv, "other event")})` : "";
  // 見出しと出し方の切り替えは 1 行に（最初の画面に入るプロンプトを 1 つでも多くする）
  const bar = `<div class="flowhead"><h3>Prompt flow</h3><div class="flowbar"><div class="segc" role="group" aria-label="Show" id="flowBy"><button data-v="all" aria-pressed="${!st.flowUser}">Everything</button><button data-v="user" aria-pressed="${!!st.flowUser}">Only user prompts</button></div><button class="pill" id="pcopy">Copy prompts</button><button class="pill" id="sreview" title="A prompt that asks an AI how you could have prompted and split the work better">Copy review prompt</button></div></div>`;
  return `${bar}<div class="tlkey${st.flowUser ? " only-user" : ""}">${key}</div><ol class="tl${st.flowUser ? " only-user" : ""}">${rows.join("")}</ol>${rest > 0 ? `<button class="more pall">${`Show ${plural(rest, "more prompt")}${also}`}</button>` : ""}${s.prompts.some(p => p.work) ? `<p class="note">${"\"AI worked\" is the time from a prompt to the AI's last activity; \"wait\" is the time from there to your next prompt. Both are estimates from the history's timestamps."}</p>` : ""}${s.prompts.some(p => p.reply) ? `<p class="note">${`The reply after a prompt is the last thing the AI wrote to you in that turn, in its own words — not its thinking, its tool calls or their output. A turn where it only ran tools, or whose words the agent does not record, has no reply. A long one shows its first ${REPLY_RUNES} characters.`}</p>` : ""}`;
}
/* 1 つのセッションを AI と振り返るためのプロンプト（kiroku 自身は AI を呼ばない） */
function sessionPrompt(s, active, med){
  const o = s.outputs || {}, L = [
    "You are an advisor on using AI agents effectively. Below is the record of one session I had with an AI agent (aggregated with kiroku).",
    "", "# What I'd like from you",
    "1. What went well in how this session was run",
    "2. Information that would have reduced rework if it had been in the first prompt (background, constraints, done criteria, etc.)",
    "3. Better ways to split and order the prompts, and an example first prompt for similar work next time",
    "", "# Assumptions",
    "- Figures are rough estimates from history. Clearly mark anything the data can't support as a guess",
    "- Estimated cost is priced at public API rates, not what I am actually billed",
    AI_DATA_NOTE], D = ["# Session",
    `- Agent: ${(s.source)}`, `- Project: ${s.project}${s.branch ? ` (branch ${s.branch})` : ""}`,
    `- Time: ${md(s.start)} ${hm(s.start)}–${hm(s.end)}, active time ${dur(active)}`,
    `- Prompts: ${s.nPrompts}, corrections: ${s.corrections}, interruptions: ${s.interrupts}${med == null ? "" : `, median wait time ${secs(med)}`}`];
  if (s.compactions && s.compactions.length) D.push(`- Compactions (the conversation was summarized to free context): ${s.compactions.length} (${s.compactions.map((t, i) => `${hm(t)}${compactKind(s, i) ? ` ${compactKind(s, i)}` : ""}`).join(", ")})`);
  if (s.limits && s.limits.length) D.push(`- Usage limit hits: ${s.limits.length} (${s.limits.map(hm).join(", ")})`);
  if (s.cost) D.push(`- Estimated cost: ${usd(s.cost)}`);
  if (s.credits) D.push(`- Kiro credits: ${crN(s.credits)}`);
  D.push(o.commits ? `- Commits: ${o.commits}${o.prs ? `, pull requests: ${o.prs}` : ""}` : "- No commits recorded");
  if (s.tools.length) D.push(`- Most used tools: ${s.tools.slice(0,6).map(([k,v]) => `${k} ${v}`).join(", ")}`);
  if (s.subagents.length) D.push(`- Subagents: ${s.subagents.length}`);
  D.push("", "# Prompt flow (time and prompt; long ones are truncated)");
  if (s.prompts.length) s.prompts.slice(0, 40).forEach(p => { const t = String(p.text || "").replace(/\s+/g, " ").trim(); D.push(`- ${p.t ? hm(p.t) : "--:--"} ${t.length > 300 ? t.slice(0, 300) + "…" : t}`); });
  else D.push("- No prompts recorded");
  if (s.prompts.length > 40) D.push(`- ${s.nPrompts - 40} more`);
  L.push("", "# History data", mdFence(D.join("\n")));
  return L.join("\n");
}
const focusDrawer = {was: false, sel: null, first: null, from: null, next: null};
const OPENER_ATTRS = ["data-sid", "data-id", "data-s", "data-c", "data-git", "data-push", "data-pr"];
let lastClick = null; addEventListener("click", e => { lastClick = e.target; }, true); // Safari はボタンを押してもフォーカスが移らないので、押した要素も覚える
function openerOf(el){ // 詳細を開いた要素を、描き直したあとも同じものを探せる形で覚える（どの枠の、どの属性の、何番目か）
  el = el && el.closest && el.closest(OPENER_ATTRS.map(a => `[${a}]`).join(","));
  const root = el && el.closest("#tl, #review"), a = root && OPENER_ATTRS.find(a => el.hasAttribute(a));
  if (!a) return null;
  const q = `[${a}="${CSS.escape(el.getAttribute(a))}"]`;
  return {root: root.id, q, n: [...root.querySelectorAll(q)].indexOf(el), y: scrollY}; }
function focusOpener(sel, from){ // 閉じたら、詳細を開いた要素（帯・バッジ・カード・検索結果）へフォーカスと読んでいた位置を戻す
  if (from){ const r = document.getElementById(from.root), el = r && r.querySelectorAll(from.q)[from.n];
    if (el){ scrollTo(0, from.y); el.focus({preventScroll: true}); return; } }
  if (!sel) return; const v = CSS.escape(sel.replace(/^(git|push|pr):/, ""));
  const el = sel.startsWith("git:") ? document.querySelector(`.gc[data-c="${v}"],[data-git="${v}"],[data-c="${v}"]`)
    : sel.startsWith("push:") ? document.querySelector(`[data-push="${v}"]`) : sel.startsWith("pr:") ? document.querySelector(`[data-pr="${v}"]`)
    : document.querySelector(`.run[data-sid="${v}"],[data-id="${v}"],[data-s="${v}"]`);
  if (el) el.focus({preventScroll: false}); }
function select(id){ // 詳細の中で別の詳細へ移ったときは、戻れるように前のものを積む
  if (id && !st.sel) focusDrawer.next = openerOf(lastClick) || openerOf(document.activeElement);
  if (id && st.sel && id !== st.sel){ st.back.push(st.sel); (st.backTop ||= []).push($("#panel").scrollTop); } else if (!id){ st.back = []; st.backTop = []; } // 戻ったとき、読んでいた位置に戻す
  st.sel = id; tipOff(); render(); if (id) $("#panel").scrollTop = 0; dtopSync(); }
// 詳細の題名が上へ流れて見えなくなったら、上の帯（#dtop）に小さく出す（どの詳細を読んでいるかわからなくならないように）
function dtopSync(){ const P = $("#panel"), h = P.querySelector("h2"), D = $("#drawer");
  $("#dtop").textContent = h ? h.textContent : "";
  D.classList.toggle("scrolled", !!h && h.getBoundingClientRect().bottom < P.getBoundingClientRect().top + 4); }
$("#panel").addEventListener("scroll", dtopSync, {passive: true});

