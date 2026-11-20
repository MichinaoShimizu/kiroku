package core

import "testing"

// A session an agent kept of kiroku's own "Analyze with AI" run (it ran in os.MkdirTemp("", "kiroku-ai-"))
// is left out: it isn't yours, and it holds a copy of the prompt and of the history the agent read.
func TestAIRunSessionsAreLeftOut(t *testing.T) {
	for dir, want := range map[string]bool{
		"/tmp/kiroku-ai-3172964588":                  true,
		"/var/folders/x/T/kiroku-ai-12":              true,
		`C:\Users\me\AppData\Local\Temp\kiroku-ai-7`: true,
		"/home/me/src/kiroku-ai-tools":               false,
		"/home/me/src/kiroku-ai-":                    false,
		"/home/me/src/kiroku":                        false,
		"":                                           false,
	} {
		if got := IsAIRunDir(dir); got != want {
			t.Errorf("IsAIRunDir(%q) = %v, want %v", dir, got, want)
		}
	}
	ts := 1.7e9
	b := NewBuilder("Claude Code", "run")
	b.Project = "/tmp/kiroku-ai-42"
	b.Tick(&ts)
	b.Prompt(&ts, "summarize this history")
	if s := b.Finish(15); s != nil {
		t.Errorf("an AI run became a session: %+v", s.Title)
	}
	b = NewBuilder("Claude Code", "mine")
	b.Project = "/home/me/src/app"
	b.Tick(&ts)
	if b.Finish(15) == nil {
		t.Error("an ordinary session was left out")
	}
}
