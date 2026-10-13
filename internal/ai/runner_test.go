package ai

import (
 "context"
 "bytes"
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

func TestLimitedWriter(t *testing.T) {
 var b bytes.Buffer
 w:=&limitedWriter{w:&b,max:3}
 n,err:=w.Write([]byte("abcdef"))
 if err==nil || n!=3 || b.String()!="abc" { t.Fatalf("n=%d err=%v output=%q",n,err,b.String()) }
 n,err=w.Write([]byte("z"))
 if err==nil || n!=0 { t.Fatalf("expected full writer error, n=%d err=%v",n,err) }
}
func TestCancellationBeforeExecution(t *testing.T) {
 ctx,cancel:=context.WithCancel(context.Background())
 cancel()
 _,err:=(Runner{Enabled:true}).Run(ctx,"claude","hello")
 if err==nil { t.Fatal("expected canceled or missing binary error") }
}
