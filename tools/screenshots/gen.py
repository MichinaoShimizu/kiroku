"""スクリーンショット用のダミーの履歴を作る（Claude Code・Codex CLI・Kiro CLI・Kiro IDE、架空の 4 プロジェクト、今日までの約 5 週間。それより前の約 11 か月は、Year in review の見本用に短い Claude Code の履歴だけ）。
スクリーンショットは英語表示で撮るので、プロンプトも英語にする。

  python3 gen.py <出力先>   → <出力先>/home/.claude/projects・<出力先>/home/.codex・<出力先>/home/.kiro と <出力先>/repos/<プロジェクト>
"""
import json, random, uuid, os, sys, datetime as dt
OUT = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "out")
REPOS = os.path.join(OUT, "repos")
random.seed(7)
extra = random.Random(11)  # 後から足したもの（コマンド・通知）は別の乱数で、ほかのダミーデータを変えない
more = random.Random(13)  # さらに後から足したもの（利用上限・サブエージェントの時間）も別の乱数で
cmpr = random.Random(17)  # コンパクションも別の乱数で
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
            if i >= 5 and cmpr.random() < (0.12 if grow else 0.02):  # 長くなった会話は、ときどき要約して文脈を空ける（コンパクション）
                ct = t - dt.timedelta(seconds=15); trig = cmpr.choice(["auto", "auto", "manual"])
                lines.append({"type":"system","subtype":"compact_boundary","timestamp":ct.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"content":"Conversation compacted","compactMetadata":{"trigger":trig,"preTokens":cmpr.randint(120000, 190000)}})
                lines.append({"type":"user","timestamp":(ct+dt.timedelta(seconds=1)).isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"isCompactSummary":True,"message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context. The conversation is summarized below."}})
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

# ほかのエージェント（Codex CLI・Kiro CLI・Kiro IDE）の履歴も作る。kiroku は複数のエージェントを 1 つの画面に並べるので、デモもそう見せる。
# Claude Code の分のあとに別の乱数で作るので、Claude Code のダミーデータは変わらない
oth = random.Random(19)
def iso(t): return t.astimezone(dt.timezone.utc).isoformat().replace("+00:00", "Z")
others = {
 "codex": {"web-app": ["Refactor the session middleware","Add rate limiting to the login API"], "data-pipeline": ["Add retries to the S3 upload step","Profile the nightly job"]},
 "kiro-cli": {"web-app": ["Write unit tests for the cart total","Explain the auth flow"], "mobile": ["Fix the layout on small screens","Add a loading state to the feed"], "docs": ["Check the docs for broken links"]},
 "kiro-ide": {"mobile": ["Implement the profile edit screen from the spec","Add offline caching to the feed"], "data-pipeline": ["Create a spec for the export API"]},
}
odone = ["Done. I updated the handler and added a test for it; all tests pass.", "I looked through the code: the logic lives in src/api.ts, and the tests are under test/. Want me to change it?", "Fixed and verified locally. The change is small, so I kept it in one file."]
CODEX = os.path.join(OUT, "home", ".codex")
KIRO = os.path.join(OUT, "home", ".kiro")
index = []
for d in range(35, -1, -1):
    day = today - dt.timedelta(days=d)
    if day.weekday() >= 5 and oth.random() < 0.7: continue
    for agent, rate in (("codex", 0.7), ("kiro-cli", 0.5), ("kiro-ide", 0.4)):
        if oth.random() >= rate: continue
        p = oth.choice(list(others[agent])); cwd = os.path.join(REPOS, p)
        start = day + dt.timedelta(hours=oth.choice([9,10,11,13,14,15,16,17,19]), minutes=oth.randint(0,50))
        if start > now - dt.timedelta(hours=2): continue
        sid = str(uuid.uuid4()); title = oth.choice(others[agent][p]); t = start
        asks = [title] + [oth.choice(follow) for _ in range(oth.randint(1, 6))]
        if agent == "codex":
            model = oth.choice(["gpt-6-sol", "gpt-6-sol", "gpt-5.5"]); tot = {"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":0}
            lines = [{"timestamp":iso(t),"type":"session_meta","payload":{"id":sid,"timestamp":iso(t),"cwd":cwd,"originator":"codex_cli_rs","cli_version":"0.80.0","source":"cli","git":{"branch":oth.choice(branches[p])}}},
                     {"timestamp":iso(t),"type":"turn_context","payload":{"cwd":cwd,"model":model}}]
            sub = None
            for i, ask in enumerate(asks):
                t += dt.timedelta(seconds=oth.randint(5, 30))
                lines.append({"timestamp":iso(t),"type":"event_msg","payload":{"type":"user_message","message":ask,"images":[]}})
                for _ in range(oth.randint(1, 4)):
                    t += dt.timedelta(seconds=oth.randint(15, 180))
                    if oth.random() < 0.5:
                        f = f"src/{oth.choice(['app','api','util','view'])}.ts"
                        lines.append({"timestamp":iso(t),"type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","input":f"*** Begin Patch\n*** Update File: {f}\n@@\n-old\n+new\n*** End Patch","call_id":"c"+uuid.uuid4().hex[:8]}})
                    else:
                        lines.append({"timestamp":iso(t),"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":json.dumps({"command":["bash","-lc",oth.choice(["rg TODO","npm test","ls src"])]}),"call_id":"c"+uuid.uuid4().hex[:8]}})
                    last = {"input_tokens":oth.randint(8000,40000),"output_tokens":oth.randint(200,3000),"reasoning_output_tokens":oth.randint(0,1500)}
                    last["cached_input_tokens"] = int(last["input_tokens"] * oth.uniform(0.5, 0.9)); last["total_tokens"] = last["input_tokens"] + last["output_tokens"]
                    for k in tot: tot[k] += last[k]
                    lines.append({"timestamp":iso(t),"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":dict(tot),"last_token_usage":last,"model_context_window":272000},"rate_limits":{"primary":{"used_percent":round(oth.uniform(5, 60), 1),"window_minutes":300},"secondary":{"used_percent":round(oth.uniform(10, 40), 1),"window_minutes":10080}}}})
                if i == 0 and sub is None and oth.random() < 0.3:  # ときどきサブエージェントに調べさせる（親の詳細に入る）
                    sub = (str(uuid.uuid4()), t)
                    t += dt.timedelta(seconds=oth.randint(60, 300))
                t += dt.timedelta(seconds=4)
                lines.append({"timestamp":iso(t),"type":"event_msg","payload":{"type":"agent_message","message":oth.choice(odone)}})
                t += dt.timedelta(seconds=oth.randint(30, 400))
            if t > now: continue
            dirn = os.path.join(CODEX, "sessions", start.strftime("%Y/%m/%d")); os.makedirs(dirn, exist_ok=True)
            with open(os.path.join(dirn, f"rollout-{start.strftime('%Y-%m-%dT%H-%M-%S')}-{sid}.jsonl"), "w") as f:
                for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
            index.append({"id":sid,"thread_name":title,"updated_at":iso(t)})
            if sub:
                cid, ct = sub; st = ct + dt.timedelta(seconds=2)
                last = {"input_tokens":oth.randint(5000,20000),"cached_input_tokens":2000,"output_tokens":oth.randint(200,1200),"reasoning_output_tokens":0}; last["total_tokens"] = last["input_tokens"] + last["output_tokens"]
                sl = [{"timestamp":iso(ct),"type":"session_meta","payload":{"id":cid,"timestamp":iso(ct),"cwd":cwd,"source":{"subagent":{"thread_spawn":{"parent_thread_id":sid,"depth":1,"agent_role":"explorer"}}}}},
                      {"timestamp":iso(ct),"type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":cid,"thread_settings":{"model":"gpt-6-luna"}}},
                      {"timestamp":iso(st),"type":"turn_context","payload":{"cwd":cwd,"model":"gpt-6-luna"}},
                      {"timestamp":iso(st),"type":"event_msg","payload":{"type":"user_message","message":"Find where this is implemented and list the related tests"}},
                      {"timestamp":iso(st+dt.timedelta(seconds=30)),"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":json.dumps({"command":["rg","-l","session"]}),"call_id":"s"+cid[:8]}},
                      {"timestamp":iso(st+dt.timedelta(seconds=60)),"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":last,"last_token_usage":last,"model_context_window":272000},"rate_limits":None}}]
                with open(os.path.join(dirn, f"rollout-{ct.strftime('%Y-%m-%dT%H-%M-%S')}-{cid}.jsonl"), "w") as f:
                    for l in sl: f.write(json.dumps(l, ensure_ascii=False)+"\n")
        elif agent == "kiro-cli":
            model = oth.choice(["claude-sonnet-4.5", "claude-sonnet-4.5", "auto"]); lines = []; turns = []
            for ask in asks:
                t += dt.timedelta(seconds=oth.randint(5, 30))
                lines.append({"kind":"Prompt","data":{"content":[{"kind":"text","data":ask}],"meta":{"timestamp":iso(t)}}})
                content = []
                for _ in range(oth.randint(1, 3)):
                    content.append({"kind":"toolUse","data":{"name":oth.choice(["fs_read","fs_write","execute_bash"]),"input":{"path":f"{cwd}/src/{oth.choice(['app','api','view'])}.ts"}}})
                    t += dt.timedelta(seconds=oth.randint(15, 150))
                content.insert(0, {"kind":"text","data":oth.choice(odone)})
                lines.append({"kind":"AssistantMessage","data":{"content":content}})
                turns.append({"end_timestamp":iso(t),"metering_usage":[{"value":round(oth.uniform(0.2, 2.5), 2),"unit":"credit"}],"total_request_count":len(content),"model":model})
                t += dt.timedelta(seconds=oth.randint(30, 400))
            if t > now: continue
            dirn = os.path.join(KIRO, "sessions", "cli"); os.makedirs(dirn, exist_ok=True)
            json.dump({"session_id":sid,"cwd":cwd,"title":title,"created_at":iso(start),"updated_at":iso(t),"session_state":{"rts_model_state":{"model_info":{"model_id":model}},"conversation_metadata":{"user_turn_metadatas":turns}}}, open(os.path.join(dirn, sid+".json"), "w"))
            with open(os.path.join(dirn, sid+".jsonl"), "w") as f:
                for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
        else:
            lines = []
            for ask in asks:
                t += dt.timedelta(seconds=oth.randint(5, 30))
                lines.append({"timestamp":iso(t),"payload":{"type":"user","content":[{"type":"text","text":ask}]}})
                for _ in range(oth.randint(1, 4)):
                    t += dt.timedelta(seconds=oth.randint(15, 150))
                    lines.append({"timestamp":iso(t),"payload":{"type":"tool_call","toolName":oth.choice(["readFile","strReplace","fsWrite"]),"args":{"path":f"src/{oth.choice(['screens','api','store'])}/{oth.choice(['profile','feed','cache'])}.ts"}}})
                t += dt.timedelta(seconds=4)
                lines.append({"timestamp":iso(t),"payload":{"type":"assistant","content":[{"type":"text","text":oth.choice(odone)}]}})
                lines.append({"timestamp":iso(t),"payload":{"type":"usage_summary","promptTurnSummaries":[{"usage":round(oth.uniform(0.3, 3), 2),"unit":"credit"}]}})
                t += dt.timedelta(seconds=oth.randint(30, 400))
            if t > now: continue
            dirn = os.path.join(KIRO, "sessions", uuid.uuid5(uuid.NAMESPACE_URL, cwd).hex[:16], "sess_"+sid); os.makedirs(dirn, exist_ok=True)
            json.dump({"id":"sess_"+sid,"title":title,"rootPaths":[cwd],"createdAt":iso(start),"modelId":"claude-sonnet-4.5"}, open(os.path.join(dirn, "session.json"), "w"))
            with open(os.path.join(dirn, "messages.jsonl"), "w") as f:
                for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
os.makedirs(CODEX, exist_ok=True)
with open(os.path.join(CODEX, "session_index.jsonl"), "w") as f:
    for l in index: f.write(json.dumps(l)+"\n")

# それより前（今日の 36 日前まで、約 11 か月）の Claude Code の履歴：Year in review の見本（docs/year.png）を 1 年ぶんにするため。
# 中身は短い依頼と返事だけ。別の乱数で最後に作るので、ここまでのダミーデータ（直近 5 週間）は変わらない。
# 春は data-pipeline、初夏から mobile、秋から web-app が中心。8 月に 2 週間の休み。後半ほど夜と並列が増える
old = random.Random(23)
for d in range(330, 35, -1):
    day = today - dt.timedelta(days=d)
    if day.month == 8 and 4 <= day.day <= 17: continue
    late = d < 150  # 後半
    if day.weekday() >= 5 and old.random() < .85: continue
    if old.random() < .12: continue
    main = "data-pipeline" if day.month <= 4 else "mobile" if day.month <= 7 else "web-app"
    for _ in range(old.choice([1, 2, 2, 3])):
        p = main if old.random() < .75 else old.choice(list(projects))
        h = old.choice([9, 10, 11, 13, 14, 15, 16] + ([19, 21, 22] if late else [17]))
        start = day + dt.timedelta(hours=h, minutes=old.randint(0, 50))
        for par in range(2 if late and old.random() < .35 else 1):  # 後半は、ときどき 2 つを並べて走らせる
            sid = str(uuid.uuid4()); cwd = os.path.join(REPOS, p); br = old.choice(branches[p]); model = old.choice(models)
            t = start + dt.timedelta(minutes=par * old.randint(5, 25)); lines = []
            for i in range(old.randint(2, 9)):
                txt = old.choice(projects[p]) if i == 0 else old.choice(follow)
                lines.append({"type":"user","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"message":{"role":"user","content":txt}})
                t += dt.timedelta(seconds=old.randint(60, 420)); mid = "msg_"+uuid.uuid4().hex[:10]
                usage = {"input_tokens":old.randint(5,200),"output_tokens":old.randint(100,3000),"cache_creation_input_tokens":old.randint(500,8000),"cache_read_input_tokens":old.randint(15000,40000)}
                lines.append({"type":"assistant","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"requestId":"req_"+mid,"message":{"id":mid,"model":model,"role":"assistant","content":[{"type":"text","text":old.choice(replies[:3])}],"usage":usage}})
                t += dt.timedelta(seconds=old.randint(30, 400))
            dirn = os.path.join(OUT, "home", ".claude", "projects", f"-Users-me-{p}"); os.makedirs(dirn, exist_ok=True)
            with open(f"{dirn}/{sid}.jsonl","w") as f:
                for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
