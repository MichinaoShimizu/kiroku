// 前回の続き：カレンダーの上の帯。今日より前のセッションのうち、AI が変えたファイルがまだコミットに入っていないものを並べる
/* 並べるのは履歴と git log から言える事実だけ（「終わっていない」とは言わない）。数え方：
   - 今日より前に終わったセッションのうち、前の稼働日（今日より前でいちばん新しい、セッションのあった日）か 7 日前の、早いほうから後のもの
   - AI が変えたファイル（プロジェクトの場所の下のもの。相対パスはその場所から。. と .. は解いてから比べる）が、
     そのセッションの始まりより後の、そのプロジェクトの場所を含むリポジトリの自分のコミットのどれにも入っていない
   - 同じファイルをあとのセッションが変えていれば、そちらのもの（今日のセッションが続きをしていれば、出さない）
   - プロジェクトの場所を含むリポジトリのコミットを kiroku が 1 つも読めていなければ出さない（git のないフォルダーを「コミットがない」と言わない。
     名前が同じだけの、ほかの場所のリポジトリのコミットは数えない）。プロジェクトの場所がわからないセッションも出さない
   - ファイルの一覧を切ったコミット（nFiles > files）があとにあれば出さない（入っているかもしれない）
   now は UNIX 秒、sessions は DATA、commits は META.git。返すのは {s, files}（files は残ったファイルの絶対パス）を、終わりの新しい順に。
   コミットは多い（1 つのリポジトリで数千、それぞれ数十ファイル）ので、プロジェクトごとに 1 度だけ「パス → 最後にコミットに入った時刻」の表にまとめる */
const PICK_DAYS = 7, PICK_SHOW = 3;
// pickPath は、パスを / 区切りにして . と .. と重なった区切りを解く。根より上に出るパスは ""
function pickPath(p){
  const q = String(p || "").replace(/\\/g, "/"), m = /^(?:[A-Za-z]:)?\//.exec(q), head = m ? m[0] : "", segs = [];
  for (const x of q.slice(head.length).split("/")){
    if (x === "" || x === ".") continue;
    if (x !== "..") segs.push(x); else if (!segs.length) return ""; else segs.pop();
  }
  return head + segs.join("/");
}
function leftOff(now, sessions, commits){
  const day0 = t => { const d = new Date(t * 1000); return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime() / 1000; };
  const t0 = day0(now), last = sessions.reduce((m, s) => s.end < t0 && s.end > m ? s.end : m, 0);
  if (!last) return [];
  const from = Math.min(day0(last), t0 - PICK_DAYS * 86400), out = [], idx = new Map();
  const abs = p => /^(?:[A-Za-z]:)?\//.test(p), under = (p, dir) => p === dir || p.startsWith(dir.endsWith("/") ? dir : dir + "/");
  // プロジェクトの名前ごとに：リポジトリ → 一覧を切ったコミットのいちばん新しい時刻（なければ 0）、ファイルの絶対パス → 最後にコミットに入った時刻
  const index = name => { let x = idx.get(name); if (x) return x;
    x = {repos: new Map(), last: new Map()}; idx.set(name, x);
    for (const c of commits){ if (c.project !== name) continue;
      const repo = pickPath(c.repo); if (!abs(repo)) continue;
      const fs = c.files || [], cut = (c.nFiles || 0) > fs.length ? c.t : 0, pre = repo.endsWith("/") ? repo : repo + "/";
      x.repos.set(repo, Math.max(x.repos.get(repo) || 0, cut));
      for (const f of fs){ const k = pre + f.path; if (!(x.last.get(k) >= c.t)) x.last.set(k, c.t); } }
    return x; };
  const filesOf = x => { const r = pickPath(x.projectPath); return (x.files || []).map(q => { const p = String(q || "").replace(/\\/g, "/"); return pickPath(abs(p) || !r ? p : r + "/" + p); }); }; // 相対パスで記録するエージェントは、プロジェクトの場所から
  for (const s of sessions){
    if (s.end < from || s.end >= t0 || !(s.nFiles > 0)) continue;
    const root = pickPath(s.projectPath); if (!abs(root)) continue;
    const x = index(s.project), repos = [...x.repos].filter(([repo]) => under(root, repo));
    if (!repos.length || repos.some(([, cut]) => cut >= s.start - 60)) continue;
    const later = sessions.filter(y => y !== s && y.project === s.project && y.end > s.end).map(y => new Set(filesOf(y)));
    const files = [...new Set(filesOf(s))].filter(p => p && p !== root && under(p, root) && !(x.last.get(p) >= s.start - 60) && !later.some(y => y.has(p)));
    if (files.length) out.push({s, files});
  }
  return out.sort((a, b) => b.s.end - a.s.end);
}
/* 「今」。kiroku serve は今の時刻。HTML のファイル（デモなど）は、作った時刻より先へは進めない（作ったあとのコミットを知らないまま
   「入っていない」と言わないよう、ファイルを作った時点の話として出す）。期間だけのファイルには出さない */
function pickNow(){
  if (!META || META.scope) return null;
  return LIVE ? nowMs() / 1000 : Math.min(nowMs() / 1000, GENERATED);
}
const pickHidden = () => { const v = store.get("pickHidden", []); return Array.isArray(v) ? v.filter(x => typeof x === "string") : []; };
// pickItem は 1 件：セッションのカード（押すと詳細）、入っていないファイル、最後に書いたプロンプト、再開のコマンドのコピー、一覧から隠すボタン。
// ファイルはプロジェクトの場所からのパスで出す（同じ名前のファイルを見分けられるように）。長ければ終わりの 2 つの部分だけ。
// 履歴の文字（題・プロジェクト・ブランチ・パス・プロンプト・コマンド）は、どれも textContent か属性の値として入れる（HTML としては読ませない）
function pickItem({s, files}){
  const root = pickPath(s.projectPath), pre = root.endsWith("/") ? root : root + "/", rel = p => p.startsWith(pre) ? p.slice(pre.length) : p;
  const short = p => { const r = rel(p); return r.length <= 32 ? r : "…/" + r.split("/").slice(-2).join("/"); };
  const n = files.length, shown = files.slice(0, 3), rest = n - shown.length;
  const last = [...(s.prompts || [])].reverse().find(p => !p.kind && String(p.text || "").trim());
  const txt = last ? String(last.text).replace(/\s+/g, " ").trim() : "", lim = (s.limits || []).some(t0 => t0 >= s.end - 120);
  const li = elx("li", "pki"), fl = elx("p", "pkl"), act = elx("div", "pka"), hide = elx("button", "pill pkd", t("Hide"));
  fl.title = files.map(rel).join("\n"); fl.append(elx("span", "pkk", `${t("Not in a commit yet")} (${n})`), ...shown.map(p => elx("code", "", short(p))));
  if (rest > 0) fl.append(elx("span", "pkm", JA ? t("{n} more files", {n: rest}) : plural(rest, "more file")));
  li.append(sesCardEl(s, `${md(s.end)} ${hm(s.start)}–${hm(s.end)} · ${s.project}${s.branch ? ` · ${s.branch}` : ""}`), fl);
  if (txt){ const q = elx("p", "pkq"); q.append(elx("span", "pkk", t("Last prompt")), " " + (txt.length > 160 ? txt.slice(0, 159) + "…" : txt)); li.append(q); }
  if (lim){ const w = elx("p", "pkl pklim"); w.append(icoEl("limit"), t("The session ended at a usage limit")); li.append(w); }
  if (s.resume){ const c = copyBtnEl(t("Copy the resume command"), s.resume); c.title = s.resume; act.append(c); }
  hide.type = "button"; hide.dataset.pkd = s.id; hide.setAttribute("aria-label", t("Hide {title} from this list", {title: s.title}));
  act.append(hide); li.append(act); return li;
}
// pickList は、今の「前回の続き」。render のたびに呼ばれるので、データ（自動更新で入れかわる）と日付が同じあいだは前の結果を使う
let pickMemo = null;
function pickList(now){
  const day = key(new Date(now * 1000)), g = META.git || [];
  if (!pickMemo || pickMemo.d !== DATA || pickMemo.g !== g || pickMemo.day !== day) pickMemo = {d: DATA, g, day, all: leftOff(now, DATA, g)};
  return pickMemo.all;
}
// pickup は帯を描く。ここで失敗しても、カレンダーは描く（帯を消して続ける）
function pickup(){
  const el = $("#pick"); if (!el) return;
  try { pickDraw(el); } catch(e){ console.error(e); el.hidden = true; el.textContent = ""; }
}
function pickDraw(el){
  const now = pickNow(), {ws, we} = period();
  const all = now != null && !st.q && now >= ws && now < we ? pickList(now) : [];
  el.hidden = !all.length; el.textContent = "";
  if (!all.length) return;
  const hid = new Set(pickHidden()), xs = all.filter(x => !hid.has(x.s.id)), nh = all.length - xs.length;
  // 狭い画面では 1 件（カレンダーを下へ押しやらない）。帯は畳める（畳むと見出しと件数の 1 行。この端末に覚える）
  const fold = store.get("pickFold", false) === true, cap = mq.matches ? 1 : PICK_SHOW, more = xs.length > cap && !st.pickAll, show = fold ? [] : more ? xs.slice(0, cap) : xs;
  const from = all.reduce((m, x) => Math.min(m, x.s.end), Infinity);
  const head = elx("div", "pkh"), h = elx("h2", "", t("Pick up where you left off")), foot = elx("div", "pkf"); h.id = "pick-h";
  const fb = elx("button", "pill pkfold", fold ? t("Show") : t("Collapse")); fb.type = "button"; fb.id = "pkfold"; fb.setAttribute("aria-expanded", String(!fold)); fb.setAttribute("aria-controls", "pick-b");
  fb.onclick = () => { store.set("pickFold", !fold); pickup(); $("#pkfold").focus(); };
  el.classList.toggle("fold", fold);
  head.append(h, fold ? elx("p", "", plural(xs.length, "session")) : elx("p", "", t("Sessions since {date} where AI changed files that are in none of your commits since then. Today's sessions are left out.", {date: md(from)})), fb);
  const body = elx("div", "pkb"); body.id = "pick-b"; body.hidden = fold;
  el.append(head, body);
  if (show.length){ const ul = elx("ul", "pks"); ul.setAttribute("role", "list"); ul.append(...show.map(pickItem)); body.append(ul); }
  // 隠す・戻すと、押したボタンは消える。フォーカスは帯に残す（ページの頭に戻さない）
  const again = () => { pickup(); const f = el.querySelector("[data-id], #pkback"); if (f) f.focus(); };
  if (more){ const b = elx("button", "pill", t("Show the remaining {n}", {n: xs.length - cap})); b.type = "button"; b.id = "pkmore";
    b.onclick = () => { st.pickAll = true; pickup(); const f = el.querySelectorAll("[data-id]")[cap]; if (f) f.focus(); }; foot.append(b); }
  else if (st.pickAll && xs.length > cap){ const b = elx("button", "pill", t("Show fewer")); b.type = "button"; b.id = "pkless";
    b.onclick = () => { st.pickAll = false; pickup(); const f = el.querySelector("#pkmore"); if (f) f.focus(); }; foot.append(b); }
  if (nh){ const b = elx("button", "pill", t("Show them again")); b.type = "button"; b.id = "pkback";
    b.onclick = () => { const ids = new Set(all.map(x => x.s.id)); store.set("pickHidden", pickHidden().filter(x => !ids.has(x))); again(); };
    foot.append(elx("span", "pkn", t("{n} hidden", {n: nh})), b); }
  body.append(foot);
  bindCopy(el);
  el.querySelectorAll("[data-id]").forEach(b => b.onclick = () => select(b.dataset.id));
  el.querySelectorAll("[data-pkd]").forEach(b => b.onclick = () => { store.set("pickHidden", [...pickHidden().filter(x => x !== b.dataset.pkd), b.dataset.pkd].slice(-200)); again(); });
}
