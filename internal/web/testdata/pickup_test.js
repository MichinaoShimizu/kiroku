/* leftOff（pickup.js）の確かめ。「前回の続き」に出すのは、履歴と git log から言える事実だけであること。
   pickup.js はほかのファイルに頼るので、PICK_DAYS・pickPath・leftOff だけを取り出して動かす（script_test.go。時刻は UTC） */
const eq = (got, want, msg) => { if (JSON.stringify(got) !== JSON.stringify(want)){ console.error(`${msg}: ${JSON.stringify(got)} ではなく ${JSON.stringify(want)} のはず`); process.exitCode = 1; } };
const DAY = 86400, T0 = Date.UTC(2026, 9, 14) / 1000, NOW = T0 + 15 * 3600; // 今は 10 月 14 日 15:00
const ses = (id, end, files, more = {}) => ({id, project: "app", projectPath: "/w/app", start: end - 1800, end, nFiles: files.length, files, ...more});
const com = (t, paths, more = {}) => ({project: "app", repo: "/w/app", t, files: paths.map(path => ({path})), nFiles: paths.length, ...more});
const OLD = com(T0 - 30 * DAY, ["README.md"]); // kiroku がこのプロジェクトのコミットを読めている印
const ids = (ss, cs) => leftOff(NOW, ss, cs).map(x => `${x.s.id}:${x.files.join(",")}`);

const y = T0 - 6 * 3600; // 昨日の 18:00
eq(ids([ses("a", y, ["/w/app/src/a.go", "/w/app/src/b.go"])], [OLD]), ["a:/w/app/src/a.go,/w/app/src/b.go"], "その後のコミットがなければ、変えたファイルを全部");
eq(ids([ses("a", y, ["/w/app/src/a.go", "/w/app/src/b.go"])], [OLD, com(y + 60, ["src/a.go"])]), ["a:/w/app/src/b.go"], "コミットに入ったファイルは除く");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], [OLD, com(y + 60, ["src/a.go"])]), [], "全部コミットに入っていれば出さない");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], [OLD, com(y - 7200, ["src/a.go"])]), ["a:/w/app/src/a.go"], "セッションより前のコミットは数えない");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], [OLD, com(y + 60, ["a.go"])]), ["a:/w/app/src/a.go"], "名前が同じだけの別のファイル（a.go と src/a.go）は入ったことにしない");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], [OLD, com(y + 60, ["src/a.go"], {project: "other"})]), ["a:/w/app/src/a.go"], "ほかのプロジェクトのコミットは数えない");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], []), [], "コミットを 1 つも読めていないプロジェクトは出さない（git のないフォルダーかもしれない）");
eq(ids([ses("a", y, ["/w/app/src/a.go"])], [OLD, com(y + 60, ["x.go"], {nFiles: 90})]), [], "一覧を切ったコミットがあとにあれば出さない（入っているかもしれない）");
eq(ids([ses("a", y, ["/tmp/plan.md", "/w/app2/x.go"])], [OLD]), [], "プロジェクトの場所の外のファイルは数えない（/w/app2 は /w/app の下ではない）");
eq(ids([ses("a", y, ["src/a.go"])], [OLD]), ["a:/w/app/src/a.go"], "相対パスで記録するエージェントのファイルは、プロジェクトの場所から数える");
eq(ids([ses("a", y, ["src/a.go", "b.go"])], [OLD, com(y + 60, ["src/a.go"])]), ["a:/w/app/b.go"], "相対パスのファイルも、コミットに入れば除く");
eq(ids([ses("a", y, ["/w/app/a.go"])], [OLD, com(y + 60, ["other/a.go"])]), ["a:/w/app/a.go"], "同じパスだけを入ったとみなす（終わりが同じ別のファイルは数えない）");
eq(ids([ses("a", y, ["/w/app/a.go", "/w/app/b.go"])], [com(y + 60, ["app/a.go"], {repo: "/w"})]), ["a:/w/app/b.go"], "プロジェクトがリポジトリの下のフォルダーなら、ルートからのパスで比べる");
eq(ids([ses("a", y, ["/w/app/a.go"]), ses("b", NOW - 600, ["a.go"])], [OLD]), [], "あとのセッションが相対パスで記録していても、同じファイルとみなす");
eq(ids([ses("a", y, ["C:\\w\\app\\src\\a.go", "C:\\w\\app\\b.go"], {projectPath: "C:\\w\\app"})], [com(y + 60, ["src/a.go"], {repo: "C:\\w\\app"})]), ["a:C:/w/app/b.go"], "Windows のパスも同じに比べる");
eq(ids([ses("a", y, ["/w/app/../../etc/passwd", "/w/app/src/../../../x", "/w/app/./src//a.go"])], [OLD]), ["a:/w/app/src/a.go"], ".. でプロジェクトの場所の外へ出るパスは数えず、. と重なった区切りは解く");
eq(ids([ses("a", y, ["/not-git/app/x.txt"], {projectPath: "/not-git/app"})], [com(y - DAY, ["y.txt"], {repo: "/other/app"})]), [], "名前が同じだけの、ほかの場所のリポジトリのコミットでは出さない");
eq(ids([ses("a", y, ["/etc/passwd", "x.go"], {projectPath: ""})], [OLD]), [], "プロジェクトの場所がわからないセッションは出さない");
eq(ids([ses("a", y, ["/w/app/a.go"])], [com(y - DAY, ["a.go"], {repo: ""})]), [], "リポジトリの場所のないコミットは数えない");
{ // 速さ：kiroku が読む上限（1 つのリポジトリで 5000 コミット、それぞれ 40 ファイル）でも、画面を固めない
  const cs = Array.from({length: 5000}, (_, i) => com(y + 60 + i, Array.from({length: 40}, (_, j) => `d${i % 50}/f${j}.go`)));
  const ss = Array.from({length: 40}, (_, i) => ses("s" + i, y - i * 60, Array.from({length: 30}, (_, j) => `/w/app/n${i}/g${j}.go`)));
  const t = Date.now(), n = leftOff(NOW, ss, cs).length;
  eq([n, Date.now() - t < 1500], [40, true], `コミット 5000 × 40 ファイル、セッション 40 × 30 ファイルで 1.5 秒以内（${Date.now() - t} ms）`); }
eq(ids([ses("a", y, [], {nFiles: 0})], [OLD]), [], "ファイルを変えていないセッションは出さない");
eq(ids([ses("a", NOW - 600, ["/w/app/src/a.go"])], [OLD]), [], "今日のセッションは出さない");
eq(ids([ses("a", y, ["/w/app/src/a.go", "/w/app/src/b.go"]), ses("b", NOW - 600, ["/w/app/src/a.go"])], [OLD]), ["a:/w/app/src/b.go"], "今日のセッションが続きをしたファイルは除く");
eq(ids([ses("a", y - 3600, ["/w/app/src/a.go"]), ses("b", y, ["/w/app/src/a.go"])], [OLD]), ["b:/w/app/src/a.go"], "同じファイルは、あとで変えたセッションのもの");
eq(ids([ses("a", y - 3 * DAY, ["/w/app/a.go"]), ses("b", y, ["/w/app/b.go"])], [OLD]), ["b:/w/app/b.go", "a:/w/app/a.go"], "7 日前までを、終わりの新しい順に");
eq(ids([ses("a", T0 - 9 * DAY, ["/w/app/a.go"]), ses("b", y, ["/w/app/b.go"])], [OLD]), ["b:/w/app/b.go"], "7 日より前は出さない");
eq(ids([ses("a", T0 - 20 * DAY + 3600, ["/w/app/a.go"]), ses("z", T0 - 20 * DAY + 7200, [], {nFiles: 0})], [OLD]), ["a:/w/app/a.go"], "長く空いても、前の稼働日の分は出す");
eq(ids([], [OLD]), [], "セッションがなければ空");
