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
	if (compiler.scope.Kind == resolver.AnnotationScope ||
		compiler.scope.Kind == resolver.TypeAliasScope) &&
		compiler.scope.Flags&resolver.CanSeeClassScope != 0 {
		handled, classErr := compiler.emitClassVisibleNameLoad(name, symbol, span)
		if classErr != nil {
			return classErr
		}
		if handled {
			return nil
		}
	}
	switch compiler.scope.Kind {
	case resolver.ModuleScope:
		return compiler.emit(bytecode.LoadName, compiler.nameIndex(name), span)
	case resolver.ClassScope:
		switch symbol.Resolution {
		case resolver.Cell, resolver.Free:
			index, indexErr := compiler.derefIndex(name)
			if indexErr != nil {
				return compiler.error(span, "%v", indexErr)
			}
			return compiler.emit(bytecode.LoadDeref, index, span)
		case resolver.GlobalExplicit:
			return compiler.emit(bytecode.LoadGlobal, compiler.nameIndex(name), span)
		default:
			return compiler.emit(bytecode.LoadName, compiler.nameIndex(name), span)
		}
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

// emitClassVisibleNameLoad checks a captured class namespace before the normal
// global or enclosing-cell fallback used by a class-visible child scope.
func (compiler *compilerState) emitClassVisibleNameLoad(
	name string,
	symbol *resolver.Symbol,
	span lexer.Span,
) (bool, error) {
	var opcode bytecode.Opcode
	var operand uint32
	switch symbol.Resolution {
	case resolver.GlobalImplicit:
		opcode = bytecode.LoadFromDictOrGlobals
		operand = compiler.nameIndex(name)
	case resolver.Cell, resolver.Free:
		opcode = bytecode.LoadFromDictOrDeref
		index, err := compiler.derefIndex(name)
		if err != nil {
			return false, compiler.error(span, "%v", err)
		}
		operand = index
	default:
		return false, nil
	}
	classDict, err := compiler.derefIndex("__classdict__")
	if err != nil {
		return false, compiler.error(span, "%v", err)
	}
	if err := compiler.emit(bytecode.LoadDeref, classDict, span); err != nil {
		return false, err
	}
	return true, compiler.emit(opcode, operand, span)
}

// emitNameStore selects namespace, fast-local, closure, or global storage from
// the resolver classification.
func (compiler *compilerState) emitNameStore(name string, span lexer.Span) error {
	symbol, err := compiler.resolvedSymbol(name, span)
	if err != nil {
		return err
	}
	switch compiler.scope.Kind {
	case resolver.ModuleScope:
		return compiler.emit(bytecode.StoreName, compiler.nameIndex(name), span)
	case resolver.ClassScope:
		switch symbol.Resolution {
		case resolver.Cell, resolver.Free:
			index, indexErr := compiler.derefIndex(name)
			if indexErr != nil {
				return compiler.error(span, "%v", indexErr)
			}
			return compiler.emit(bytecode.StoreDeref, index, span)
		case resolver.GlobalExplicit:
			return compiler.emit(bytecode.StoreGlobal, compiler.nameIndex(name), span)
		default:
			return compiler.emit(bytecode.StoreName, compiler.nameIndex(name), span)
		}
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
	switch compiler.scope.Kind {
	case resolver.ModuleScope:
		return compiler.emit(bytecode.DeleteName, compiler.nameIndex(name), span)
	case resolver.ClassScope:
		switch symbol.Resolution {
		case resolver.Cell, resolver.Free:
			index, indexErr := compiler.derefIndex(name)
			if indexErr != nil {
				return compiler.error(span, "%v", indexErr)
			}
			return compiler.emit(bytecode.DeleteDeref, index, span)
		case resolver.GlobalExplicit:
			return compiler.emit(bytecode.DeleteGlobal, compiler.nameIndex(name), span)
		default:
			return compiler.emit(bytecode.DeleteName, compiler.nameIndex(name), span)
		}
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
