package compiler

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// emitNameLoad selects namespace, fast-local, closure, or global access from
// the current resolver scope.
func (compiler *compilerState) emitNameLoad(name string, span lexer.Span) error {
	symbol, err := compiler.resolvedSymbol(name, span)
	if err != nil {
		return err
	}
	if compiler.scope.Kind != resolver.FunctionScope {
		return compiler.emit(bytecode.LoadName, compiler.nameIndex(name), span)
	}
	switch symbol.Resolution {
	case resolver.Local:
		index, err := compiler.localIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.LoadFast, index, span)
	case resolver.GlobalExplicit, resolver.GlobalImplicit:
		return compiler.emit(bytecode.LoadGlobal, compiler.nameIndex(name), span)
	case resolver.Cell, resolver.Free:
		index, err := compiler.derefIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.LoadDeref, index, span)
	default:
		return compiler.error(span, "name %q has unresolved scope", name)
	}
}

// emitNameStore selects namespace, fast-local, closure, or global storage from
// the resolver classification.
func (compiler *compilerState) emitNameStore(name string, span lexer.Span) error {
	symbol, err := compiler.resolvedSymbol(name, span)
	if err != nil {
		return err
	}
	if compiler.scope.Kind != resolver.FunctionScope {
		return compiler.emit(bytecode.StoreName, compiler.nameIndex(name), span)
	}
	switch symbol.Resolution {
	case resolver.Local:
		index, err := compiler.localIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.StoreFast, index, span)
	case resolver.GlobalExplicit, resolver.GlobalImplicit:
		return compiler.emit(bytecode.StoreGlobal, compiler.nameIndex(name), span)
	case resolver.Cell, resolver.Free:
		index, err := compiler.derefIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.StoreDeref, index, span)
	default:
		return compiler.error(span, "name %q has unresolved scope", name)
	}
}

// emitNameDelete selects namespace, fast-local, closure, or global deletion from
// the resolver classification.
func (compiler *compilerState) emitNameDelete(name string, span lexer.Span) error {
	symbol, err := compiler.resolvedSymbol(name, span)
	if err != nil {
		return err
	}
	if compiler.scope.Kind != resolver.FunctionScope {
		return compiler.emit(bytecode.DeleteName, compiler.nameIndex(name), span)
	}
	switch symbol.Resolution {
	case resolver.Local:
		index, err := compiler.localIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.DeleteFast, index, span)
	case resolver.GlobalExplicit, resolver.GlobalImplicit:
		return compiler.emit(bytecode.DeleteGlobal, compiler.nameIndex(name), span)
	case resolver.Cell, resolver.Free:
		index, err := compiler.derefIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		return compiler.emit(bytecode.DeleteDeref, index, span)
	default:
		return compiler.error(span, "name %q has unresolved scope", name)
	}
}

func (compiler *compilerState) resolvedSymbol(name string, span lexer.Span) (*resolver.Symbol, error) {
	symbol := compiler.scope.Symbols[name]
	if symbol == nil {
		return nil, compiler.error(span, "resolver has no symbol for %q", name)
	}
	return symbol, nil
}

func (compiler *compilerState) localIndex(name string) (uint32, error) {
	index, ok := compiler.localIDs[name]
	if !ok {
		return 0, fmt.Errorf("local table has no entry for %q", name)
	}
	return index, nil
}

func (compiler *compilerState) derefIndex(name string) (uint32, error) {
	index, ok := compiler.derefIDs[name]
	if !ok {
		return 0, fmt.Errorf("closure table has no entry for %q", name)
	}
	return index, nil
}
