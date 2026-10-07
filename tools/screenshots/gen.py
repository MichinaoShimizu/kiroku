"""スクリーンショット用のダミーの Claude Code の履歴を作る（架空の 4 プロジェクト、今日までの約 5 週間）。
スクリーンショットは英語表示で撮るので、プロンプトも英語にする。

  python3 gen.py <出力先>   → <出力先>/home/.claude/projects と <出力先>/repos/<プロジェクト>
"""
import json, random, uuid, os, sys, datetime as dt
OUT = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "out")
REPOS = os.path.join(OUT, "repos")
random.seed(7)
extra = random.Random(11)  # 後から足したもの（コマンド・通知）は別の乱数で、ほかのダミーデータを変えない
more = random.Random(13)  # さらに後から足したもの（利用上限・サブエージェントの時間）も別の乱数で
JST = dt.timezone(dt.timedelta(hours=9))
projects = {
 "web-app": ["Fix validation on the login form","Add E2E tests for the checkout flow","Find out why the dashboard loads slowly","Clean up API error handling","Update dependencies"],
 "data-pipeline": ["Investigate the failing nightly batch","Write a migration for the schema change","Speed up the aggregation query","Fix garbled characters in CSV import"],
 "mobile": ["Build the push notification settings screen","Analyze the crash logs","Add dark mode support"],
 "docs": ["Tidy up the README","Update the API reference","Write the release notes"],
}
branches = {"web-app":["main","feat/checkout","fix/login"],"data-pipeline":["main","feat/migrate"],"mobile":["main","feat/push"],"docs":["main"]}
models = ["claude-opus-5-5"]*3 + ["claude-sonnet-5-5"]*4 + ["claude-haiku-4-5-20251001"]
retry = ["No, that's not what I meant","Revert that and try again","Check it once more"]
follow = ["Continue","Make the tests pass too","Show me the diff","Looks good, next","There is a type error","Make it simpler"]
tools = ["Read","Edit","Bash","Grep","Write"]
# 1 ターンの最後に人へ返す文（依頼の流れに畳んで出る）。最後のものは長く、切られた応答の見え方も撮れるようにする。
# コミットに触れる文は、そのターンにコミットしたときだけ使う（コミットのないセッションで「コミットした」と言わないように）
committed = "I changed three files and ran the tests: 42 passed, 0 failed. The diff is small, so I kept it as one commit."
replies = [
 "Done. Validation now runs on submit, and the test covers the empty-email case.",
 "Fixed. The query had no index, so it scanned the whole table; the nightly batch now finishes in about 40 seconds.",
 "I ran the tests after the change: 42 passed, 0 failed.",
 "I could not reproduce it. The log shows a 500 from the payment stub, so the test environment looks like it is missing PAYMENT_URL. Shall I add it to .env.example?",
 "Added dark mode behind a setting, reusing the theme tokens that were already there. The screen follows the system setting on first launch and remembers the choice after that. I left the chart colours alone: they need their own palette to stay readable on a dark background, which looks like a change of its own.",
]
now = dt.datetime.now(JST)
today = now.replace(hour=0,minute=0,second=0,microsecond=0)
last_wed = today - dt.timedelta(days=today.weekday() + 5)  # 先週の水曜。スクリーンショットとテストで開く先週に、利用上限の当たりを 1 つ入れる
forced = False
for d in range(35, -1, -1):
    day = today - dt.timedelta(days=d)
    wk = day.weekday()
    n = random.choice([0,1,1,2]) if wk>=5 else random.choice([2,3,3,4,5])
    for _ in range(n):
        p = random.choices(list(projects), weights=[5,3,2,1])[0]
        start = day + dt.timedelta(hours=random.choice([9,10,11,13,14,15,16,17,20,22]), minutes=random.randint(0,50))
        if start > now - dt.timedelta(minutes=30): continue
        sid = str(uuid.uuid4()); cwd = os.path.join(REPOS, p); br = random.choice(branches[p])
        model = random.choice(models)
        t = start; lines = []; title = None
        nprompt = random.randint(2, 14)
        grow = random.choice([0, 0, 1, 4]); step = 0  # 会話が長くなるほど文脈が増えるセッションを混ぜる
        for i in range(nprompt):
            txt = random.choice(projects[p]) if i == 0 else (random.choice(retry) if random.random()<0.08 else random.choice(follow))
            title = title or txt; did = False  # did: このターンにコミットしたか
            if i > 0 and extra.random() < 0.12:  # スラッシュコマンドも、人が打ったプロンプトとして混ぜる
                c = extra.choice(["review", "test", "commit"]); txt = f"<command-message>{c} is running…</command-message>\n<command-name>/{c}</command-name>\n<command-args></command-args>"
            if i > 0 and extra.random() < 0.03:  # ログをそのまま貼った、長すぎるプロンプトも混ぜる
                txt = "This test fails, can you fix it?\n" + "\n".join(f"  at handler (src/app.ts:{100+j}:7) Error: expected 200 but got 500" for j in range(extra.randint(70, 140)))
            if i > 0 and extra.random() < 0.08:  # 自動で入るもの（通知・注記）も混ぜる
                note = extra.choice(["<task-notification><summary>Background tests finished: 42 passed</summary></task-notification>", "<system-reminder>The user opened src/app.ts in the IDE.</system-reminder>"])
                lines.append({"type":"user","timestamp":(t-dt.timedelta(seconds=20)).isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"isMeta":True,"message":{"role":"user","content":note}})
            lines.append({"type":"user","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"message":{"role":"user","content":txt}})
            for k in range(random.randint(1,5)):
                t += dt.timedelta(seconds=random.randint(20,240))
                mid = "msg_"+uuid.uuid4().hex[:10]
                usage = {"input_tokens":random.randint(5,200),"output_tokens":random.randint(100,3000),"cache_creation_input_tokens":random.randint(500,8000),"cache_read_input_tokens":random.randint(15000,40000) + step * grow * 5000}; step += 1
                tool = random.choice(tools); trying = False
                if i == nprompt-1 and k == 0 and random.random() < 0.6: tool = "Commit"
                if tool == "Commit":
                    tool = "Bash"; trying = True; inp = {"command": f"git add -A && git commit -m '{title}'" + (" && git push && gh pr create --fill" if random.random()<0.3 else "")}
                elif tool == "Edit":
                    n1 = random.randint(1,6); n2 = random.randint(1,20)
                    inp = {"file_path":f"{cwd}/src/app.ts","old_string":"\n".join(f"old{j}" for j in range(n1)),"new_string":"\n".join(f"new{j}" for j in range(n2))}
                elif tool == "Write":
                    inp = {"file_path":f"{cwd}/src/new.ts","content":"\n".join("x" for _ in range(random.randint(10,80)))}
                else:
                    inp = {"file_path":f"{cwd}/src/{random.choice(['app','api','util','view'])}.ts"} if tool=="Read" else {"command":"npm test"} if tool=="Bash" else {"pattern":"TODO"}
                content = [{"type":"tool_use","id":"t"+mid,"name":tool,"input":inp}]
                task = random.random()<0.05
                if task:
                    trying = False  # 呼び出しがサブエージェントに替わったら、コミットはしていない
                    content = [{"type":"tool_use","id":"t"+mid,"name":"Task","input":{"subagent_type":random.choice(["Explore","general-purpose"]),"description":"Explore the code","prompt":"Find the related code"}}]
                lines.append({"type":"assistant","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"requestId":"req_"+mid,"message":{"id":mid,"model":model,"role":"assistant","content":content,"usage":usage}})
                out = "ok"
                if "gh pr create" in content[0]["input"].get("command", ""):
                    out = f"https://github.com/example/{p}/pull/{random.randint(10, 300)}"
                tr = {"type":"tool_result","tool_use_id":content[0]["id"],"content":out}
                if random.random() < 0.04: tr["is_error"] = True
                did = did or (trying and not tr.get("is_error"))  # 失敗したコミットは、コミットしたことにしない
                took = dt.timedelta(seconds=more.randint(60, 540) if task else 5)  # サブエージェントは、結果が返るまで数分かかる
                lines.append({"type":"user","timestamp":(t+took).isoformat(),"cwd":cwd,"sessionId":sid,"message":{"role":"user","content":[tr]}})
                if task: t += took
            if extra.random() < 0.85:  # 人に返した文（ツール呼び出しのあと、次の依頼の前）
                rid = "msg_"+uuid.uuid4().hex[:10]
                text = extra.choice(replies)
                if did: text = committed
                lines.append({"type":"assistant","timestamp":(t+dt.timedelta(seconds=6)).isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"requestId":"req_"+rid,"message":{"id":rid,"model":model,"role":"assistant","content":[{"type":"text","text":text}]}})
            t += dt.timedelta(seconds=random.randint(30, 600))
        force = day == last_wed and not forced and t.hour >= 13; forced = forced or force
        if more.random() < 0.06 or force:  # ときどき利用上限に当たって止まる（Claude Code が作るエラーの発言。そこでセッションも終わる）
            t += dt.timedelta(seconds=more.randint(20, 90)); rid = "msg_"+uuid.uuid4().hex[:10]
            reset = (t + dt.timedelta(hours=more.randint(1, 4))).replace(minute=0, second=0, microsecond=0)
            lines.append({"type":"assistant","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"isApiErrorMessage":True,"message":{"id":rid,"model":"<synthetic>","role":"assistant","content":[{"type":"text","text":f"5-hour limit reached ∙ resets {reset.strftime('%-I%p').lower()}"}]}})
        if t > now: continue
        dirn = os.path.join(OUT, "home", ".claude", "projects", f"-Users-me-{p}"); os.makedirs(dirn, exist_ok=True)
        with open(f"{dirn}/{sid}.jsonl","w") as f:
            for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
# 履歴を長く残す設定にしておく（既定の 30 日のままだと、画面に「過去の履歴が消えます」のお知らせが出る）
os.makedirs(os.path.join(OUT, "home", ".claude"), exist_ok=True)
json.dump({"cleanupPeriodDays": 3650}, open(os.path.join(OUT, "home", ".claude", "settings.json"), "w"))
