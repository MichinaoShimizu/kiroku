/* 指標ガイド：指標どうしのつながりを 1 つのダイアログで見せる。
   かけたもの（Cost）→ AI との働き方（4 つのレバー）→ 残ったもの（Left behind）、それを割って比べる（Compared）、
   Worth a look から次の期間へ。名前と「わかること」は HELP から取るので、指標の説明を変えればここも変わる。
   HELP のどの指標もここのどこかに入れる（TestGuideCoversHelp） */
const GUIDE = {
  cost: ["active", "prompts", "tokens", "cost", "credits", "projection", "projectionCr"],
  levers: [
    {n: "Hand over more", up: true, m: "When AI keeps working while you wait, more gets done per hour of your time. The more runs in parallel, the further AI run time pulls ahead of active time", ids: ["ai", "parallel", "subagents"]},
    {n: "Keep your attention unbroken", m: "Waiting for a reply, missing that it arrived and getting back up to speed after a project switch all take your time without moving the work", ids: ["focus", "switches", "wait"]},
    {n: "Get it across the first time", m: "When a prompt misses, corrections and redone work pile up. Typing the same instructions every time belongs here too", ids: ["fix", "friction", "bigPrompts", "repeats"]},
    {n: "Fit the context and the model", m: "A conversation that keeps growing rereads more input on every response, an expensive model on light work pays more for the same result, and a usage limit stops the work", ids: ["cache", "longctx", "compactions", "modelfit", "models", "costPerAsk", "limits"]},
  ],
  out: ["gitCommits", "commits", "lines", "files", "pushes", "prs"],
  cmp: ["costPerCommit", "outSessions"], // 画面の「Cost and outputs」の Compared と同じ
  dig: ["outputs", "projects", "heavy", "native", "findings"],
};
const GNUM = ["①", "②", "③", "④"];
function openGuide(){
  hideHint(); // 開いた「?」の説明は、ダイアログの後ろに残さない
  const D = $("#mg"), h = id => H()[id];
  const names = ids => ids.filter(h).map(id => `<li>${esc(h(id).n)}</li>`).join("");
  const rows = ids => ids.filter(h).map(id => `<div><dt>${esc(h(id).n)}</dt><dd>${esc(h(id).c)}</dd></div>`).join("");
  const tag = l => l.up ? `<span class="mgtag up">${esc(t("Left behind ↑"))}</span>` : `<span class="mgtag down">${esc(t("Cost ↓"))}</span>`;
  D.innerHTML = `<form method="dialog" class="dclose"><button class="iconbtn" aria-label="${esc(t("Close"))}"><svg class="i" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18"/></svg></button></form>
    <div class="eyebrow">${esc(t("Metrics guide"))}</div><h2 id="mgh">${esc(t("How the metrics fit together"))}</h2>
    <p class="glead" id="mgd">${esc(t("kiroku doesn't measure productivity. It shows what you put in, what was left behind, and how you worked with AI in between. Each metric about how you worked acts through one of four levers: one adds to what is left behind, three take away from the cost. In the terms of the SPACE framework, this covers Activity and Efficiency and flow."))}</p>
    <div class="gflow" role="group" aria-label="${esc(t("Cost passes through four levers and becomes what is left behind"))}">
      <section class="gbox gin"><h3>${esc(t("Cost"))}</h3><p>${esc(t("What you put in"))}</p><ul>${names(GUIDE.cost)}</ul></section>
      <span class="garr" aria-hidden="true"></span>
      <section class="gbox glev"><h3>${esc(t("How you work with AI"))}</h3><p>${esc(t("Four levers"))}</p><ol>${GUIDE.levers.map((l, i) => `<li><span class="mgnum" aria-hidden="true">${GNUM[i]}</span>${esc(t(l.n))}${tag(l)}</li>`).join("")}</ol></section>
      <span class="garr" aria-hidden="true"></span>
      <section class="gbox gout"><h3>${esc(t("Left behind"))}</h3><p>${esc(t("What remains in git and on GitHub"))}</p><ul>${names(GUIDE.out)}</ul></section>
    </div>
    <div class="gbox gcmp"><h3>${esc(t("Compared"))}</h3><p>${esc(t("What was left behind for what you put in, to compare your own periods (not people or teams)"))}</p><ul>${names(GUIDE.cmp)}</ul></div>
    <h3 class="gh">${esc(t("The four levers"))}</h3>
    ${GUIDE.levers.map((l, i) => `<section class="glever"><h4><span class="mgnum" aria-hidden="true">${GNUM[i]}</span>${esc(t(l.n))}${tag(l)}</h4><p>${esc(t(l.m))}</p><dl class="grows">${rows(l.ids)}</dl></section>`).join("")}
    <h3 class="gh">${esc(t("Making a change"))}</h3>
    <ol class="gsteps"><li><b>${esc(t("Open Worth a look"))}</b>${esc(t("Metrics that crossed a threshold, in priority order"))}</li><li><b>${esc(t("Open the sessions"))}</b>${esc(t("See where the time and usage went"))}</li><li><b>${esc(t("Try one change"))}</b>${esc(t("Follow \"What to try\" in the metric's \"?\""))}</li><li><b>${esc(t("Check the same metric"))}</b>${esc(t("Next week or month, in its 8-week or 8-month trend"))}</li></ol>
    <p class="gnote">${esc(t("To dig in: {list}.", {list: GUIDE.dig.filter(h).map(id => h(id).n).join(JA ? "、" : ", ")}))}</p>
    <h3 class="gh">${esc(t("What this doesn't tell you"))}</h3>
    <dl class="grows gnot">
      <div><dt>${esc(t("Value and quality"))}</dt><dd>${esc(t("Commits and lines are amounts, not the value or difficulty of a change. One generated file can make lines changed large"))}</dd></div>
      <div><dt>${esc(t("Work that leaves nothing in git"))}</dt><dd>${esc(t("Research, review, design and discussion don't show up in Left behind"))}</dd></div>
      <div><dt>${esc(t("Time saved"))}</dt><dd>${esc(t("kiroku can't know how long the work would have taken without AI. What you see is the change between your own periods"))}</dd></div>
      <div><dt>${esc(t("Quality, delivery and satisfaction"))}</dt><dd>${esc(t("Of the five dimensions in {space}, kiroku covers Activity (Left behind) and Efficiency and flow (the levers). Quality and delivery, as in {dora}, and Satisfaction, Performance and Collaboration need data kiroku doesn't read: deployments, incidents, reviews, and how you and your team feel about the work"))
        .replace("{space}", () => ext("https://queue.acm.org/detail.cfm?id=3454124", esc(t("the SPACE framework ↗")))).replace("{dora}", () => ext("https://dora.dev/", esc(t("the DORA metrics ↗"))))}</dd></div>
      <div><dt>${esc(t("Other agents' outputs"))}</dt><dd>${esc(t("The share of commits by AI, cost per commit and sessions that reached a commit come from Claude Code only, the one agent whose history records its outputs"))}</dd></div>
    </dl>`;
  D.showModal(); D.scrollTop = 0;
  const hd = $("#mgh"); hd.tabIndex = -1; hd.focus();
}
