// Local AI execution: review the exact prompt before consenting; show output in-page.
const AI_SENDS = "The agent you choose may send the prompt and the history it reads to its provider.";
const AI_AGENTS = [["claude", "claude (Claude Code)"], ["codex", "codex (Codex)"], ["kiro", "kiro-cli (Kiro CLI)"]];
// Why "Run with AI" is off here (button tooltips and the dialog)
const AI_OFF_WHY = () => LIVE ? t("This kiroku serve was started without --ai. Start it with kiroku serve --ai to run prompts with an agent") : t("A saved HTML file never runs an AI agent. Run kiroku serve --ai on your computer to run prompts with an agent");
// AI_SEP は、続けて出す 2 つの文の間（日本語では空けない）
const AI_SEP = () => JA ? "" : " ";
async function runAIReview(button, prompt, title) {
  if (document.getElementById("ai-review-dialog")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-review-dialog";
  dialog.style.cssText = "width:min(900px,94vw);max-height:90vh;padding:20px;border:1px solid #888;border-radius:12px;background:var(--bg,#fff);color:inherit";
  const heading = document.createElement("h2"); heading.textContent = title;
  const notice = document.createElement("p");
  notice.textContent = t("Check the exact prompt below. When you press Run, it goes to the agent on this computer.") + AI_SEP() + t(AI_SENDS);
  const agentLabel = document.createElement("label"); agentLabel.textContent = t("Agent") + " ";
  const agent = document.createElement("select");
  for (const [id, name] of AI_AGENTS) { const o = document.createElement("option"); o.value = id; o.textContent = name; agent.append(o); }
  agentLabel.append(agent);
  const preview = document.createElement("textarea");
  preview.readOnly = true; preview.value = prompt;
  preview.setAttribute("aria-label", t("Prompt to send"));
  preview.style.cssText = "display:block;width:100%;height:220px;margin:12px 0;font:12px monospace";
  const status = document.createElement("p"); status.setAttribute("role", "status");
  const output = document.createElement("textarea"); output.readOnly = true;
  output.setAttribute("aria-label", t("AI review result"));
  output.style.cssText = "display:none;width:100%;height:250px;margin:12px 0;font:13px monospace";
  const start = document.createElement("button"); start.type = "button"; start.className = "pill ai-action"; start.textContent = t("Run");
  const cancel = document.createElement("button"); cancel.type = "button"; cancel.className = "pill"; cancel.textContent = t("Close");
  const copyResult = document.createElement("button"); copyResult.type = "button"; copyResult.className = "pill"; copyResult.textContent = t("Copy result"); copyResult.hidden = true;
  const controls = document.createElement("div"); controls.style.cssText = "display:flex;gap:12px;flex-wrap:wrap";
  controls.append(start, cancel, copyResult);
  dialog.append(heading, notice, agentLabel, preview, status, output, controls);
  document.body.append(dialog);
  let controller = null;
  function close() { if (controller) controller.abort(); if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  cancel.onclick = close;
  dialog.addEventListener("cancel", e => { e.preventDefault(); close(); });
  copyResult.onclick = async () => {
    try { await navigator.clipboard.writeText(output.value); status.textContent = t("Result copied"); }
    catch { output.focus(); output.select(); status.textContent = t("Select the result and copy it manually"); }
  };
  start.onclick = async () => {
    controller = new AbortController();
    start.disabled = true; agent.disabled = true; output.style.display = "none"; copyResult.hidden = true;
    cancel.textContent = t("Cancel execution");
    status.textContent = t("Running {agent}…", {agent: agent.value});
    try {
      const response = await fetch("ai/run", {method:"POST", headers:{"Content-Type":"application/json","X-Kiroku":"1"}, body:JSON.stringify({agent:agent.value,prompt:preview.value,consent:true}),signal:controller.signal});
      if (!response.ok) { const why = (await response.text().catch(() => "")).trim().slice(0, 600); throw new Error(why || t("AI execution failed (HTTP {status})", {status: response.status})); }
      const result = await response.json();
      output.value = result.Output || "";
      output.style.display = "block"; copyResult.hidden = false;
      status.textContent = t("Completed with {agent}", {agent: result.Agent || agent.value});
    } catch (error) {
      status.textContent = error.name === "AbortError" ? t("Canceled") : String(error.message || error);
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
  const what = document.createElement("p"); what.textContent = t("With --ai, \"Run with AI\" shows the exact prompt and, after you confirm, runs it with Claude Code, Codex or Kiro CLI on this computer.") + AI_SEP() + t(AI_SENDS) + AI_SEP() + t("Until then, the Copy buttons copy the same prompt for you to paste into an agent yourself.");
  const close = document.createElement("button"); close.type = "button"; close.className = "pill"; close.textContent = t("Close");
  function dismiss() { if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  close.onclick = dismiss;
  dialog.addEventListener("cancel", e => { e.preventDefault(); dismiss(); });
  dialog.append(heading, why, what, close);
  document.body.append(dialog);
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}
