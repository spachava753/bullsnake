package resolver

import (
	"fmt"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type resolver struct {
	filename              string
	table                 *Table
	current               *Scope
	annotations           map[*Scope]*Scope
	comprehensionIterable int
	control               controlContext
}

type controlContext struct {
	loopStarDepths  []int
	exceptStarDepth int
}

func (state *resolver) bind(name string, flags SymbolFlags, span lexer.Span) (string, error) {
	if name == "__debug__" {
		return "", state.syntaxError(span, "cannot assign to __debug__")
	}
	return state.record(name, flags, span), nil
}

// record merges one source occurrence into the current scope while preserving
// the first occurrence, binding, declaration, and insertion order.
func (state *resolver) record(name string, flags SymbolFlags, span lexer.Span) string {
	name = manglePrivate(state.current.PrivateName, name)
	symbol := state.current.Symbols[name]
	if symbol == nil {
		symbol = &Symbol{Name: name, FirstSeen: span}
		state.current.Symbols[name] = symbol
		state.current.SymbolOrder = append(state.current.SymbolOrder, name)
	}
	symbol.Flags |= flags
	if isBinding(flags) && symbol.FirstBinding == (lexer.Span{}) {
		symbol.FirstBinding = span
	}
	if flags&(GlobalDeclaration|NonlocalDeclaration) != 0 && symbol.Declaration == (lexer.Span{}) {
		symbol.Declaration = span
	}
	return name
}

func isBinding(flags SymbolFlags) bool {
	return flags&(Assigned|Parameter|Imported|TypeParameter) != 0
}

func unsupportedNode(node compilerast.Node) error {
	return fmt.Errorf("resolver: unsupported syntax node %T", node)
}
