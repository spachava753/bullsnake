package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileStatements(statements []compilerast.Stmt) error {
	for _, statement := range statements {
		if err := compiler.compileStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

// compileStatement emits one supported statement while preserving Python's
// value-before-target assignment order.
func (compiler *compilerState) compileStatement(statement compilerast.Stmt) error {
	switch statement := statement.(type) {
	case *compilerast.PassStmt:
		return compiler.emit(bytecode.Nop, 0, statement.Span())
	case *compilerast.ExprStmt:
		if err := compiler.compileExpr(statement.Value); err != nil {
			return err
		}
		return compiler.emit(bytecode.PopTop, 0, statement.Span())
	case *compilerast.AssignStmt:
		if err := compiler.compileExpr(statement.Value); err != nil {
			return err
		}
		for index, target := range statement.Targets {
			if index != len(statement.Targets)-1 {
				if err := compiler.emit(bytecode.Copy, 1, statement.Span()); err != nil {
					return err
				}
			}
			if err := compiler.compileStore(target); err != nil {
				return err
			}
		}
		return nil
	default:
		return compiler.unsupported(statement)
	}
}

func (compiler *compilerState) compileStore(expression compilerast.Expr) error {
	name, ok := expression.(*compilerast.Name)
	if !ok {
		return compiler.unsupported(expression)
	}
	if compiler.scope.Symbols[name.ID] == nil {
		return compiler.error(name.Span(), "resolver has no symbol for %q", name.ID)
	}
	return compiler.emit(bytecode.StoreName, compiler.nameIndex(name.ID), name.Span())
}
