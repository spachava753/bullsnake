package resolver

import (
	"fmt"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
)

// collectExpr dispatches every AST expression and visits children in CPython's
// symbol-table order while recording name uses and bindings.
func (state *resolver) collectExpr(expression compilerast.Expr) error {
	if expression == nil {
		return nil
	}
	switch expression := expression.(type) {
	case *compilerast.Name:
		if state.current.Flags&UnevaluatedAnnotations != 0 {
			return nil
		}
		switch expression.Context {
		case compilerast.Load:
			state.record(expression.ID, Used, expression.Span())
			if expression.ID == "super" &&
				(state.current.Kind == FunctionScope || state.current.Kind == AnnotationScope ||
					state.current.Kind == TypeParametersScope || state.current.Kind == TypeVariableScope ||
					state.current.Kind == TypeAliasScope) {
				state.record("__class__", Used, expression.Span())
			}
			return nil
		case compilerast.Store:
			_, err := state.bind(expression.ID, Assigned, expression.Span())
			return err
		case compilerast.Delete:
			if expression.ID == "__debug__" {
				return state.syntaxError(expression.Span(), "cannot delete __debug__")
			}
			state.record(expression.ID, Assigned, expression.Span())
			return nil
		default:
			return fmt.Errorf("resolver: unsupported expression context %s", expression.Context)
		}
	case *compilerast.YieldExpr:
		return state.collectYield(expression)
	case *compilerast.AwaitExpr:
		return state.collectAwait(expression)
	case *compilerast.NamedExpr:
		return state.collectNamedExpression(expression)
	case *compilerast.LambdaExpr:
		return state.collectLambda(expression)
	case *compilerast.ConditionalExpr:
		if err := state.collectExpr(expression.Condition); err != nil {
			return err
		}
		if err := state.collectExpr(expression.Then); err != nil {
			return err
		}
		return state.collectExpr(expression.Else)
	case *compilerast.UnaryExpr:
		return state.collectExpr(expression.Operand)
	case *compilerast.BooleanExpr:
		return state.collectExprs(expression.Values)
	case *compilerast.BinaryExpr:
		if err := state.collectExpr(expression.Left); err != nil {
			return err
		}
		return state.collectExpr(expression.Right)
	case *compilerast.CompareExpr:
		if err := state.collectExpr(expression.Left); err != nil {
			return err
		}
		return state.collectExprs(expression.Comparators)
	case *compilerast.ListComprehensionExpr:
		return state.collectComprehension(
			expression, "listcomp", ListComprehension,
			expression.Clauses, expression.Element, nil,
		)
	case *compilerast.SetComprehensionExpr:
		return state.collectComprehension(
			expression, "setcomp", SetComprehension,
			expression.Clauses, expression.Element, nil,
		)
	case *compilerast.DictComprehensionExpr:
		return state.collectComprehension(
			expression, "dictcomp", DictComprehension,
			expression.Clauses, expression.Key, expression.Value,
		)
	case *compilerast.GeneratorExpr:
		return state.collectComprehension(
			expression, "genexpr", GeneratorExpression,
			expression.Clauses, expression.Element, nil,
		)
	case *compilerast.StringConcatExpr:
		return state.collectExprs(expression.Parts)
	case *compilerast.FormattedStringExpr:
		return state.collectExprs(expression.Parts)
	case *compilerast.FormattedValueExpr:
		if err := state.collectExpr(expression.Value); err != nil {
			return err
		}
		return state.collectExprs(expression.Format)
	case *compilerast.TupleExpr:
		return state.collectExprs(expression.Elements)
	case *compilerast.ListExpr:
		return state.collectExprs(expression.Elements)
	case *compilerast.SetExpr:
		return state.collectExprs(expression.Elements)
	case *compilerast.DictExpr:
		if err := state.collectExprs(expression.Keys); err != nil {
			return err
		}
		return state.collectExprs(expression.Values)
	case *compilerast.CallExpr:
		if err := state.collectExpr(expression.Function); err != nil {
			return err
		}
		if err := state.collectExprs(expression.Arguments); err != nil {
			return err
		}
		for _, keyword := range expression.Keywords {
			if err := state.collectExpr(keyword.Value); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.AttributeExpr:
		return state.collectExpr(expression.Value)
	case *compilerast.SubscriptExpr:
		if err := state.collectExpr(expression.Value); err != nil {
			return err
		}
		return state.collectExpr(expression.Index)
	case *compilerast.SliceExpr:
		if err := state.collectExpr(expression.Lower); err != nil {
			return err
		}
		if err := state.collectExpr(expression.Upper); err != nil {
			return err
		}
		return state.collectExpr(expression.Step)
	case *compilerast.StarredExpr:
		return state.collectExpr(expression.Value)
	case *compilerast.NumberLiteral, *compilerast.StringLiteral,
		*compilerast.BooleanLiteral, *compilerast.NoneLiteral, *compilerast.EllipsisLiteral:
		return nil
	default:
		return unsupportedNode(expression)
	}
}

func (state *resolver) collectExprs(expressions []compilerast.Expr) error {
	for _, expression := range expressions {
		if err := state.collectExpr(expression); err != nil {
			return err
		}
	}
	return nil
}
