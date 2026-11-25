// 振り返り・日報のプロンプトのコピー。kiroku は AI のエージェントを動かさない（4.0 で AI の実行を外した）。
// プロンプトをクリップボードに入れるだけで、貼るエージェントと送るかどうかは利用者が決める
/* promptBtn は「プロンプトをコピー」のボタン（id はボタンの id、label は title。読み上げの名前は見えている文字のまま） */
function promptBtn(id, label){
  return `<button class="pill ai-copy" id="${id}" type="button" title="${esc(label)}">${ico("copy")}<span>${esc(t("Copy prompt"))}</span></button>`;
}
const PROMPT_COPIED = () => t("Copied the prompt. kiroku didn't run or send anything. Paste it into your AI agent.");
const copyPrompt = (button, prompt) => copy(prompt, PROMPT_COPIED(), 6000, button);
