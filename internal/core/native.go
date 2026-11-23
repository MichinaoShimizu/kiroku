package core

// エージェントごとに、そのエージェントだけが記録している数字（参考指標）。画面には出さない。
// 集計（span.go）は観測の時刻だけを使う。値は internal/source のテストが確かめる（native_test.go）。

// Measure は 1 つの観測（例: 1 ターンで使ったクレジット）。
type Measure struct {
	Key string
	T   *float64
	V   float64
}
