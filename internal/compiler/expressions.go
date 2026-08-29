package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileExpr emits one expression and leaves exactly one value on the stack.
func (compiler *compilerState) compileExpr(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		if expression.Context != compilerast.Load {
			return compiler.error(expression.Span(), "name expression is not a load")
		}
		if compiler.scope.Symbols[expression.ID] == nil {
			return compiler.error(expression.Span(), "resolver has no symbol for %q", expression.ID)
		}
		return compiler.emit(bytecode.LoadName, compiler.nameIndex(expression.ID), expression.Span())
	case *compilerast.NumberLiteral:
		constant, err := parseNumberLiteral(expression.Text)
		if err != nil {
			return compiler.error(expression.Span(), "%v", err)
		}
		return compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(constant),
			expression.Span(),
		)
	case *compilerast.StringLiteral:
		constant, err := parseStringLiteral(expression.Text)
		if err != nil {
			return compiler.error(expression.Span(), "%v", err)
		}
		return compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(constant),
			expression.Span(),
		)
	case *compilerast.StringConcatExpr:
		return compiler.compileStringConcat(expression)
	case *compilerast.NoneLiteral:
		return compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			expression.Span(),
		)
	case *compilerast.BooleanLiteral:
		return compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.Bool(expression.Value)),
			expression.Span(),
		)
	case *compilerast.EllipsisLiteral:
		return compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.Ellipsis()),
			expression.Span(),
		)
	default:
		return compiler.unsupported(expression)
	}
}
