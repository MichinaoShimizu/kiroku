// Local AI execution: review the exact prompt before consenting; show output in-page.
const AI_AGENTS = [["claude", "claude (Claude Code)"], ["codex", "codex (Codex)"], ["kiro", "kiro-cli (Kiro CLI)"]];
// Why "Run with AI" is off here (button tooltips and the dialog)
const AI_OFF_WHY = () => LIVE ? t("This kiroku serve was started without --ai. Start it with kiroku serve --ai to run prompts with an agent.") : t("A saved HTML file never runs an AI agent. Run kiroku serve --ai on your computer to run prompts with an agent.");
// AI_SEP は、続けて出す 2 つの文の間（日本語では空けない）
const AI_SEP = () => JA ? "" : " ";
async function runAIReview(button, prompt, title) {
  if (document.getElementById("ai-review-dialog")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-review-dialog";
  dialog.style.cssText = "width:min(900px,94vw);max-height:90vh;padding:20px;border:1px solid #888;border-radius:12px;background:var(--bg,#fff);color:inherit";
  const heading = document.createElement("h2"); heading.textContent = title;
  const notice = document.createElement("p");
  notice.textContent = t("Check the exact prompt below. When you press Run, it goes to the agent on this computer.");
  const agentLabel = document.createElement("label"); agentLabel.textContent = t("Agent") + " ";
  const agent = document.createElement("select");
  for (const [id, name] of AI_AGENTS) { const o = document.createElement("option"); o.value = id; o.textContent = name; agent.append(o); }
  agentLabel.append(agent);
  const preview = document.createElement("textarea");
  preview.readOnly = true; preview.value = prompt;
  preview.setAttribute("aria-label", t("Prompt to send"));
  preview.style.cssText = "display:block;width:100%;height:220px;margin:12px 0;font:12px monospace";
  const status = document.createElement("p"); status.setAttribute("role", "status");
  // 経過（実行したコマンド・エージェントの動き・エラー出力）と、返事。返事は整えた表示と Markdown のまま見る表示を切り替えられる
  const activity = document.createElement("pre"); activity.className = "ai-log"; activity.hidden = true;
  activity.setAttribute("aria-label", t("Progress"));
  const resultHead = document.createElement("div"); resultHead.className = "ai-rhead"; resultHead.hidden = true;
  const resultLabel = document.createElement("span"); resultLabel.textContent = t("Result");
  const views = document.createElement("div"); views.className = "segc"; views.setAttribute("role", "group"); views.setAttribute("aria-label", t("Show the result as"));
  const viewBtns = [["formatted", t("Formatted")], ["markdown", "Markdown"]].map(([v, l]) => { const b = document.createElement("button"); b.type = "button"; b.dataset.v = v; b.textContent = l; views.append(b); return b; });
  resultHead.append(resultLabel, views);
  const output = document.createElement("div"); output.className = "ai-out"; output.hidden = true; output.tabIndex = 0;
  output.setAttribute("aria-label", t("Result"));
  let view = store.get("aiView", "formatted"), text = "", drawn = -1;
  const draw = () => { // 返事を、選んだ表示で描き直す（届くたびに呼ぶので、1 フレームに 1 回まで）
    if (drawn === text.length + (view === "markdown" ? 0.5 : 0)) return;
    drawn = text.length + (view === "markdown" ? 0.5 : 0);
    const stick = output.scrollHeight - output.scrollTop - output.clientHeight < 24;
    output.replaceChildren(); output.classList.toggle("raw", view === "markdown");
    if (view === "markdown") output.textContent = text; else output.append(mdView(text));
    if (stick) output.scrollTop = output.scrollHeight;
  };
  let pending = 0;
  const redraw = () => { if (!pending) pending = requestAnimationFrame(() => { pending = 0; draw(); }); };
  const setView = v => { view = v === "markdown" ? "markdown" : "formatted"; store.set("aiView", view); viewBtns.forEach(b => b.setAttribute("aria-pressed", String(b.dataset.v === view))); draw(); };
  viewBtns.forEach(b => b.onclick = () => setView(b.dataset.v));
  setView(view);
  const log = (s, cls) => { // 経過に 1 行足す（文字だけ。下に着いていれば追いかける）
    const stick = activity.scrollHeight - activity.scrollTop - activity.clientHeight < 24;
    const span = document.createElement("span"); if (cls) span.className = cls; span.textContent = s; activity.append(span);
    if (stick) activity.scrollTop = activity.scrollHeight;
  };
  const start = document.createElement("button"); start.type = "button"; start.className = "pill ai-action"; start.textContent = t("Run");
  const cancel = document.createElement("button"); cancel.type = "button"; cancel.className = "pill"; cancel.textContent = t("Close");
  const copyResult = document.createElement("button"); copyResult.type = "button"; copyResult.className = "pill"; copyResult.textContent = t("Copy result"); copyResult.hidden = true;
  const controls = document.createElement("div"); controls.style.cssText = "display:flex;gap:12px;flex-wrap:wrap";
  controls.append(start, cancel, copyResult);
  dialog.append(heading, notice, agentLabel, preview, status, activity, resultHead, output, controls);
  document.body.append(dialog);
  let controller = null;
  function close() { if (controller) controller.abort(); if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  cancel.onclick = close;
  dialog.addEventListener("cancel", e => { e.preventDefault(); close(); });
  copyResult.onclick = async () => {
    try { await navigator.clipboard.writeText(text); status.textContent = t("Result copied"); }
    catch { setView("markdown"); const r = document.createRange(); r.selectNodeContents(output); const sel = getSelection(); sel.removeAllRanges(); sel.addRange(r); status.textContent = t("Select the result and copy it manually"); }
  };
  start.onclick = async () => {
    controller = new AbortController();
    start.disabled = true; agent.disabled = true; copyResult.hidden = true;
    text = ""; drawn = -1; activity.replaceChildren(); activity.hidden = false; resultHead.hidden = false; output.hidden = false; draw();
    preview.style.height = "72px"; // 送ったプロンプトは小さくして、経過と結果を見えるところに
    cancel.textContent = t("Cancel");
    status.textContent = t("Running {agent}…", {agent: agent.value});
    let done = null;
    try {
      const response = await fetch("ai/run", {method:"POST", headers:{"Content-Type":"application/json","X-Kiroku":"1"}, body:JSON.stringify({agent:agent.value,prompt:preview.value,consent:true}),signal:controller.signal});
      if (!response.ok) { const why = (await response.text().catch(() => "")).trim().slice(0, 600); throw new Error(why || t("AI execution failed (HTTP {status})", {status: response.status})); }
      // 1 行に 1 つの JSON（コマンド・返事の文字・ツール・エラー出力・最後に done）が、起きた順に届く
      const reader = response.body.getReader(), dec = new TextDecoder();
      let buf = "";
      const take = line => {
        if (!line.trim()) return;
        let e; try { e = JSON.parse(line); } catch { return; }
        const s = typeof e.s === "string" ? e.s : "";
        if (e.t === "cmd" && Array.isArray(e.argv)) log("$ " + e.argv.map(a => /^[\w.\/:=+-]+$/.test(String(a)) ? String(a) : JSON.stringify(String(a))).join(" ") + "\n", "cmd");
        else if (e.t === "tool") log("› " + s + "\n", "tool");
        else if (e.t === "log") log(s, "err");
        else if (e.t === "text") { text += s; redraw(); }
        else if (e.t === "done") done = e;
      };
      for (;;) {
        const {value, done: end} = await reader.read();
        if (value) { buf += dec.decode(value, {stream: true}); let k; while ((k = buf.indexOf("\n")) >= 0) { take(buf.slice(0, k)); buf = buf.slice(k + 1); } }
        if (end) { take(buf + dec.decode()); break; }
      }
      if (!done) throw new Error(t("The run ended without a result"));
      if (!done.ok) throw new Error(String(done.error || t("AI execution failed")).slice(0, 600));
      text = String(done.output ?? text); drawn = -1; draw();
      copyResult.hidden = !text;
      status.textContent = t("Completed with {agent}", {agent: done.agent || agent.value});
    } catch (error) {
      status.textContent = error.name === "AbortError" ? t("Canceled") : String(error.message || error);
      draw();
    } finally { controller = null; cancel.textContent = t("Close"); start.disabled = false; agent.disabled = false; start.textContent = t("Run again"); }
  };
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}

// Without kiroku serve --ai (a saved file, or serve started without it), explain how to turn it on.
function showLocalAIInfo() {
  if (document.getElementById("ai-local-info")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-local-info";
  dialog.style.cssText = "width:min(480px,90vw);max-height:80vh;padding:24px;border:1px solid #888;border-radius:12px;background:var(--surface,#fff);color:var(--ink,#111)";
  const heading = document.createElement("h2"); heading.textContent = t("Run with AI is off");
  const why = document.createElement("p"); why.textContent = LIVE
    ? t("This kiroku serve was started without --ai. To turn it on, stop it (Ctrl+C in its terminal) and start it again with kiroku serve --ai. kiroku autostart can't pass --ai: if you use it, run kiroku autostart off first.")
    : t("This is a saved HTML file, which never runs an AI agent. To run prompts with an agent, start kiroku serve --ai on your computer.");
  const what = document.createElement("p"); what.textContent = t("With --ai, \"Run with AI\" shows the exact prompt and, after you confirm, runs it with Claude Code, Codex or Kiro CLI on this computer.") + AI_SEP() + t("Until then, the Copy buttons copy the same prompt for you to paste into an agent yourself.");
  const close = document.createElement("button"); close.type = "button"; close.className = "pill"; close.textContent = t("Close");
  function dismiss() { if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  close.onclick = dismiss;
  dialog.addEventListener("cancel", e => { e.preventDefault(); dismiss(); });
  dialog.append(heading, why, what, close);
  document.body.append(dialog);
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}
