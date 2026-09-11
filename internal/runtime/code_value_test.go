package runtime_test

import (
	"testing"

	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestCodeValueRuntimeIdentity(t *testing.T) {
	code := compileSource(t, "def f(): pass\ncode = f.__code__\n")
	firstRuntime := bullruntime.New()
	first, err := firstRuntime.ExecuteModule("first", code)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := firstRuntime.ExecuteModule("repeated", code)
	if err != nil {
		t.Fatal(err)
	}
	second, err := bullruntime.New().ExecuteModule("second", code)
	if err != nil {
		t.Fatal(err)
	}
	firstCode, _ := first.Get("code")
	repeatedCode, _ := repeated.Get("code")
	secondCode, _ := second.Get("code")
	if firstCode != repeatedCode || firstCode == secondCode {
		t.Fatal("Python code wrappers must share prepared identity only within one runtime")
	}
}
