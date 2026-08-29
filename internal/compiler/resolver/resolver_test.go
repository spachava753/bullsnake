package resolver

import (
	"errors"
	"testing"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
)

// TestDump protects the stable representation used by checked-in resolver cases.
func TestDump(t *testing.T) {
	root := &Scope{
		Kind:        ModuleScope,
		Name:        "top",
		Flags:       UsesAnnotations,
		Symbols:     make(map[string]*Symbol),
		SymbolOrder: []string{"alpha", "omega"},
	}
	root.Symbols["alpha"] = &Symbol{Name: "alpha", Flags: Used | Assigned, Resolution: Local}
	root.Symbols["omega"] = &Symbol{Name: "omega", Flags: Used, Resolution: GlobalImplicit}
	function := &Scope{
		Kind:        FunctionScope,
		Name:        "f",
		Parent:      root,
		PrivateName: "Account",
		Parameters:  []string{"x"},
		Flags:       Nested | ReturnsValue,
		Symbols: map[string]*Symbol{
			"x":        {Name: "x", Flags: Parameter, Resolution: Cell},
			"captured": {Name: "captured", Flags: Used, Resolution: Free},
		},
		SymbolOrder: []string{"x", "captured"},
	}
	root.Children = []*Scope{function}
	table := &Table{Root: root, Features: FutureAnnotations}

	want := `Table(features=[FutureAnnotations], root=Module(name="top", private="", flags=[UsesAnnotations], parameters=[], symbols=[alpha=Local[Used, Assigned], omega=GlobalImplicit[Used]], children=[Function(name="f", private="Account", flags=[Nested, ReturnsValue], parameters=["x"], symbols=[x=Cell[Parameter], captured=Free[Used]], children=[])]))`
	if got := Dump(table); got != want {
		t.Fatalf("Dump() =\n%s\nwant:\n%s", got, want)
	}
	if got := Dump(nil); got != "nil" {
		t.Fatalf("Dump(nil) = %q, want nil", got)
	}
}

// TestScopeForDistinguishesPurposeAndItem covers AST nodes that create several scopes.
func TestScopeForDistinguishesPurposeAndItem(t *testing.T) {
	module := &compilerast.Module{}
	body := &Scope{Kind: ModuleScope, Name: "top"}
	bound := &Scope{Kind: TypeVariableScope, Name: "T bound"}
	defaultValue := &Scope{Kind: TypeVariableScope, Name: "T default"}
	table := &Table{scopes: map[scopeKey]*Scope{
		{node: module, purpose: ModuleBody}:                   body,
		{node: module, purpose: TypeVariableBound, item: 0}:   bound,
		{node: module, purpose: TypeVariableDefault, item: 0}: defaultValue,
	}}

	if got := table.ScopeFor(module, ModuleBody, 0); got != body {
		t.Fatalf("module body = %#v, want %#v", got, body)
	}
	if got := table.ScopeFor(module, TypeVariableBound, 0); got != bound {
		t.Fatalf("type variable bound = %#v, want %#v", got, bound)
	}
	if got := table.ScopeFor(module, TypeVariableDefault, 0); got != defaultValue {
		t.Fatalf("type variable default = %#v, want %#v", got, defaultValue)
	}
	if got := table.ScopeFor(module, TypeVariableDefault, 1); got != nil {
		t.Fatalf("unknown scope = %#v, want nil", got)
	}
	if got := (*Table)(nil).ScopeFor(module, ModuleBody, 0); got != nil {
		t.Fatalf("nil table scope = %#v, want nil", got)
	}
}

// TestPrivateNameMangling records Python's class-private name rules.
func TestPrivateNameMangling(t *testing.T) {
	for _, test := range []struct {
		class string
		name  string
		want  string
	}{
		{class: "Account", name: "__value", want: "_Account__value"},
		{class: "__Account", name: "__value", want: "_Account__value"},
		{class: "___", name: "__value", want: "__value"},
		{class: "Account", name: "__value__", want: "__value__"},
		{class: "Account", name: "left.__value", want: "left.__value"},
		{class: "Account", name: "normal", want: "normal"},
		{class: "", name: "__value", want: "__value"},
	} {
		t.Run(test.class+"/"+test.name, func(t *testing.T) {
			if got := manglePrivate(test.class, test.name); got != test.want {
				t.Fatalf("manglePrivate(%q, %q) = %q, want %q", test.class, test.name, got, test.want)
			}
		})
	}
}

// TestErrorFormatting protects resolver diagnostics and unknown enum formatting.
func TestErrorFormatting(t *testing.T) {
	failure := &Error{
		Kind:     SyntaxError,
		Message:  "return outside function",
		Filename: "input.py",
		Span: lexer.Span{
			Start: lexer.Position{Offset: 4, Line: 2, Column: 3},
			End:   lexer.Position{Offset: 10, Line: 2, Column: 9},
		},
	}
	if got, want := failure.Error(), "input.py:2:4: SyntaxError: return outside function"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if got := ErrorKind(255).String(); got != "ErrorKind(255)" {
		t.Fatalf("unknown error kind = %q", got)
	}
	if got := ScopeKind(255).String(); got != "ScopeKind(255)" {
		t.Fatalf("unknown scope kind = %q", got)
	}
	if got := NameScope(255).String(); got != "NameScope(255)" {
		t.Fatalf("unknown name scope = %q", got)
	}
	if got := ScopePurpose(255).String(); got != "ScopePurpose(255)" {
		t.Fatalf("unknown scope purpose = %q", got)
	}
	if got := SymbolFlags(1 << 15).String(); got != "SymbolFlags(0x8000)" {
		t.Fatalf("unknown symbol flags = %q", got)
	}
	if got := ScopeFlags(1 << 31).String(); got != "ScopeFlags(0x80000000)" {
		t.Fatalf("unknown scope flags = %q", got)
	}
}

// FuzzResolveNeverPanics feeds parsed source to the resolver and checks located errors.
func FuzzResolveNeverPanics(f *testing.F) {
	for _, source := range []string{
		"", "value = source\n", "def f(x):\n    return x\n", "[x for x in values]", "class C:\n    pass\n",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		module, err := parser.Parse("fuzz.py", source)
		if err != nil {
			return
		}
		table, err := Resolve("fuzz.py", module)
		if err != nil {
			var resolveErr *Error
			if !errors.As(err, &resolveErr) {
				t.Fatalf("Resolve error = %#v, want *resolver.Error", err)
			}
			span := resolveErr.Span
			if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset || span.End.Offset > len(source) {
				t.Fatalf("invalid span %+v for %d-byte source", span, len(source))
			}
			return
		}
		if table == nil || table.Root == nil {
			t.Fatal("Resolve returned an incomplete table without an error")
		}
		_ = Dump(table)
	})
}
