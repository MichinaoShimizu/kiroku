// 振り返り・日報のプロンプトのコピー。kiroku は AI のエージェントを動かさない（4.0 で AI の実行を外した）。
// プロンプトをクリップボードに入れるだけで、貼るエージェントと送るかどうかは利用者が決める
/* promptBtn は「プロンプトをコピー」のボタン（id はボタンの id、label は title。読み上げの名前は見えている文字のまま） */
function promptBtn(id, label){
  return `<button class="pill ai-copy" id="${id}" type="button" title="${esc(label)}">${ico("copy")}<span>${esc(t("Copy prompt"))}</span></button>`;
}
const PROMPT_COPIED = () => t("Copied the prompt. kiroku didn't run or send anything. Paste it into your AI agent.");
// 依頼文がまだ届いていないとき（kiroku serve で開いた直後）は、文の抜けたものをコピーしない（そう言って true）
const textsPending = () => { if (TEXTS) return false; toast(t("Still loading your prompts. Try again in a moment."), 3000); return true; };
const copyPrompt = (button, prompt) => { if (!textsPending()) copy(prompt, PROMPT_COPIED(), 6000, button); };
