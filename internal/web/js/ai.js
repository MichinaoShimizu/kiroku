// Local AI execution: review the exact prompt before consenting; show output in-page.
async function runAIReview(button, prompt, title) {
  if (document.getElementById("ai-review-dialog")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-review-dialog";
  dialog.style.cssText = "width:min(900px,94vw);max-height:90vh;padding:20px;border:1px solid #888;border-radius:12px;background:var(--bg,#fff);color:inherit";
  const heading = document.createElement("h2"); heading.textContent = title;
  const notice = document.createElement("p");
  notice.textContent = "Review the exact prompt below. Running an agent may transmit its contents and referenced history to an external AI provider.";
  const agentLabel = document.createElement("label"); agentLabel.textContent = "Agent ";
  const agent = document.createElement("select");
  for (const id of ["claude", "codex", "kiro"]) { const o = document.createElement("option"); o.value = id; o.textContent = id; agent.append(o); }
  agentLabel.append(agent);
  const preview = document.createElement("textarea");
  preview.readOnly = true; preview.value = prompt;
  preview.setAttribute("aria-label", "Prompt to send");
  preview.style.cssText = "display:block;width:100%;height:220px;margin:12px 0;font:12px monospace";
  const status = document.createElement("p"); status.setAttribute("role", "status");
  const output = document.createElement("textarea"); output.readOnly = true;
  output.setAttribute("aria-label", "AI review result");
  output.style.cssText = "display:none;width:100%;height:250px;margin:12px 0;font:13px monospace";
  const start = document.createElement("button"); start.textContent = "Consent and run";
  const cancel = document.createElement("button"); cancel.textContent = "Close";
  const copyResult = document.createElement("button"); copyResult.textContent = "Copy result"; copyResult.hidden = true;
  const controls = document.createElement("div"); controls.style.cssText = "display:flex;gap:12px;flex-wrap:wrap";
  controls.append(start, cancel, copyResult);
  dialog.append(heading, notice, agentLabel, preview, status, output, controls);
  document.body.append(dialog);
  let controller = null;
  function close() { if (controller) controller.abort(); if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  cancel.onclick = close;
  dialog.addEventListener("cancel", e => { e.preventDefault(); close(); });
  copyResult.onclick = async () => {
    try { await navigator.clipboard.writeText(output.value); status.textContent = "Result copied"; }
    catch { output.focus(); output.select(); status.textContent = "Select the result and copy it manually"; }
  };
  start.onclick = async () => {
    controller = new AbortController();
    start.disabled = true; agent.disabled = true; output.style.display = "none"; copyResult.hidden = true;
    cancel.textContent = "Cancel execution";
    status.textContent = "Running " + agent.value + "…";
    try {
      const response = await fetch("ai/run", {method:"POST", headers:{"Content-Type":"application/json","X-Kiroku":"1"}, body:JSON.stringify({agent:agent.value,prompt:preview.value,consent:true}),signal:controller.signal});
      if (!response.ok) throw new Error("AI execution failed (HTTP " + response.status + ")");
      const result = await response.json();
      output.value = result.Output || "";
      output.style.display = "block"; copyResult.hidden = false;
      status.textContent = "Completed with " + (result.Agent || agent.value);
    } catch (error) {
      status.textContent = error.name === "AbortError" ? "Canceled" : String(error.message || error);
    } finally { controller = null; cancel.textContent = "Close"; start.disabled = false; agent.disabled = false; start.textContent = "Run again"; }
  };
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}

// Static demos explain the feature without attempting local execution.
function showLocalAIInfo() {
  if (document.getElementById("ai-local-info")) return;
  const dialog = document.createElement("dialog");
  dialog.id = "ai-local-info";
  dialog.style.cssText = "width:min(440px,90vw);max-height:80vh;padding:24px;border:1px solid #888;border-radius:12px;background:var(--surface,#fff);color:var(--ink,#111)";
  const heading = document.createElement("h2"); heading.textContent = "AI review · Local only";
  const message = document.createElement("p"); message.textContent = "AI review runs only when you start kiroku on your computer with \"kiroku serve --ai\". It then runs Claude Code, Codex or Kiro CLI on this computer after you confirm each run, and that agent may send the prompt and the history it reads to its provider. Without --ai, and in a saved HTML file, kiroku never runs an AI agent; use the Copy buttons to paste a prompt into an agent yourself.";
  const close = document.createElement("button"); close.type = "button"; close.textContent = "Close";
  function dismiss() { if (dialog.open && typeof dialog.close === "function") dialog.close(); dialog.remove(); }
  close.onclick = dismiss;
  dialog.addEventListener("cancel", e => { e.preventDefault(); dismiss(); });
  dialog.append(heading, message, close);
  document.body.append(dialog);
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}
