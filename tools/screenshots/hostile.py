#!/usr/bin/env python3
# 悪意のある文字列を仕込んだ合成の履歴（Claude Code）と git リポジトリを作る（CI の e2e で XSS を確かめるため）。
# プロンプト・タイトル・ブランチ・ファイル名・コミット・モデル名・ツール名などに HTML やスクリプトを入れ、
# 同じプロンプトを 4 セッションで繰り返す（「Worth a look」の繰り返しの指摘にも出るように）。
#   python3 tools/screenshots/hostile.py <作業ディレクトリ>  → <作業ディレクトリ>/fx/projects が --root
# 日付は 2026-10-05 の週なので、kiroku html --week 2026-10-05 で書き出す。Windows ではファイル名に使えない文字があるので、Linux と macOS 用。
import json, os, subprocess, sys, uuid
S = sys.argv[1]
P = '"><svg onload=alert(11)>'
REPO = os.path.join(S, "fx", "repo" + "<img src=x onerror=alert(10)>'\"")
os.makedirs(REPO, exist_ok=True)
env = dict(os.environ, GIT_AUTHOR_DATE="2026-10-05T10:30:00Z", GIT_COMMITTER_DATE="2026-10-05T10:30:00Z",
           GIT_AUTHOR_NAME="x", GIT_AUTHOR_EMAIL="x@x", GIT_COMMITTER_NAME="x", GIT_COMMITTER_EMAIL="x@x")
def g(*a, **k): subprocess.run(["git", "-C", REPO, *a], check=True, env=env, capture_output=True, **k)
if not os.path.exists(os.path.join(REPO, ".git")):
    g("init", "-q", "-b", "main")
    g("checkout", "-q", "-b", "x<img/src=x/onerror=alert(12)>")
    fn = "f<img src=x onerror=alert(13)>.txt"
    open(os.path.join(REPO, fn), "w").write("a\n")
    g("add", "-A")
    g("commit", "-q", "-m", "<img src=x onerror=alert(14)> __META__ {{x}}   </script><script>alert(15)</script>",
      "-m", "body <img src=x onerror=alert(16)> [c](javascript:alert(17)) __DATA__")
    g("remote", "add", "origin", 'https://example.com/o/r"><img src=x onerror=alert(18)>')
root = os.path.join(S, "fx", "projects", "-p")
os.makedirs(root, exist_ok=True)
REP = "fix: <img src=x onerror=alert(20)> please fix the build again"
for i in range(4):
    sid = str(uuid.UUID(int=i+1))
    t = lambda m: f"2026-10-05T{10+i:02d}:{m:02d}:00Z"
    br = "b<img src=x onerror=alert(21)>" + P
    base = {"cwd": REPO, "gitBranch": br, "sessionId": sid}
    L = []
    L.append({"type": "custom-title", "customTitle": f"T{i} <img src=x onerror=alert(22)>{P} __META__   </script><!--", "sessionId": sid})
    L.append({**base, "type": "user", "timestamp": t(0), "message": {"role": "user", "content": REP}})
    L.append({**base, "type": "assistant", "timestamp": t(1), "requestId": f"r{i}a", "message": {"id": f"m{i}a", "model": "claude-x<img src=x onerror=alert(23)>", "role": "assistant",
      "content": [{"type": "tool_use", "id": f"tu{i}", "name": "Bash", "input": {"command": "git commit -m x && gh pr create"}}], "usage": {"input_tokens": 10, "output_tokens": 10}}})
    L.append({**base, "type": "user", "timestamp": t(2), "message": {"role": "user", "content": [{"type": "tool_result", "tool_use_id": f"tu{i}",
      "content": "https://github.com/o/r<x/pull/1 javascript:alert(1)//pull/2"}]}})
    L.append({**base, "type": "assistant", "timestamp": t(3), "requestId": f"r{i}b", "message": {"id": f"m{i}b", "model": "claude-opus-4-1", "role": "assistant",
      "content": [{"type": "tool_use", "id": f"tw{i}", "name": "Write<img src=x onerror=alert(24)>", "input": {"file_path": f"{REPO}/w<img src=x onerror=alert(25)>.py", "content": "x"}},
                  {"type": "tool_use", "id": f"ta{i}", "name": "Task", "input": {"subagent_type": "t<img src=x onerror=alert(26)>", "description": "d<img src=x onerror=alert(27)>", "prompt": "p"}}],
      "usage": {"input_tokens": 10, "output_tokens": 10}}})
    L.append({**base, "type": "user", "timestamp": t(5), "message": {"role": "user", "content": "<system-reminder>r<img src=x onerror=alert(28)></system-reminder>javascript:alert(29) `__DATA__` {{constructor}}    </script><script>alert(30)</script> " + P}})
    L.append({**base, "type": "user", "timestamp": t(6), "message": {"role": "user", "content": "<command-name>/x<img src=x onerror=alert(31)></command-name><command-args>" + P + "</command-args>"}})
    L.append({**base, "type": "user", "timestamp": t(8), "message": {"role": "user", "content": REP + " now"}})
    L.append({**base, "type": "user", "timestamp": t(7), "message": {"role": "user", "content": "<task-notification>n<b>x</b>&lt;img src=x onerror=alert(32)&gt;</task-notification>"}})
    L.append({**base, "type": "assistant", "timestamp": t(9), "requestId": f"r{i}c", "message": {"id": f"m{i}c", "model": "claude-opus-4-1", "role": "assistant", "content": [{"type": "text", "text": "ok <img src=x onerror=alert(33)>"}], "usage": {"input_tokens": 10, "output_tokens": 10}}})
    with open(os.path.join(root, sid + ".jsonl"), "w") as f:
        for x in L: f.write(json.dumps(x) + "\n")
print(REPO)
