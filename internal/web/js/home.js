// ホームのページ（# なし・#/）：ページごとのカードに、そのページを開く前に知りたいことを少しだけ。
// セッション履歴は今週（なければ記録のあるいちばん新しい週）の数・日ごとの稼働時間・最近のセッション、
// PRトリアージは PR の数（PRトリアージで読んだ答えがあるときだけ）。ホームを開いても GitHub には問い合わせない（kiroku serve --prs の約束どおり、
// 問い合わせるのは PRトリアージを開いたときと「更新」を押したときだけ）。履歴の文字は esc を通して出す
// homeWeek は、ホームに出す週 {w, from, label}。今週に記録がなければ、記録のあるいちばん新しい週（なければ null）
function homeWeek(){
  if (META && (META.demo || META.scope)){ const k = key(st.week); // デモと期間だけのファイルは、カレンダーが開く週（boot が決めた st.week）と同じ週
    return WEEKS[k] && WEEKS[k].sessions ? {w: WEEKS[k], from: st.week, label: t("Week of {date}", {date: dMD(st.week)})} : null; }
  const now = mondayOf(today0()), k = key(now);
  if (WEEKS[k] && WEEKS[k].sessions) return {w: WEEKS[k], from: now, label: t("This week")};
  const last = Object.keys(WEEKS).filter(x => WEEKS[x] && WEEKS[x].sessions).sort().pop(); if (!last) return null;
  const [y, m, d] = last.split("-").map(Number), from = new Date(y, m-1, d);
  return {w: WEEKS[last], from, label: t("Week of {date}", {date: dMD(from)})};
}
// homeDays は、週の日ごとの稼働時間の小さな棒（今日の棒は色を濃く。まだ来ていない日は棒なし）。読み上げは日ごとの時間をつないだ文
function homeDays(w, from){
  const days = w.days || [], max = Math.max(1, ...days.map(d => d.active || 0)), t0 = key(today0());
  const lab = days.map((d, i) => `${md(addDays(from, i).getTime()/1000)} ${dur(d.active || 0)}`).join(TSEP());
  return `<div class="hmdays" role="img" aria-label="${esc(t("Active time by day") + ": " + lab)}">${days.map((d, i) => { const day = addDays(from, i), today = key(day) === t0;
    return `<span class="hmday${today ? " today" : ""}${wkc(day.getDay())}" title="${esc(`${md(day.getTime()/1000)} · ${dur(d.active || 0)}`)}"><span class="hmv">${d.active ? esc(dur(d.active)) : ""}</span><i style="height:${d.active ? Math.max(6, Math.round(100 * d.active / max)) : 0}%"></i><b>${esc(dow(day.getDay()))}</b></span>`; }).join("")}</div>`;
}
const homeStat = (k, v) => `<div class="hmstat"><div class="k">${esc(k)}</div><div class="v">${v}</div></div>`;
// homeSes は、セッション履歴のカード
function homeSes(){
  const H = homeWeek(), recent = [...DATA].sort((a, b) => b.end - a.end).slice(0, 5);
  let glance = noneH(esc(t("No records")));
  if (H){ const u = H.w.usage || {}, c = u.tokens ? costOf(u) : null;
    glance = `<h3 class="hmsub">${esc(H.label)}</h3><div class="hmstats">${homeStat(t("Active time"), dur(H.w.active || 0, true))}${homeStat(t("Sessions"), esc(commas(H.w.sessions || 0)))}${u.tokens ? homeStat(c == null ? t("Tokens") : t("Estimated cost"), c == null ? esc(tok(u.tokens)) : usdH(c)) : ""}${u.credits ? homeStat(t("Kiro credits"), crH(u.credits)) : ""}</div>${homeDays(H.w, H.from)}`; }
  return `<article class="panel hmcard"><h2 class="hmh">${ico("day")}<a href="#/sessions" data-go="sessions" data-view="cal">${esc(t("Session history"))}</a></h2>
    <p class="hmabout">${esc(t("Look back on your AI agents' sessions by week or month, on a calendar and in a summary"))}</p>
    ${glance}
    ${recent.length ? `<h3 class="hmsub">${esc(t("Recent sessions"))}</h3><div class="hmrecent">${recent.map(s => sesCardH(s, sesMeta(s, `${hm(s.start)}–${hm(s.end)}`))).join("")}</div>` : ""}
    <div class="hmgo"><a class="pill" href="#/sessions" data-go="sessions" data-view="cal">${esc(t("Open the calendar"))}</a><a class="pill" href="#/sessions" data-go="sessions" data-view="sum">${esc(t("Open the summary"))}</a></div></article>`;
}
// homePrs は、PRトリアージのカード。数は、PRトリアージを開いて読んだ答えがあるときだけ（ここから GitHub には問い合わせない）
function homePrs(){
  const on = prsOn(), d = PRS.data, about = `<p class="hmabout">${esc(t("Open pull requests that involve you, where each one is stopped, and a prompt for the next step"))}</p>`;
  if (!on) return `<article class="panel hmcard off"><h2 class="hmh">${ico("pr")}<span>${esc(t("PR triage"))}</span></h2>${about}${prsOffH()}</article>`;
  let glance = `<p class="note">${esc(t("Open it to ask GitHub through the GitHub CLI on this computer; the counts then show here too. Nothing from your history is sent."))}</p>`;
  if (d && !d.error && Array.isArray(d.items)){ const items = d.items, first = it => ["author", "review", "mention"].find(k => (it.why || []).includes(k));
    const n = k => items.filter(it => first(it) === k).length, fix = items.filter(it => first(it) === "author" && prFlow(it).kind === "fix").length;
    glance = `<h3 class="hmsub">${esc(t("From GitHub (through gh) as of {time}", {time: hm(d.fetched)}))}</h3><div class="hmstats">${homeStat(t("Opened by you"), esc(String(n("author"))) + (fix ? `<small class="hmfix">${esc(t("Needs a fix: {n}", {n: fix}))}</small>` : ""))}${homeStat(t("Waiting for your review"), esc(String(n("review"))))}${homeStat(t("Mentioning you"), esc(String(n("mention"))))}</div>`; }
  return `<article class="panel hmcard"><h2 class="hmh">${ico("pr")}<a href="#/prs" data-go="prs">${esc(t("PR triage"))}</a></h2>${about}${glance}
    <div class="hmgo"><a class="pill" href="#/prs" data-go="prs">${esc(t("Open PR triage"))}</a></div></article>`;
}
// homeSync は、ホームを開いていれば描き直す（render が呼ぶ。データが新しくなったときも）
function homeSync(){
  const V = $("#homev"); if (!V) return;
  V.hidden = st.page !== "home"; if (V.hidden){ V.textContent = ""; return; } // 隠したカードを残さない（同じ data-id の検索結果などより先に見つかり、閉じたときのフォーカスが隠れた要素へ行く）
  const had = document.activeElement && V.contains(document.activeElement) && document.activeElement.dataset.id; // 描き直しでフォーカスが消えないように
  V.innerHTML = `<h2 class="sr" id="homeh">${esc(t("Home"))}</h2><div class="hmgrid">${homeSes()}${homePrs()}</div>`;
  V.querySelectorAll(".card[data-id]").forEach(b => b.onclick = () => select(b.dataset.id));
  if (had){ const b = [...V.querySelectorAll(".card[data-id]")].find(x => x.dataset.id === had); if (b) b.focus({preventScroll: true}); }
}
