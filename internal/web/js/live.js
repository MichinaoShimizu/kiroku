// kiroku serve のときの自動更新
/* ── live（kiroku serve） ── 新しい履歴を見つけたら、見ている週・選んだセッションはそのままで取り込む */
if (LIVE){
  const badge = $("#live"); badge.hidden = false;
  if (aiRun()){ badge.textContent = t("LIVE · AI on"); badge.title = t("Updates automatically as your history grows. \"Run with AI\" is on (kiroku serve --ai)"); }
  let busy = false;
  async function poll(){
    if (busy || document.hidden) return; busy = true;
    try {
      const r = await fetch("stamp", {cache:"no-store"}); if (!r.ok) throw new Error(r.status);
      if (parseFloat(await r.text()) !== GENERATED){
        const j = await (await fetch("data.json", {cache:"no-store"})).json(), before = DATA.length;
        applyData(j);
        render(); if ($("#yr").open) refreshYear();
        const d = DATA.length - before;
        toast(d === 1 ? t("Added 1 new session") : d > 0 ? t("Added {n} new sessions", {n: d}) : t("Updated to the latest history"));
      }
      badge.classList.remove("off"); badge.title = t("Updates automatically as your history grows (last checked {time})", {time: hm(Date.now()/1000)});
    } catch(e){ badge.classList.add("off"); badge.title = t("Not connected to kiroku serve. If you stopped it, start it again to resume"); }

    busy = false;
  }
  setInterval(poll, 3000); document.addEventListener("visibilitychange", poll);
}
