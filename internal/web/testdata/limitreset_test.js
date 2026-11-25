// review.js の limitReset（script_test.go の TestLimitReset）。TZ=Asia/Tokyo で動かす
const eq = (got, want) => { if (got !== want){ console.error(`${JSON.stringify(got)} != ${JSON.stringify(want)}`); process.exitCode = 1; } };
const T = 1791244800; // 2026-10-06 09:00 JST
const s = {limits: [T, T + 3600, T + 7200, T + 10800], limitResetAt: [T + 5 * 3600, T + 6 * 86400, 0], limitResets: ["", "", "9pm"]};
eq(limitReset(s, 0), "14:00");            // 同じ日: 時刻だけ（今いる時間帯で）
eq(limitReset(s, 1), "Mon, Oct 12 09:00"); // 違う日: 日付も
eq(limitReset(s, 2), "21:00");            // 時刻がなければ、エラー文の時刻を 24 時間に
eq(limitReset(s, 3), "");                 // どちらもない
eq(limitReset({limits: [T], limitResetAt: ["<b>x</b>"]}, 0), ""); // 数でない値は使わない
eq(limitReset({limits: [T]}, 0), "");
for (const bad of [NaN, -5, Infinity, 1e20, T - 60, "<img src=x>"]) eq(limitReset({limits: [T], limitResetAt: [bad], limitResets: ["9pm"]}, 0), "21:00"); // 使えない時刻なら文のほうへ
