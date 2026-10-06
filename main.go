// kiroku — AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の作業履歴を週カレンダーで振り返る。
//
// 中身は internal/cli にある。ここは go install github.com/MichinaoShimizu/kiroku@latest で入れられるよう、入口だけを置く。
package main

import "github.com/MichinaoShimizu/kiroku/internal/cli"

var version = "dev" // リリース時に -ldflags で入れる

func main() { cli.Main(version) }
