package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileTypeAlias creates a hidden value function with the resolver-selected
// closure, wraps it without executing it, and binds the non-generic alias.
func (compiler *compilerState) compileTypeAlias(statement *compilerast.TypeAliasStmt) error {
	if len(statement.TypeParameters) != 0 {
		return compiler.error(statement.Span(), "generic type aliases are not compiled")
	}
	scope := compiler.table.ScopeFor(statement, resolver.TypeAliasValue, 0)
	if scope == nil || scope.Kind != resolver.TypeAliasScope {
		return compiler.error(statement.Span(), "resolver has no type alias scope for %q", statement.Name)
	}

	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:      compiler.filename,
		module:        compiler.module,
		owner:         statement,
		table:         compiler.table,
		scope:         scope,
		codeName:      statement.Name,
		qualifiedName: compiler.childQualifiedName(statement.Name),
		firstLine:     statement.Span().Start.Line,
		codeFlags:     flags,
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		reachable:     true,
	}
	if scope.Flags&resolver.CanSeeClassScope != 0 {
		child.addFree("__classdict__")
	}
	child.initializeScopeLayout(scope)
	if err := child.compileExpr(statement.Value); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, statement.Value.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}

	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(statement.Name)),
		statement.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.MakeTypeAlias, 0, statement.Span()); err != nil {
		return err
	}
	return compiler.emitNameStore(statement.Name, statement.Span())
}
