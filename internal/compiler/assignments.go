package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
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

// compileAnnotatedAssignment handles function-local annotations, whose
// annotation expressions are validated by the resolver but never executed.
func (compiler *compilerState) compileAnnotatedAssignment(
	statement *compilerast.AnnAssignStmt,
) error {
	if compiler.scope.Kind != resolver.FunctionScope {
		return compiler.error(statement.Span(), "module and class annotated assignments are not compiled")
	}
	if statement.Value != nil {
		if err := compiler.compileExpr(statement.Value); err != nil {
			return err
		}
		return compiler.compileStore(statement.Target)
	}
	switch target := statement.Target.(type) {
	case *compilerast.Name:
		return nil
	case *compilerast.AttributeExpr:
		return compiler.compileAnnotationAddress(target.Value)
	case *compilerast.SubscriptExpr:
		if err := compiler.compileAnnotationAddress(target.Value); err != nil {
			return err
		}
		return compiler.compileAnnotationSubscript(target.Index)
	default:
		return compiler.error(statement.Target.Span(), "invalid annotated assignment target")
	}
}

func (compiler *compilerState) compileAnnotationAddress(expression compilerast.Expr) error {
	if err := compiler.compileExpr(expression); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, expression.Span())
}

// compileAnnotationSubscript evaluates slice and extended-slice components
// separately because an annotation-only target never performs subscription.
func (compiler *compilerState) compileAnnotationSubscript(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.SliceExpr:
		for _, component := range []compilerast.Expr{
			expression.Lower,
			expression.Upper,
			expression.Step,
		} {
			if component == nil {
				continue
			}
			if err := compiler.compileAnnotationAddress(component); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.TupleExpr:
		for _, element := range expression.Elements {
			if err := compiler.compileAnnotationSubscript(element); err != nil {
				return err
			}
		}
		return nil
	default:
		return compiler.compileAnnotationAddress(expression)
	}
}

// compileStore consumes one assigned value and evaluates any address
// expressions before emitting the target-specific store operation.
func (compiler *compilerState) compileStore(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		if expression.Context != compilerast.Store {
			return compiler.error(expression.Span(), "name target is not a store")
		}
		return compiler.emitNameStore(expression.ID, expression.Span())
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
