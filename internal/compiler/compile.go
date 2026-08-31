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
	firstLine := 1
	if len(module.Body) != 0 {
		firstLine = module.Body[0].Span().Start.Line
	}
	state := &compilerState{
		filename:      filename,
		module:        module,
		owner:         module,
		table:         table,
		scope:         table.Root,
		codeName:      "<module>",
		qualifiedName: "<module>",
		firstLine:     firstLine,
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		reachable:     true,
	}
	if table.Root.Flags&resolver.UsesAnnotations != 0 {
		if table.Features&resolver.FutureAnnotations != 0 {
			if err := state.emitFutureAnnotationsMap(module.Span()); err != nil {
				return nil, err
			}
		} else {
			if err := state.emit(bytecode.BuildSet, 0, module.Span()); err != nil {
				return nil, err
			}
			if err := state.emit(
				bytecode.StoreName,
				state.nameIndex(conditionalAnnotationsName),
				module.Span(),
			); err != nil {
				return nil, err
			}
		}
	}
	if err := state.compileStatements(module.Body); err != nil {
		return nil, err
	}
	if err := state.compileDeferredAnnotations(); err != nil {
		return nil, err
	}
	position := module.Span().End
	if position.Line == 0 {
		position.Line = 1
	}
	if err := state.emitImplicitReturn(lexer.Span{Start: position, End: position}); err != nil {
		return nil, err
	}
	return state.finish()
}
