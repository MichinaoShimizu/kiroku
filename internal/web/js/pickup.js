// 前回の続き：カレンダーの上の帯。今日より前のセッションのうち、AI が変えたファイルがまだコミットに入っていないものを並べる
/* 並べるのは履歴と git log から言える事実だけ（「終わっていない」とは言わない）。数え方：
   - 今日より前に終わったセッションのうち、前の稼働日（今日より前でいちばん新しい、セッションのあった日）か 7 日前の、早いほうから後のもの
   - AI が変えたファイル（プロジェクトの場所の下のもの。相対パスはその場所から）が、そのセッションの始まりより後の、同じプロジェクトの自分のコミットのどれにも入っていない
   - 同じファイルをあとのセッションが変えていれば、そちらのもの（今日のセッションが続きをしていれば、出さない）
   - kiroku が自分のコミットを 1 つも読めていないプロジェクトは出さない（git のないフォルダーを「コミットがない」と言わない）
   - ファイルの一覧を切ったコミット（nFiles > files）があとにあれば出さない（入っているかもしれない）
   now は UNIX 秒、sessions は DATA、commits は META.git。返すのは {s, files}（files は残ったファイルのパス）を、終わりの新しい順に */
const PICK_DAYS = 7, PICK_SHOW = 3;
const pickPath = p => String(p || "").replace(/\\/g, "/");
function leftOff(now, sessions, commits){
  const day0 = t => { const d = new Date(t * 1000); return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime() / 1000; };
  const t0 = day0(now), last = sessions.reduce((m, s) => s.end < t0 && s.end > m ? s.end : m, 0);
  if (!last) return [];
  const from = Math.min(day0(last), t0 - PICK_DAYS * 86400), out = [];
  const abs = p => /^(?:\/|[A-Za-z]:\/)/.test(p);
  for (const s of sessions){
    if (s.end < from || s.end >= t0 || !(s.nFiles > 0)) continue;
    const mine = commits.filter(c => c.project === s.project);
    if (!mine.length) continue;
    const cs = mine.filter(c => c.t >= s.start - 60);
    if (cs.some(c => (c.nFiles || 0) > (c.files || []).length)) continue;
    const trim = p => pickPath(p).replace(/\/+$/, ""), root = trim(s.projectPath);
    const full = (p, r) => abs(p) || !r ? p : r + "/" + p; // 相対パスで記録するエージェントは、プロジェクトの場所から
    // コミットのパスはリポジトリのルートから。プロジェクトの場所がルートなら同じパスだけ、ルートの下のフォルダーなら終わりが合うもの
    const inCommit = p => { const rel = root && p.startsWith(root + "/") ? p.slice(root.length + 1) : abs(p) ? null : p;
      return cs.some(c => (c.files || []).some(f => rel == null ? p.endsWith("/" + f.path) : f.path === rel || (trim(c.repo) !== root && f.path.endsWith("/" + rel)))); };
    const later = sessions.filter(x => x !== s && x.project === s.project && x.end > s.end).map(x => { const r = trim(x.projectPath); return new Set((x.files || []).map(q => full(pickPath(q), r))); });
    const files = [...new Set((s.files || []).map(q => full(pickPath(q), root)))].filter(p => (!root || p.startsWith(root + "/")) && !inCommit(p) && !later.some(x => x.has(p)));
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
  const root = pickPath(s.projectPath).replace(/\/+$/, ""), rel = p => root && p.startsWith(root + "/") ? p.slice(root.length + 1) : p;
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
function pickup(){
  const el = $("#pick"); if (!el) return;
  const now = pickNow(), {ws, we} = period();
  let all = [];
  try { if (now != null && !st.q && now >= ws && now < we) all = leftOff(now, DATA, META.git || []); } catch(e){ console.error(e); } // ここで失敗しても、カレンダーは描く
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
