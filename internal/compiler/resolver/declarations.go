package resolver

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func (state *resolver) collectGlobal(statement *compilerast.GlobalStmt) error {
	for _, name := range statement.Names {
		if err := state.recordDeclaration(name, GlobalDeclaration, statement.Span()); err != nil {
			return err
		}
	}
	return nil
}

func (state *resolver) collectNonlocal(statement *compilerast.NonlocalStmt) error {
	if state.current.Kind == ModuleScope {
		return state.syntaxError(statement.Span(), "nonlocal declaration not allowed at module level")
	}
	for _, name := range statement.Names {
		if err := state.recordDeclaration(name, NonlocalDeclaration, statement.Span()); err != nil {
			return err
		}
	}
	return nil
}

func (state *resolver) recordDeclaration(name string, declaration SymbolFlags, span lexer.Span) error {
	name = manglePrivate(state.current.PrivateName, name)
	if symbol := state.current.Symbols[name]; symbol != nil {
		if err := state.declarationConflict(name, symbol.Flags, declaration, span); err != nil {
			return err
		}
	}
	state.record(name, declaration, span)
	return nil
}

// declarationConflict reports the first source-order use or binding that makes
// a global or nonlocal declaration invalid.
func (state *resolver) declarationConflict(name string, flags, declaration SymbolFlags, span lexer.Span) error {
	other := GlobalDeclaration
	word := "nonlocal"
	if declaration == GlobalDeclaration {
		other = NonlocalDeclaration
		word = "global"
	}
	switch {
	case flags&other != 0:
		return state.syntaxError(span, "name %q is nonlocal and global", name)
	case flags&Parameter != 0:
		return state.syntaxError(span, "name %q is parameter and %s", name, word)
	case flags&Used != 0:
		return state.syntaxError(span, "name %q is used prior to %s declaration", name, word)
	case flags&Annotated != 0:
		return state.syntaxError(span, "annotated name %q cannot be %s", name, word)
	case flags&(Assigned|Imported) != 0:
		return state.syntaxError(span, "name %q is assigned before %s declaration", name, word)
	default:
		return nil
	}
}
