// 動いているエージェントと、エージェントが終わったあとも残っているプロセス（kiroku serve のときだけ。ADR 0035）
/* /procs が返すのは、プロセスの短い名前・PID・開始時刻・作業フォルダ・メモリ・CPU 時間・結びついたセッションだけ（コマンドラインや環境変数は来ない）。
   名前とフォルダはこの PC のプロセスから来るので、ほかの履歴と同じく esc() してから入れる */
let PROCS = [], PX_SIG = "", PX_MSG = "";
const procsOn = () => LIVE && !!(META && META.procs);
const procsLeft = () => PROCS.filter(p => p.kind === "left" || (p.orphan && !p.tty));
const procsFor = id => PROCS.filter(p => p.session === id);
const pxKey = p => `${p.pid}:${p.start}`;
async function loadProcs(){
  if (!procsOn() || document.hidden) return;
  try { const r = await kfetch("procs"); if (!r.ok) return; const j = await r.json(); PROCS = Array.isArray(j.procs) ? j.procs : []; }
  catch(e){ return; }
  const sig = JSON.stringify(PROCS.map(p => [pxKey(p), p.n, p.session, p.rss >> 20, Math.floor(p.cpu / 60)])); // 変わっていなければ描き直さない（フォーカスと読んでいる位置を保つ）
  drawProcsBadge();
  if (sig === PX_SIG) return;
  PX_SIG = sig; redrawProcs();
}
function startProcs(){
  if (!procsOn()) return;
  loadProcs(); setInterval(loadProcs, 15000); document.addEventListener("visibilitychange", loadProcs);
  $("#procs").onclick = () => openProcs($("#procs"));
  // 確かめている途中の Esc は、ダイアログを閉じずに、確かめるのだけをやめる
  $("#pd").addEventListener("cancel", e => { const a = $("#pd .pxask"); if (a){ e.preventDefault(); const no = a.querySelector(".pxno"); if (no) no.click(); } });
}
// 上の LIVE の横のボタン。残っているものがあれば注意の色で数を出し、エージェントが動いているだけなら数だけ、何もなければ隠す。
// 狭い画面では数だけにする（ヘッダーからはみ出さないように。読み上げは aria-label の文）
function drawProcsBadge(){
  const b = $("#procs"), left = procsLeft().filter(p => !p.self).length, agents = PROCS.filter(p => p.kind === "agent").length;
  const full = left ? (left === 1 ? t("1 process left running") : t("{n} processes left running", {n: left})) : agents === 1 ? t("1 agent running") : t("{n} agents running", {n: agents});
  b.hidden = !PROCS.length;
  b.classList.toggle("warn", left > 0);
  const l = document.createElement("span"), s = document.createElement("span");
  l.className = "pxbl"; l.textContent = full; s.className = "pxbs"; s.textContent = String(left || agents); s.setAttribute("aria-hidden", "true");
  b.replaceChildren(l, s);
  b.setAttribute("aria-label", full);
  b.title = left ? t("Processes that AI agents started are still running after the agent ended. Click to see them and stop them") : t("AI agents running on this computer. Click to see them");
}
// procStatus は、どこで動いているか・なぜ出しているか（わかっていることだけを書く）
function procStatus(p){
  if (p.kind === "left") return t("Started by {agent}, now running on its own", {agent: p.agent});
  if (p.orphan && !p.tty) return t("Its terminal or program is gone");
  if (p.tty) return t("Open in a terminal");
  return p.parent ? t("Started from {name}", {name: p.parent}) : t("Running");
}
const procPlace = p => { const s = DATA.find(x => x.id === p.session); return s ? s.source : p.agent; };
const pxDur = sec => sec < 60 ? t("under a minute") : dur(sec / 60);
const pxPath = f => { const parts = String(f).split(/[\\/]/).filter(Boolean); return f.length > 40 && parts.length > 2 ? "…/" + parts.slice(-2).join("/") : f; }; // 長いフォルダは末尾の 2 つだけ（全体は title に）
function procRow(p, inSession){
  const s = !inSession && p.session ? DATA.find(x => x.id === p.session) : null;
  const meta = [t("running for {d}", {d: pxDur(Math.max(0, nowMs()/1000 - p.start))}), t("{v} memory", {v: bytes(p.rss || 0)}), t("CPU time {v}", {v: pxDur(p.cpu || 0)}),
    p.n > 1 ? t("{n} processes", {n: p.n}) : ""].filter(Boolean).join(" · ");
  return `<li class="pxrow" data-pid="${p.pid}" data-start="${p.start}">
    <div class="pxh">${agMark(procPlace(p))}<b translate="no">${esc(p.name)}</b><span class="pxpid">PID ${p.pid}</span><span class="pxst">${esc(procStatus(p))}</span></div>
    <div class="pxm">${esc(meta)}</div>${p.cwd ? `<div class="pxm pxcwd" translate="no" title="${esc(p.cwd)}">${esc(pxPath(p.cwd))}</div>` : ""}
    ${s ? sesCardH(s, `${md(s.start)} ${hm(s.start)}–${hm(s.end)} · ${esc(s.project)}`) : ""}
    <div class="pxact">${p.self ? `<span class="note">${esc(t("kiroku runs inside it, so it can't be stopped from here."))}</span>` : `<button class="pill pxstop" type="button">${esc(t("Stop…"))}</button>`}</div></li>`;
}
function procList(ps, inSession){ return `<ul class="pxlist">${ps.map(p => procRow(p, inSession)).join("")}</ul>`; }
// 一覧のダイアログ。残っているものを先に、それぞれ古いものから（新しいものは下に足されるので、押そうとしたボタンがずれない）
function drawProcsDialog(){
  const D = $("#pd"), left = procsLeft(), agents = PROCS.filter(p => !left.includes(p)); // ターミナルのなくなったエージェントも「残ったプロセス」に（上のボタンの数とそろえる）
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">${esc(t("On this computer, now"))}</div><h2 id="pdh" tabindex="-1">${esc(t("AI agents and what they left running"))}</h2>
    <p class="mdef">${esc(t("AI agents running on this computer, and processes they started (such as a dev server) that are still running after the agent ended, with the session each belongs to."))}</p>
    <p class="pxmsg" role="status">${esc(PX_MSG)}</p>
    ${left.length ? `${secH(esc(t("Left running")) + ` · ${left.length}`)}${procList(left)}` : ""}
    ${agents.length ? `${secH(esc(t("Running agents")) + ` · ${agents.length}`)}${procList(agents)}` : ""}
    ${PROCS.length ? "" : noneH(esc(t("No AI agent is running, and nothing started by one is left running.")))}
    <p class="note">${esc(t("Stop asks the process and everything under it to end (SIGTERM), like kill. Anything unsaved in it is lost. kiroku reads only your own processes. It never reads their command lines, and reads their environment only to look for the agents' markers, keeping nothing of it."))}</p>`;
  bindProcRows(D);
  D.querySelectorAll("[data-id]").forEach(b => b.onclick = () => { D.close(); select(b.dataset.id); });
}
function openProcs(from){
  const D = $("#pd"); PX_MSG = ""; drawProcsDialog();
  D.onclose = () => { if (from && from.isConnected && !st.sel) from.focus(); };
  D.showModal(); D.scrollTop = 0;
  $("#pdh").focus();
  loadProcs(); // 描き直すのは中身が変わったときだけ
}
// 詳細パネルの「まだ動いているもの」。PROCS が変わるたびに入れ直す
function sessionProcsH(s){ return procsOn() ? `<div id="dprocs" data-sid="${esc(s.id)}">${sessionProcsInner(s.id)}</div>` : ""; }
function sessionProcsInner(id){ const ps = procsFor(id); return ps.length ? `${secH(esc(t("Still running")) + ` · ${ps.length}`)}${procList(ps, true)}` : ""; }
function fillSessionProcs(){
  const box = $("#dprocs"); if (!box || box.querySelector(".pxask")) return;
  box.innerHTML = sessionProcsInner(box.dataset.sid); bindProcRows(box);
}
// redrawProcs は、一覧と詳細を今の PROCS で描き直す。確かめている途中の一覧は残す。フォーカスは同じプロセスの同じボタンへ戻す
function redrawProcs(){
  const D = $("#pd"), a = document.activeElement, row = a && a.closest && a.closest(".pxrow");
  const cls = a && a.classList ? [...a.classList].find(c => c.startsWith("px")) : null;
  const root = D.open && D.contains(a) ? "#pd" : a && a.closest && a.closest("#dprocs") ? "#dprocs" : null;
  if (D.open && !D.querySelector(".pxask")){ const y = D.scrollTop; drawProcsDialog(); D.scrollTop = y; }
  fillSessionProcs();
  if (root) focusProc(root, row ? `${row.dataset.pid}:${row.dataset.start}` : null, null, cls);
}
// focusProc は、root の中の key のプロセスの cls のボタンへ、なければ at 番目の「止める…」へ、それもなければ見出しへフォーカスを移す
function focusProc(root, key, at, cls = "pxstop"){
  const R = $(root); if (!R) return;
  const rows = [...R.querySelectorAll(".pxrow")], row = key && rows.find(r => `${r.dataset.pid}:${r.dataset.start}` === key);
  const el = (row && cls && row.querySelector("." + cls)) || (at != null && R.querySelectorAll(".pxstop")[Math.min(at, R.querySelectorAll(".pxstop").length - 1)]) || (root === "#pd" ? $("#pdh") : null);
  if (el && document.activeElement !== el) el.focus();
}
// 止める：押すとその場で確かめ（既定のフォーカスは「キャンセル」）、もう一度押したら送る。終わらなければ強制終了（SIGKILL）を出す
function bindProcRows(root){
  root.querySelectorAll(".pxrow .pxstop").forEach(b => b.onclick = () => {
    const li = b.closest(".pxrow"), p = PROCS.find(x => String(x.pid) === li.dataset.pid && String(x.start) === li.dataset.start); if (!p) return;
    askStop(li.querySelector(".pxact"), p, false);
  });
}
function askStop(box, p, force){
  const v = {name: p.name, pid: p.pid, n: p.n - 1, agent: p.agent}, agent = p.kind === "agent";
  const q = force ? t("{name} (PID {pid}) is still running. Force it to stop (SIGKILL)? It gets no chance to clean up.", v)
    : agent ? (p.n > 1 ? t("End {agent} (PID {pid}) and the {n} processes under it? The session it is running stops, and anything unsaved is lost.", v) : t("End {agent} (PID {pid})? The session it is running stops, and anything unsaved is lost.", v))
    : p.n > 1 ? t("Stop {name} (PID {pid}) and the {n} processes under it? Anything unsaved in them is lost.", v)
    : t("Stop {name} (PID {pid})? Anything unsaved in it is lost.", v);
  box.innerHTML = `<div class="pxask" role="group"><span>${esc(q)}</span><button class="pill warn pxgo" type="button">${esc(force ? t("Force stop") : t("Stop"))}</button><button class="pill pxno" type="button">${esc(t("Cancel"))}</button></div>`;
  const go = box.querySelector(".pxgo"), no = box.querySelector(".pxno"), li = box.closest(".pxrow"), key = pxKey(p);
  const root = li.closest("#pd") ? "#pd" : "#dprocs", idx = () => [...$(root).querySelectorAll(".pxrow")].indexOf(li);
  no.onclick = () => { box.innerHTML = ""; redrawProcs(); focusProc(root, key, null); };
  go.onclick = async () => {
    const at = idx(), inDlg = root === "#pd";
    // 送ったあとは取り消せないので、キャンセルを残さず、何をしているかだけを出す
    const busy = document.createElement("div"), line = document.createElement("span");
    busy.className = "pxask pxbusy"; busy.setAttribute("role", "status");
    line.textContent = force ? t("Forcing {name} to stop…", v) : t("Asked {name} to stop; checking…", v);
    busy.append(line); box.replaceChildren(busy);
    let res = null, err = "";
    try {
      const r = await kfetch("procs/stop", {method: "POST", headers: {"Content-Type": "application/json", "X-Kiroku": "1"}, body: JSON.stringify({pid: p.pid, start: p.start, force})});
      if (r.ok) res = await r.json(); else err = r.status === 404 ? t("{name} has already ended.", v) : t("Couldn't stop {name}. See the terminal where kiroku serve runs.", v);
    } catch(e){ err = t("The connection to kiroku serve was lost."); }
    if (res && res.running > 0 && !force) return askStop(box, p, true);
    const msg = res ? (res.running > 0 ? t("{name} is still running. Stop it from its terminal or with kill -9 {pid}.", v) : t("Stopped {name}.", v)) : err;
    if (inDlg) PX_MSG = msg; else toast(msg, 5000); // ダイアログの中ではトーストが見えないので、ダイアログの中に出す
    box.innerHTML = ""; PX_SIG = "";
    await loadProcs(); redrawProcs(); focusProc(root, key, at);
  };
  no.focus();
}
