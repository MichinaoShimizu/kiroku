"""スクリーンショット用のダミーの Claude Code の履歴を作る（架空の 4 プロジェクト、今日までの約 5 週間）。
スクリーンショットは英語表示で撮るので、依頼文も英語にする。

  python3 gen.py <出力先>   → <出力先>/home/.claude/projects と <出力先>/repos/<プロジェクト>
"""
import json, random, uuid, os, sys, datetime as dt
OUT = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "out")
REPOS = os.path.join(OUT, "repos")
random.seed(7)
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
now = dt.datetime.now(JST)
today = now.replace(hour=0,minute=0,second=0,microsecond=0)
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
        t = start; lines = []
        nprompt = random.randint(2, 14)
        grow = random.choice([0, 0, 1, 4]); step = 0  # 会話が長くなるほど文脈が増えるセッションを混ぜる
        for i in range(nprompt):
            txt = random.choice(projects[p]) if i == 0 else (random.choice(retry) if random.random()<0.08 else random.choice(follow))
            lines.append({"type":"user","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"message":{"role":"user","content":txt}})
            for k in range(random.randint(1,5)):
                t += dt.timedelta(seconds=random.randint(20,240))
                mid = "msg_"+uuid.uuid4().hex[:10]
                usage = {"input_tokens":random.randint(5,200),"output_tokens":random.randint(100,3000),"cache_creation_input_tokens":random.randint(500,8000),"cache_read_input_tokens":random.randint(15000,40000) + step * grow * 5000}; step += 1
                tool = random.choice(tools)
                if i == nprompt-1 and k == 0 and random.random() < 0.6: tool = "Commit"
                if tool == "Commit":
                    tool = "Bash"; inp = {"command": "git add -A && git commit -m 'update'" + (" && git push && gh pr create --fill" if random.random()<0.3 else "")}
                elif tool == "Edit":
                    n1 = random.randint(1,6); n2 = random.randint(1,20)
                    inp = {"file_path":f"{cwd}/src/app.ts","old_string":"\n".join(f"old{j}" for j in range(n1)),"new_string":"\n".join(f"new{j}" for j in range(n2))}
                elif tool == "Write":
                    inp = {"file_path":f"{cwd}/src/new.ts","content":"\n".join("x" for _ in range(random.randint(10,80)))}
                else:
                    inp = {"file_path":f"{cwd}/src/{random.choice(['app','api','util','view'])}.ts"} if tool=="Read" else {"command":"npm test"} if tool=="Bash" else {"pattern":"TODO"}
                content = [{"type":"tool_use","id":"t"+mid,"name":tool,"input":inp}]
                if random.random()<0.05:
                    content = [{"type":"tool_use","id":"t"+mid,"name":"Task","input":{"subagent_type":random.choice(["Explore","general-purpose"]),"description":"Explore the code","prompt":"Find the related code"}}]
                lines.append({"type":"assistant","timestamp":t.isoformat(),"cwd":cwd,"gitBranch":br,"sessionId":sid,"requestId":"req_"+mid,"message":{"id":mid,"model":model,"role":"assistant","content":content,"usage":usage}})
                out = "ok"
                if "gh pr create" in content[0]["input"].get("command", ""):
                    out = f"https://github.com/example/{p}/pull/{random.randint(10, 300)}"
                tr = {"type":"tool_result","tool_use_id":content[0]["id"],"content":out}
                if random.random() < 0.04: tr["is_error"] = True
                lines.append({"type":"user","timestamp":(t+dt.timedelta(seconds=5)).isoformat(),"cwd":cwd,"sessionId":sid,"message":{"role":"user","content":[tr]}})
            t += dt.timedelta(seconds=random.randint(30, 600))
        if t > now: continue
        dirn = os.path.join(OUT, "home", ".claude", "projects", f"-Users-me-{p}"); os.makedirs(dirn, exist_ok=True)
        with open(f"{dirn}/{sid}.jsonl","w") as f:
            for l in lines: f.write(json.dumps(l, ensure_ascii=False)+"\n")
# 履歴を長く残す設定にしておく（既定の 30 日のままだと、画面に「過去の履歴が消えます」のお知らせが出る）
os.makedirs(os.path.join(OUT, "home", ".claude"), exist_ok=True)
json.dump({"cleanupPeriodDays": 3650}, open(os.path.join(OUT, "home", ".claude", "settings.json"), "w"))
