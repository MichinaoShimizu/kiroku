// Local AI execution: review the exact prompt before consenting; show output in-page.
// 表示名（コマンド名）。選択肢と、失敗したときの文に使う
const AI_AGENTS = [["claude", "Claude Code (claude)"], ["codex", "Codex (codex)"], ["kiro", "Kiro CLI (kiro-cli)"]];
const aiName = id => ((AI_AGENTS.find(a => a[0] === id) || [])[1] || id).replace(/ \(.*\)$/, "");
// Why "Analyze with AI" is off here (button tooltips and the dialog)
const AI_OFF_WHY = () => !LIVE ? t("A saved HTML file never runs an AI agent. Run kiroku serve --ai on your computer to run prompts with an agent.")
  : META && META.aiRun ? t("No AI agent command (claude, codex or kiro-cli) was found on this computer. Install one, then restart kiroku serve --ai.")
  : t("This kiroku serve was started without --ai. Start it with kiroku serve --ai to run prompts with an agent.");
// AI_SEP は、続けて出す 2 つの文の間（日本語では空けない）
const AI_SEP = () => JA ? "" : " ";
// 閉じたあとに同じプロンプトで開き直したら、前の結果を見せる（このページを開いているあいだだけ。保存はしない）
const AI_LAST = new Map();
async function runAIReview(button, prompt, title) {
  if (document.getElementById("ai-review-dialog")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-review-dialog"; dialog.className = "ai-dialog";
  dialog.setAttribute("aria-labelledby", "ai-review-title");
  const heading = document.createElement("h2"); heading.id = "ai-review-title"; heading.textContent = title;
  const notice = document.createElement("p");
  notice.textContent = t("Check the exact prompt below. When you press Run, it goes to the agent on this computer.");
  const agentLabel = document.createElement("label"); agentLabel.className = "ai-agent"; agentLabel.append(document.createTextNode(t("Agent")));
  const agent = document.createElement("select");
  for (const [id, name] of AI_AGENTS.filter(([id]) => aiAgents().includes(id))) { const o = document.createElement("option"); o.value = id; o.textContent = name; agent.append(o); }
  agentLabel.append(agent);
  const preview = document.createElement("textarea");
  preview.readOnly = true; preview.value = prompt; preview.className = "ai-prompt";
  preview.setAttribute("aria-label", t("Prompt to send"));
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
  const showResult = on => { resultHead.hidden = output.hidden = !on; };
  const draw = () => { // 返事を、選んだ表示で描き直す（届くたびに呼ぶので、1 フレームに 1 回まで）
    if (drawn === text.length + (view === "markdown" ? 0.5 : 0)) return;
    drawn = text.length + (view === "markdown" ? 0.5 : 0);
    const stick = output.scrollHeight - output.scrollTop - output.clientHeight < 24;
    output.replaceChildren(); output.classList.toggle("raw", view === "markdown");
    if (view === "markdown") output.textContent = text; else output.append(mdView(text));
    if (stick) output.scrollTop = output.scrollHeight;
  };
  let pending = 0;
  // 返事が大きくなったら、描き直す間をあける（整えるのに時間がかかり、届くたびに描くと画面が重くなる）
  const redraw = () => { if (!pending) pending = text.length > 100000 ? setTimeout(() => { pending = 0; draw(); }, 500) : requestAnimationFrame(() => { pending = 0; draw(); }); };
  const setView = v => { view = v === "markdown" ? "markdown" : "formatted"; store.set("aiView", view); viewBtns.forEach(b => b.setAttribute("aria-pressed", String(b.dataset.v === view))); drawn = -1; draw(); };
  viewBtns.forEach(b => b.onclick = () => setView(b.dataset.v));
  setView(view);
  let errLines = 0;
  const log = (s, cls) => { // 経過に 1 行足す（文字だけ。下に着いていれば追いかける）
    const stick = activity.scrollHeight - activity.scrollTop - activity.clientHeight < 24;
    const span = document.createElement("span"); if (cls) span.className = cls; span.textContent = s; activity.append(span);
    if (cls === "err") errLines++;
    if (stick) activity.scrollTop = activity.scrollHeight;
  };
  const start = document.createElement("button"); start.type = "button"; start.className = "pill ai-action"; start.textContent = t("Run");
  const cancel = document.createElement("button"); cancel.type = "button"; cancel.className = "pill"; cancel.textContent = t("Close");
  const copyResult = document.createElement("button"); copyResult.type = "button"; copyResult.className = "pill"; copyResult.textContent = t("Copy result"); copyResult.hidden = true;
  // ボタンの並びは、ダイアログの下に貼りつけておく（出力が長くても、キャンセルをすぐ押せるように）
  const controls = document.createElement("div"); controls.className = "ai-controls";
  controls.append(start, cancel, copyResult);
  // 結果が出たら、入力（説明・エージェント・プロンプト）と経過はゆっくりたたむ。見出しの下のボタンで開き直せる
  const inputs = document.createElement("div"); inputs.className = "ai-inputs"; inputs.id = "ai-review-inputs";
  const inputsIn = document.createElement("div"); inputsIn.className = "ai-inputs-in";
  inputsIn.append(notice, agentLabel, preview, activity); inputs.append(inputsIn);
  const inToggle = document.createElement("button"); inToggle.type = "button"; inToggle.className = "ai-intoggle"; inToggle.hidden = true;
  inToggle.setAttribute("aria-controls", inputs.id);
  const setInputs = open => {
    inputs.classList.toggle("closed", !open); inputs.inert = !open; dialog.classList.toggle("ai-focus", !open);
    inToggle.setAttribute("aria-expanded", String(open));
    inToggle.textContent = open ? t("Hide prompt and progress") : t("Show prompt and progress");
  };
  inToggle.onclick = () => setInputs(inputs.classList.contains("closed"));
  setInputs(true);
  dialog.append(heading, inToggle, inputs, status, resultHead, output, controls);
  document.body.append(dialog);
  const key = title + "\n" + prompt;
  let controller = null;
  function close() { if (controller) controller.abort(); if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  // 実行中のキャンセルと Esc は、実行を止めるだけ（ダイアログは閉じず、届いた分は残す）。止まっていれば閉じる
  const stopOrClose = () => { if (controller) controller.abort(); else close(); };
  cancel.onclick = stopOrClose;
  dialog.addEventListener("cancel", e => { e.preventDefault(); stopOrClose(); });
  copyResult.onclick = async () => {
    try { await navigator.clipboard.writeText(text); copyResult.textContent = t("Copied"); setTimeout(() => { copyResult.textContent = t("Copy result"); }, 2000); }
    catch { setView("markdown"); const r = document.createRange(); r.selectNodeContents(output); const sel = getSelection(); sel.removeAllRanges(); sel.addRange(r); status.textContent = t("Select the result and copy it manually"); }
  };
  const last = AI_LAST.get(key); // 前に同じプロンプトで得た結果
  if (last){ text = last.text; showResult(true); drawn = -1; draw(); copyResult.hidden = false; start.textContent = t("Run again"); status.textContent = t("Last result with {agent}", {agent: aiName(last.agent)}); inToggle.hidden = false; setInputs(false); }
  start.onclick = async () => {
    controller = new AbortController();
    inToggle.hidden = true; setInputs(true);
    start.disabled = true; agent.disabled = true; copyResult.hidden = true; errLines = 0;
    text = ""; drawn = -1; activity.replaceChildren(); activity.hidden = false; showResult(true); draw();
    preview.classList.add("small"); // 送ったプロンプトは小さくして、経過と結果を見えるところに
    cancel.textContent = t("Cancel"); cancel.focus(); // 押したボタンが使えなくなるので、フォーカスはキャンセルへ
    const name = aiName(agent.value);
    status.textContent = t("Running {agent}…", {agent: name});
    let done = null, ok = false;
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
      if (!done.ok) // エージェントのエラー出力は経過に出ているので、ここでは繰り返さない
        throw new Error(errLines ? t("{agent} stopped with an error. Its output is shown above.", {agent: name}) : String(done.error || t("AI execution failed")).slice(0, 600));
      text = String(done.output ?? text); drawn = -1; draw(); output.scrollTop = 0; // 読み始めは先頭から
      ok = !!text;
      status.textContent = t("Completed with {agent}", {agent: name});
      if (ok){ AI_LAST.set(key, {text, agent: agent.value}); inToggle.hidden = false; setInputs(false); } // 結果だけを見せる（失敗・中止のときは、原因が見えるよう開いたまま）
    } catch (error) {
      status.textContent = error.name === "AbortError" ? t("Canceled") : String(error.message || error);
      draw();
    } finally {
      controller = null; cancel.textContent = t("Close"); start.disabled = false; agent.disabled = false; start.textContent = t("Run again");
      copyResult.hidden = !text; showResult(!!text);
      if (dialog.open) (ok ? copyResult : start).focus();
    }
  };
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}

// Without kiroku serve --ai (a saved file, or serve started without it), explain how to turn it on.
function showLocalAIInfo() {
  if (document.getElementById("ai-local-info")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-local-info";
  dialog.style.cssText = "width:min(480px,90vw);max-height:80vh;padding:24px;border:1px solid #888;border-radius:12px;background:var(--surface,#fff);color:var(--ink,#111)";
  dialog.setAttribute("aria-labelledby", "ai-local-title");
  const heading = document.createElement("h2"); heading.id = "ai-local-title"; heading.textContent = t("Analyze with AI is off");
  const why = document.createElement("p"); why.textContent = LIVE && META && META.aiRun ? AI_OFF_WHY() : LIVE
    ? t("This kiroku serve was started without --ai. To turn it on, stop it (Ctrl+C in its terminal) and start it again with kiroku serve --ai. kiroku autostart can't pass --ai: if you use it, run kiroku autostart off first.")
    : t("This is a saved HTML file, which never runs an AI agent. To run prompts with an agent, start kiroku serve --ai on your computer.");
  const noAgent = LIVE && META && META.aiRun; // --ai は付いているが、エージェントのコマンドがない
  const what = document.createElement("p"); what.textContent = (noAgent ? "" : t("With --ai, \"Analyze with AI\" shows the exact prompt and, after you confirm, runs it with Claude Code, Codex or Kiro CLI on this computer.") + AI_SEP()) + t("Until then, the Copy buttons copy the same prompt for you to paste into an agent yourself.");
  const close = document.createElement("button"); close.type = "button"; close.className = "pill"; close.textContent = t("Close");
  function dismiss() { if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  close.onclick = dismiss;
  dialog.addEventListener("cancel", e => { e.preventDefault(); dismiss(); });
  dialog.append(heading, why, what, close);
  document.body.append(dialog);
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}
