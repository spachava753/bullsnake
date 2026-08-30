package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileExpr emits one expression and leaves exactly one value on the stack.
func (compiler *compilerState) compileExpr(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		if expression.Context != compilerast.Load {
			return compiler.error(expression.Span(), "name expression is not a load")
		}
		return compiler.emitNameLoad(expression.ID, expression.Span())
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
	case *compilerast.FormattedStringExpr:
		return compiler.compileFormattedString(expression)
	case *compilerast.StringConcatExpr:
		return compiler.compileStringConcat(expression)
	case *compilerast.TupleExpr:
		if expression.Context != compilerast.Load {
			return compiler.error(expression.Span(), "tuple expression is not a load")
		}
		return compiler.compileIterableDisplay(
			expression.Elements,
			bytecode.BuildTuple,
			bytecode.BuildList,
			bytecode.ListAppend,
			bytecode.ListExtend,
			true,
			expression.Span(),
		)
	case *compilerast.ListExpr:
		if expression.Context != compilerast.Load {
			return compiler.error(expression.Span(), "list expression is not a load")
		}
		return compiler.compileIterableDisplay(
			expression.Elements,
			bytecode.BuildList,
			bytecode.BuildList,
			bytecode.ListAppend,
			bytecode.ListExtend,
			false,
			expression.Span(),
		)
	case *compilerast.SetExpr:
		return compiler.compileIterableDisplay(
			expression.Elements,
			bytecode.BuildSet,
			bytecode.BuildSet,
			bytecode.SetAdd,
			bytecode.SetUpdate,
			false,
			expression.Span(),
		)
	case *compilerast.DictExpr:
		return compiler.compileDictDisplay(expression)
	case *compilerast.ListComprehensionExpr:
		return compiler.compileEagerComprehension(
			expression,
			expression.Clauses,
			"<listcomp>",
			resolver.ListComprehension,
			bytecode.BuildList,
			func(child *compilerState) error {
				return child.appendComprehensionValue(
					expression.Element,
					bytecode.ListAppend,
				)
			},
		)
	case *compilerast.SetComprehensionExpr:
		return compiler.compileEagerComprehension(
			expression,
			expression.Clauses,
			"<setcomp>",
			resolver.SetComprehension,
			bytecode.BuildSet,
			func(child *compilerState) error {
				return child.appendComprehensionValue(expression.Element, bytecode.SetAdd)
			},
		)
	case *compilerast.UnaryExpr:
		return compiler.compileUnary(expression)
	case *compilerast.BinaryExpr:
		return compiler.compileBinary(expression)
	case *compilerast.BooleanExpr:
		return compiler.compileBoolean(expression)
	case *compilerast.CompareExpr:
		return compiler.compileComparison(expression)
	case *compilerast.ConditionalExpr:
		return compiler.compileConditional(expression)
	case *compilerast.NamedExpr:
		return compiler.compileNamedExpression(expression)
	case *compilerast.LambdaExpr:
		return compiler.compileLambdaExpression(expression)
	case *compilerast.AttributeExpr:
		return compiler.compileAttribute(expression)
	case *compilerast.SubscriptExpr:
		return compiler.compileSubscript(expression)
	case *compilerast.SliceExpr:
		return compiler.compileSlice(expression)
	case *compilerast.CallExpr:
		return compiler.compileCall(expression)
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
