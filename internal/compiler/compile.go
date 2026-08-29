// Package compiler turns resolved Bullsnake AST modules into bytecode.
package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// Compile compiles one parsed and resolved file-input module.
func Compile(filename string, module *compilerast.Module, table *resolver.Table) (*bytecode.Code, error) {
	if module == nil {
		return nil, &Error{Message: "nil module", Filename: filename}
	}
	if table == nil || table.Root == nil || table.Root.Kind != resolver.ModuleScope {
		return nil, &Error{Message: "missing module symbol table", Filename: filename, Span: module.Span()}
	}
	state := &compilerState{
		filename:    filename,
		module:      module,
		table:       table,
		scope:       table.Root,
		constantIDs: make(map[bytecode.Constant]uint32),
		nameIDs:     make(map[string]uint32),
		reachable:   true,
	}
	if err := state.compileStatements(module.Body); err != nil {
		return nil, err
	}
	position := module.Span().End
	if position.Line == 0 {
		position.Line = 1
	}
	span := lexer.Span{Start: position, End: position}
	if err := state.emit(bytecode.LoadConst, state.constantIndex(bytecode.None()), span); err != nil {
		return nil, err
	}
	if err := state.emit(bytecode.ReturnValue, 0, span); err != nil {
		return nil, err
	}
	return state.finish()
}
