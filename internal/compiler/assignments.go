package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func (compiler *compilerState) compileNamedExpression(expression *compilerast.NamedExpr) error {
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, expression.Span()); err != nil {
		return err
	}
	return compiler.compileStore(expression.Target)
}

// compileStore consumes one assigned value and evaluates any address
// expressions before emitting the target-specific store operation.
func (compiler *compilerState) compileStore(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "name target is not a store")
		}
		if compiler.scope.Symbols[expression.ID] == nil {
			return compiler.error(expression.Span(), "resolver has no symbol for %q", expression.ID)
		}
		return compiler.emit(bytecode.StoreName, compiler.nameIndex(expression.ID), expression.Span())
	case *compilerast.AttributeExpr:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "attribute target is not a store")
		}
		if err := compiler.compileExpr(expression.Value); err != nil {
			return err
		}
		return compiler.emit(bytecode.StoreAttr, compiler.nameIndex(expression.Name), expression.Span())
	case *compilerast.SubscriptExpr:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "subscript target is not a store")
		}
		if err := compiler.compileExpr(expression.Value); err != nil {
			return err
		}
		if err := compiler.compileExpr(expression.Index); err != nil {
			return err
		}
		return compiler.emit(bytecode.StoreSubscript, 0, expression.Span())
	case *compilerast.TupleExpr:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "tuple target is not a store")
		}
		return compiler.compileSequenceStore(expression.Elements, expression.Span())
	case *compilerast.ListExpr:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "list target is not a store")
		}
		return compiler.compileSequenceStore(expression.Elements, expression.Span())
	case *compilerast.StarredExpr:
		return compiler.error(expression.Span(), "starred assignment targets are not compiled")
	default:
		return compiler.unsupported(expression)
	}
}

// compileSequenceStore unpacks one fixed or starred value and recursively
// consumes the resulting target values from left to right.
func (compiler *compilerState) compileSequenceStore(elements []compilerast.Expr, span lexer.Span) error {
	starIndex := -1
	for index, element := range elements {
		starred, ok := element.(*compilerast.StarredExpr)
		if !ok {
			continue
		}
		if starred.Context != compilerast.Store {
			return compiler.error(starred.Span(), "starred target is not a store")
		}
		if starIndex >= 0 {
			return compiler.error(starred.Span(), "multiple starred assignment targets")
		}
		starIndex = index
	}

	if starIndex < 0 {
		if err := compiler.emit(bytecode.UnpackSequence, uint32(len(elements)), span); err != nil {
			return err
		}
	} else {
		operand, ok := bytecode.PackUnpackEx(
			uint32(starIndex),
			uint32(len(elements)-starIndex-1),
		)
		if !ok {
			return compiler.error(span, "too many expressions in starred assignment")
		}
		if err := compiler.emit(bytecode.UnpackEx, operand, span); err != nil {
			return err
		}
	}

	for index, element := range elements {
		if index == starIndex {
			element = element.(*compilerast.StarredExpr).Value
		}
		if err := compiler.compileStore(element); err != nil {
			return err
		}
	}
	return nil
}
