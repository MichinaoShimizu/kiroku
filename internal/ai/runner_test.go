package ai

import (
 "context"
 "errors"
 "strings"
 "testing"
)

func TestDisabled(t *testing.T) {
 _,err:=(Runner{}).Run(context.Background(),"claude","hello")
 if !errors.Is(err,ErrDisabled) { t.Fatalf("expected disabled, got %v",err) }
}
func TestUnsupportedAgent(t *testing.T) {
 _,err:=(Runner{Enabled:true}).Run(context.Background(),"shell","hello")
 if !errors.Is(err,ErrUnsupportedAgent) { t.Fatalf("expected unsupported, got %v",err) }
}
func TestEmptyPrompt(t *testing.T) {
 _,err:=(Runner{Enabled:true}).Run(context.Background(),"claude"," ")
 if err==nil { t.Fatal("expected error") }
}
func TestOversizedPrompt(t *testing.T) {
 _,err:=(Runner{Enabled:true}).Run(context.Background(),"claude",strings.Repeat("x",256*1024+1))
 if err==nil { t.Fatal("expected error") }
}
