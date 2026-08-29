package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileDeleteStatement(statement *compilerast.DeleteStmt) error {
	for _, target := range statement.Targets {
		if err := compiler.compileDeleteTarget(target); err != nil {
			return err
		}
	}
	return nil
}

// compileDeleteTarget evaluates one target address or recursively visits a
// grouped target without unpacking a runtime value.
func (compiler *compilerState) compileDeleteTarget(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		if expression.Context != compilerast.Delete {
			return compiler.error(expression.Span(), "name delete target has the wrong context")
		}
		return compiler.emitNameDelete(expression.ID, expression.Span())
	case *compilerast.AttributeExpr:
		if expression.Context != compilerast.Delete {
			return compiler.error(expression.Span(), "attribute delete target has the wrong context")
		}
		if err := compiler.compileExpr(expression.Value); err != nil {
			return err
		}
		return compiler.emit(bytecode.DeleteAttr, compiler.nameIndex(expression.Name), expression.Span())
	case *compilerast.SubscriptExpr:
		if expression.Context != compilerast.Delete {
			return compiler.error(expression.Span(), "subscript delete target has the wrong context")
		}
		if err := compiler.compileExpr(expression.Value); err != nil {
			return err
		}
		if err := compiler.compileExpr(expression.Index); err != nil {
			return err
		}
		return compiler.emit(bytecode.DeleteSubscript, 0, expression.Span())
	case *compilerast.TupleExpr:
		if expression.Context != compilerast.Delete {
			return compiler.error(expression.Span(), "tuple delete target has the wrong context")
		}
		for _, element := range expression.Elements {
			if err := compiler.compileDeleteTarget(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.ListExpr:
		if expression.Context != compilerast.Delete {
			return compiler.error(expression.Span(), "list delete target has the wrong context")
		}
		for _, element := range expression.Elements {
			if err := compiler.compileDeleteTarget(element); err != nil {
				return err
			}
		}
		return nil
	default:
		return compiler.error(expression.Span(), "invalid delete target")
	}
}
