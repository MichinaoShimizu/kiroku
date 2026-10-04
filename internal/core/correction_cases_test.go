package core

// correctionCases は、言い直しの判定を確かめる例。first は会話の最初の依頼か。
var correctionCases = []struct {
	text  string
	first bool
	want  bool
}{
	// 言い直し・差し戻し（拾う）
	{"違う、そうじゃなくて", false, true},
	{"違います。ヘッダーではなくフッターです", false, true},
	{"いや、そうじゃない", false, true},
	{"それは違うよ", false, true},
	{"やり直して", false, true},
	{"さっきの変更は元に戻して", false, true},
	{"今の編集を取り消して", false, true},
	{"まだ直ってない", false, true},
	{"ボタンじゃなくてリンクにして", false, true},
	{"No, that's wrong", false, true},
	{"nope", false, true},
	{"Revert that and try again", false, true},
	{"It still doesn't work", false, true},
	{"The build is still failing", false, true},
	{"Roll back the last change", false, true},
	{"That's not what I asked for", false, true},
	{"Undo your last edit", false, true},
	{"you broke the login page", false, true},
	{"Start over with a simpler approach", false, true},
	{"テストは通ったけど、\nやっぱり元に戻して", false, true},

	// ふつうの依頼（拾わない）
	{"Fix validation on the login form", false, false},
	{"Add a working example", false, false},
	{"Write the release notes", false, false},
	{"Make the tests pass too", false, false},
	{"テストも通して", false, false},
	{"違う色にして目立たせて", false, false},
	{"違う方法も試してみて", false, false},
	{"ログインとは違う画面を作る", false, false},
	{"取り消しボタンを追加して", false, false},
	{"やり直し機能を実装して", false, false},
	{"Add an undo button to the editor", false, false},
	{"Implement undo/redo for the canvas", false, false},
	{"Add a rollback step to the deploy script", false, false},
	{"Write a migration that can be reverted", false, false},
	{"Revert commit abc1234 on the release branch", false, false},
	{"Show a 'Try again' button when the request fails", false, false},
	{"Handle the case where the token is wrong", false, false},
	{"エラーになる。\n```\nError: connection doesn't work\n```", false, false},
	{"このログを見て\n> TypeError: x is not a function\n> still failing after retry", false, false},
	{"Here is the error:\n    at main.go:12 doesn't work", false, false},

	// 会話の最初の依頼は、まだ直すものがないので拾わない
	{"The login button doesn't work", true, false},
	{"ログインが動かない。前のバージョンに戻して", true, false},

	// 調整に使っていない例（判定を決めたあとに足した）
	{"違うってば", false, true},
	{"いや、そっちじゃない", false, true},
	{"これじゃなくて前のやつ", false, true},
	{"Nope, revert it", false, true},
	{"That is wrong, use the other API", false, true},
	{"it's still broken", false, true},
	{"Please undo the last commit", false, true},
	{"No. Use tabs.", false, true},
	{"Wrong file, I meant main.go", false, true},
	{"変更を戻してください", false, true},
	{"直ってないよ", false, true},
	{"No need to add tests", false, false},
	{"違うブランチで作業して", false, false},
	{"Add retry logic and try again on 5xx", false, false},
	{"Implement an undo stack", false, false},
	{"Revert the dependency bump in go.mod", false, false},
	{"Update the wrong-password message", false, false},
	{"取り消し線のスタイルを追加", false, false},
	{"今度は別のファイルを直して", false, false},
	{"Is this the right approach?", false, false},
}
