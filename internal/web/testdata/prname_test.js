// prName・ghPull: GitHub の PR の短い名前は、行き先が https://github.com/ のときだけ。ほかは URL のまま見せる。
const cases = [
  ["https://github.com/o/repo/pull/12", "repo#12", true],
  ["https://www.github.com/o/repo/pull/3/files", "repo#3", true],
  ["https://evil.example/github.com/attacker/fake/pull/1", "evil.example/github.com/attacker/fake/pull/1", false], // github.com の PR に見せない
  ["https://github.com.evil.example/o/r/pull/1", "github.com.evil.example r#1", false], // ホストごと見せるので、github.com のものには見えない
  ["http://github.com/o/r/pull/1", "github.com/o/r/pull/1", false],
  ["https://github.com/o/r/pull/12x", "github.com/o/r/pull/12x", false],
  ["https://gitlab.com/o/r/-/merge_requests/4", "gitlab.com/o/r/-/merge_requests/4", false],
  ["https://ghe.acme.internal/platform/web-app/pull/7", "ghe.acme.internal web-app#7", false], // GitHub Enterprise：ホストを残す
  ["https://evil.example/attacker/fake/pull/1", "evil.example fake#1", false],
  ["", "", false],
];
let bad = 0;
for (const [url, name, gh] of cases) {
  const got = prName(url), isGh = !!ghPull(url);
  if (got !== name || isGh !== gh) { console.error(`prName(${JSON.stringify(url)}) = ${JSON.stringify(got)} (github ${isGh}), want ${JSON.stringify(name)} (github ${gh})`); bad++; }
}
if (bad) process.exit(1);
