// 振り返り・日報のプロンプトのコピー。kiroku は AI のエージェントを動かさない（4.0 で AI の実行を外した）。
// プロンプトをクリップボードに入れるだけで、貼るエージェントと送るかどうかは利用者が決める
/* promptBtn は「プロンプトをコピー」のボタン（id はボタンの id、label は読み上げの名前） */
function promptBtn(id, label){
  return `<button class="pill ai-copy" id="${id}" type="button" aria-label="${esc(label)}" title="${esc(label)}">${ico("copy")}<span>${esc(t("Copy prompt"))}</span></button>`;
}
const PROMPT_COPIED = () => t("Copied the prompt. kiroku ran nothing and sent nothing; paste it into your AI agent.");
const copyPrompt = (button, prompt) => copy(prompt, PROMPT_COPIED(), 6000, button);
