"""gen.py の履歴に合わせて、ダミーの git リポジトリを作る（AI が実行したコミットと、手で行ったコミット、ときどきの push）。

  python3 mkgit.py <出力先>
"""
import json, glob, os, subprocess, random, sys, datetime as dt
OUT = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "out")
random.seed(3)
push=random.Random(7) # push の分は別の乱数にして、コミットの中身は変えない
R=os.path.join(OUT,'repos')
ev=[]
for f in glob.glob(os.path.join(OUT,'home','.claude','projects','*','*.jsonl')):
    for l in open(f):
        o=json.loads(l)
        if o['type']!='assistant': continue
        for c in o['message']['content']:
            if c.get('name')=='Bash' and 'git commit' in c['input'].get('command',''):
                ev.append((o['cwd'], dt.datetime.fromisoformat(o['timestamp']).timestamp()+3, 'AI', 'update'))
# hand commits
projs=set(e[0] for e in ev) | {f"{R}/{p}" for p in ["web-app","data-pipeline","mobile","docs"]}
now=dt.datetime.now().timestamp()
for p in projs:
    for _ in range(random.randint(6,14)):
        t=now-random.uniform(0,35)*86400
        ev.append((p,t,'hand',random.choice(["fix typo","refactor","update deps","add tests","tweak styles"])))
ev.sort(key=lambda e:e[1])
for p in projs:
    os.makedirs(p,exist_ok=True)
    if not os.path.isdir(p+'/.git'):
        subprocess.run(['git','-C',p,'init','-q']); subprocess.run(['git','-C',p,'config','user.email','me@example.com']); subprocess.run(['git','-C',p,'config','user.name','me']); subprocess.run(['git','-C',p,'remote','add','origin',f'git@github.com:example/{os.path.basename(p)}.git'])
for i,(p,t,kind,msg) in enumerate(ev):
    with open(f"{p}/f{i%7}.txt","a") as fh: fh.write("line\n"*random.randint(1,40))
    subprocess.run(['git','-C',p,'add','-A'])
    d=dt.datetime.fromtimestamp(t).astimezone().isoformat()
    env=dict(os.environ,GIT_AUTHOR_DATE=d,GIT_COMMITTER_DATE=d)
    subprocess.run(['git','-C',p,'commit','-qm',msg],env=env,check=True)
    if push.random() < 0.35: # ときどき push した（リモートには送れないので、push と同じ reflog の行だけを残す）
        pt=dt.datetime.fromtimestamp(t+push.uniform(60,1200)).astimezone().isoformat()
        subprocess.run(['git','-C',p,'update-ref','-m','update by push','refs/remotes/origin/main','HEAD'],env=dict(os.environ,GIT_COMMITTER_DATE=pt),check=True)
print(len(ev))
