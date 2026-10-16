// 動いているエージェントと、エージェントが終わったあとも残っているプロセス（kiroku serve のときだけ。ADR 0035）
/* /procs が返すのは、プロセスの短い名前・PID・開始時刻・作業フォルダ・メモリ・CPU 時間・結びついたセッションだけ（コマンドラインや環境変数は来ない）。
   名前とフォルダはこの PC のプロセスから来るので、ほかの履歴と同じく esc() してから入れる */
let PROCS = [];
const procsOn = () => LIVE && !!(META && META.procs);
const procsLeft = () => PROCS.filter(p => p.kind === "left" || (p.orphan && !p.tty));
const procsFor = id => PROCS.filter(p => p.session === id);
async function loadProcs(){
  if (!procsOn() || document.hidden) return;
  try { const r = await kfetch("procs"); if (!r.ok) return; const j = await r.json(); PROCS = Array.isArray(j.procs) ? j.procs : []; }
  catch(e){ return; }
  drawProcsBadge(); fillSessionProcs();
  const D = $("#pd"); if (D && D.open && !D.querySelector(".pxask")) drawProcsDialog(); // 確かめている途中は描き直さない
}
function startProcs(){
  if (!procsOn()) return;
  loadProcs(); setInterval(loadProcs, 15000); document.addEventListener("visibilitychange", loadProcs);
  $("#procs").onclick = () => openProcs($("#procs"));
}
// 上の LIVE の横のボタン。残っているものがあれば注意の色で数を出し、エージェントが動いているだけなら数だけ、何もなければ隠す
function drawProcsBadge(){
  const b = $("#procs"), left = procsLeft().length, agents = PROCS.filter(p => p.kind === "agent").length;
  b.hidden = !PROCS.length;
  b.classList.toggle("warn", left > 0);
  b.textContent = left ? t("{n} left running", {n: left}) : agents === 1 ? t("1 agent running") : t("{n} agents running", {n: agents});
  b.title = left ? t("Processes that AI agents started are still running after the agent ended. Click to see them and stop them") : t("AI agents running on this computer. Click to see them");
}
// procStatus は、どこで動いているか・なぜ出しているか
function procStatus(p){
  if (p.kind === "left") return t("Left running after {agent} ended", {agent: p.agent});
  if (p.orphan && !p.tty) return t("Its terminal or program is gone");
  if (p.tty) return t("Open in a terminal");
  return p.parent ? t("Run by {name}", {name: p.parent}) : t("Running");
}
const procPlace = p => { const s = DATA.find(x => x.id === p.session); return s ? s.source : p.agent; };
function procRow(p, inSession){
  const s = !inSession && p.session ? DATA.find(x => x.id === p.session) : null, warn = p.kind === "left" || (p.orphan && !p.tty);
  const meta = [t("running for {d}", {d: dur(Math.max(0, nowMs()/1000 - p.start)/60)}), t("{v} memory", {v: bytes(p.rss || 0)}), t("{v} CPU", {v: dur((p.cpu || 0)/60)}),
    p.n > 1 ? t("{n} processes", {n: p.n}) : ""].filter(Boolean).join(" · ");
  return `<li class="pxrow${warn ? " warn" : ""}" data-pid="${p.pid}" data-start="${p.start}">
    <div class="pxh">${agMark(procPlace(p))}<b translate="no">${esc(p.name)}</b><span class="pxpid">PID ${p.pid}</span><span class="pxst">${esc(procStatus(p))}</span></div>
    <div class="pxm">${esc(meta)}</div>${p.cwd ? `<div class="pxm pxcwd" translate="no" title="${esc(p.cwd)}">${esc(p.cwd)}</div>` : ""}
    ${s ? sesCardH(s, `${md(s.start)} ${hm(s.start)}–${hm(s.end)} · ${esc(s.project)}`) : ""}
    <div class="pxact">${p.self ? `<span class="note">${esc(t("kiroku runs inside it. Stop it from its terminal."))}</span>` : `<button class="pill pxstop" type="button">${esc(t("Stop…"))}</button>`}</div></li>`;
}
function procList(ps, inSession){ return `<ul class="pxlist">${ps.map(p => procRow(p, inSession)).join("")}</ul>`; }
// 一覧のダイアログ。残っているものを先に（Go がその順で返す）
function drawProcsDialog(){
  const D = $("#pd"), left = procsLeft(), agents = PROCS.filter(p => !left.includes(p)); // ターミナルのなくなったエージェントも「残っているもの」に（上のボタンの数とそろえる）
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">${esc(t("This computer, now"))}</div><h2 id="pdh">${esc(t("Running AI agents"))}</h2>
    <p class="mdef">${esc(t("AI agents running on this computer, and processes they started (such as a dev server) that are still running after the agent ended, with the session each belongs to."))}</p>
    ${left.length ? `${secH(esc(t("Left running")) + ` · ${left.length}`)}${procList(left)}` : ""}
    ${agents.length ? `${secH(esc(t("Agents")) + ` · ${agents.length}`)}${procList(agents)}` : ""}
    ${PROCS.length ? "" : noneH(esc(t("No AI agent is running, and nothing started by one is left running.")))}
    <p class="note">${esc(t("Stop sends the process and everything under it a request to end (SIGTERM), like kill. Anything unsaved in it is lost. kiroku reads only your own processes, and never their command lines or environment."))}</p>`;
  bindProcRows(D);
  D.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
}
function openProcs(from){
  const D = $("#pd"); drawProcsDialog();
  D.onclose = () => { if (from && from.isConnected && !st.sel) from.focus(); };
  D.showModal(); D.scrollTop = 0;
  const h = $("#pdh"); h.tabIndex = -1; h.focus();
  loadProcs();
}
// 詳細パネルの「まだ動いているもの」。PROCS が変わるたびに入れ直す
function sessionProcsH(s){ return procsOn() ? `<div id="dprocs" data-sid="${esc(s.id)}">${sessionProcsInner(s.id)}</div>` : ""; }
function sessionProcsInner(id){ const ps = procsFor(id); return ps.length ? `${secH(esc(t("Still running")) + ` · ${ps.length}`)}${procList(ps, true)}` : ""; }
function fillSessionProcs(){
  const box = $("#dprocs"); if (!box || box.querySelector(".pxask")) return;
  box.innerHTML = sessionProcsInner(box.dataset.sid); bindProcRows(box);
}
// 止める：押すとその場で確かめ、もう一度押したら送る。終わらなければ強制終了（SIGKILL）を出す
function bindProcRows(root){
  root.querySelectorAll(".pxrow .pxstop").forEach(b => b.onclick = () => {
    const li = b.closest(".pxrow"), p = PROCS.find(x => String(x.pid) === li.dataset.pid && String(x.start) === li.dataset.start); if (!p) return;
    askStop(li.querySelector(".pxact"), p, false);
  });
}
// redrawProcs は、確かめる文を消してから、一覧と詳細を今の PROCS で描き直す
function redrawProcs(box){ box.innerHTML = ""; if ($("#pd").open) drawProcsDialog(); fillSessionProcs(); }
function askStop(box, p, force){
  const q = force ? t("{name} (PID {pid}) is still running. Force it to stop (SIGKILL)? It gets no chance to clean up.", {name: p.name, pid: p.pid})
    : p.n > 1 ? t("Stop {name} (PID {pid}) and the {n} processes under it? Anything unsaved in them is lost.", {name: p.name, pid: p.pid, n: p.n - 1})
    : t("Stop {name} (PID {pid})? Anything unsaved in it is lost.", {name: p.name, pid: p.pid});
  box.innerHTML = `<div class="pxask" role="group"><span>${esc(q)}</span><button class="pill warn pxgo" type="button">${esc(force ? t("Force stop") : t("Stop"))}</button><button class="pill pxno" type="button">${esc(t("Cancel"))}</button></div>`;
  const go = box.querySelector(".pxgo"), no = box.querySelector(".pxno");
  no.onclick = () => redrawProcs(box);
  go.onclick = async () => {
    go.disabled = no.disabled = true; go.textContent = t("Stopping…");
    let res = null, err = "";
    try {
      const r = await kfetch("procs/stop", {method: "POST", headers: {"Content-Type": "application/json", "X-Kiroku": "1"}, body: JSON.stringify({pid: p.pid, start: p.start, force})});
      if (r.ok) res = await r.json(); else err = r.status === 404 ? t("It has already ended.") : t("Couldn't stop it. See the terminal where kiroku serve runs.");
    } catch(e){ err = t("The connection to kiroku serve was lost."); }
    if (res && res.running > 0 && !force) return askStop(box, p, true);
    toast(res ? (res.running > 0 ? t("{name} is still running. Stop it from its terminal or with kill -9 {pid}.", {name: p.name, pid: p.pid}) : t("Stopped {name}.", {name: p.name})) : err, 5000);
    box.innerHTML = ""; await loadProcs(); redrawProcs(box);
  };
  go.focus();
}
