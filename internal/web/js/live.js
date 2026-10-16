// kiroku serve のときの自動更新と、鍵を付けて data.json を読んでから描く起動（ADR 0031）
/* ── live（kiroku serve） ── 新しい履歴を見つけたら、見ている週・選んだセッションはそのままで取り込む */
function startLive(){
  const badge = $("#live"); badge.hidden = false;
  if (aiRun()){ badge.textContent = t("LIVE · AI on"); badge.title = t("Updates automatically as your history grows. \"Analyze with AI\" is on (turn it off with kiroku serve --no-ai)"); }
  let busy = false;
  async function poll(){
    if (busy || document.hidden) return; busy = true;
    try {
      const r = await kfetch("stamp"); if (!r.ok) throw new Error(r.status);
      if (parseFloat(await r.text()) !== GENERATED){
        const d0 = await kfetch("data.json"); if (!d0.ok) throw new Error(d0.status);
        const j = await d0.json(), before = DATA.length;
        applyData(j);
        render();
        const d = DATA.length - before;
        toast(d === 1 ? t("Added 1 new session") : d > 0 ? t("Added {n} new sessions", {n: d}) : t("Updated to the latest history"));
      }
      badge.classList.remove("off"); badge.title = t("Updates automatically as your history grows (last checked {time})", {time: hm(Date.now()/1000)});
    } catch(e){ badge.classList.add("off"); badge.title = t("Not connected to kiroku serve. If you stopped it, start it again to resume."); }

    busy = false;
  }
  setInterval(poll, 3000); document.addEventListener("visibilitychange", poll);
  startProcs();
}
/* kiroku serve の / は鍵なしでも開けるので、履歴を入れていない骨組み（META.shell）。localStorage の鍵を付けて data.json を読み、
   届いてから最初の描画（期間を選ぶところから）をする。鍵がない・違う（403）ときは、履歴の代わりに kiroku open の案内を出す */
// showLocked は、履歴の代わりに出す案内。key なら kiroku open の案内（lockedPage と同じ文）、そうでなければ、つながらないこと。
// 文字はすべて textContent で入れる
function showLocked(key){
  $("main").hidden = true;
  const box = document.createElement("main"), el = (tag, s) => { const x = document.createElement(tag); x.textContent = s; return x; };
  box.className = "wrap locked"; box.id = "locked";
  // {cmd} のところにコマンドを code で入れる（訳では語順が変わるので、文ごと辞書で引いてから分ける）
  const withCmd = (s, cmd) => { const p = document.createElement("p"), [a, b] = s.split("{cmd}"); p.append(a, el("code", cmd), b || ""); return p; };
  if (key) box.append(el("h2", t("Open kiroku with its key")), el("p", t("kiroku serve shows your history only to a browser that has its key, so that other users of this computer can't read it.")),
    withCmd(t("Run {cmd} in a terminal to open this page with the key. After that, this browser can open it without the key."), "kiroku open"),
    withCmd(t("To open it on another device, run {cmd} and open the address it prints there."), "kiroku open --print"));
  else box.append(el("h2", t("Not connected to kiroku serve")), el("p", t("Not connected to kiroku serve. If you stopped it, start it again to resume.")));
  $("main").after(box);
}
async function openShell(){
  $("main").hidden = true; // 届くまでは、空の期間を描かない
  if (!serveKey()){ showLocked(true); return; }
  let r;
  try { r = await kfetch("data.json"); } catch(e){ showLocked(false); return; }
  if (r.status === 503){ setTimeout(() => location.reload(), 1000); return; } // 読み込み中に戻っていた（/ が読み込み中の画面を返す）
  if (r.status === 403){ showLocked(true); return; }
  let j = null;
  try { if (r.ok) j = await r.json(); } catch(e){}
  if (!j){ showLocked(false); return; }
  applyData(shiftTimes(j));
  $("main").hidden = false;
  boot(); startLive();
}
// 起動。kiroku serve の骨組みは data.json が届いてから、HTML ファイルは埋め込んだデータですぐに描く
if (LIVE && META && META.shell) openShell();
else { boot(); if (LIVE) startLive(); }
