package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileAttribute(expression *compilerast.AttributeExpr) error {
	if expression.Context != compilerast.Load {
		return compiler.error(expression.Span(), "attribute expression is not a load")
	}
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	return compiler.emit(bytecode.LoadAttr, compiler.nameIndex(expression.Name), expression.Span())
}

func (compiler *compilerState) compileSubscript(expression *compilerast.SubscriptExpr) error {
	if expression.Context != compilerast.Load {
		return compiler.error(expression.Span(), "subscript expression is not a load")
	}
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression.Index); err != nil {
		return err
	}
	return compiler.emit(bytecode.BinarySubscript, 0, expression.Span())
}

// compileSlice evaluates present bounds in source order and substitutes None
// for omitted lower and upper bounds before building one slice value.
func (compiler *compilerState) compileSlice(expression *compilerast.SliceExpr) error {
	for _, bound := range []compilerast.Expr{expression.Lower, expression.Upper} {
		if bound == nil {
			if err := compiler.emit(
				bytecode.LoadConst,
				compiler.constantIndex(bytecode.None()),
				expression.Span(),
			); err != nil {
				return err
			}
			continue
		}
		if err := compiler.compileExpr(bound); err != nil {
			return err
		}
	}
	count := uint32(2)
	if expression.Step != nil {
		if err := compiler.compileExpr(expression.Step); err != nil {
			return err
		}
		count = 3
	}
	return compiler.emit(bytecode.BuildSlice, count, expression.Span())
}
