package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileIfStatement emits one false edge and, when an else suite exists, a
// second edge that skips it after the true suite completes.
func (compiler *compilerState) compileIfStatement(statement *compilerast.IfStmt) error {
	end := compiler.newLabel()
	otherwise := end
	if len(statement.Else) != 0 {
		otherwise = compiler.newLabel()
	}

	if err := compiler.compileExpr(statement.Condition); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, otherwise, statement.Condition.Span()); err != nil {
		return err
	}
	if err := compiler.compileStatements(statement.Body); err != nil {
		return err
	}
	if len(statement.Else) == 0 {
		return compiler.markLabel(end, statement.Span())
	}

	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, end, statement.Span()); err != nil {
			return err
		}
	}
	if err := compiler.markLabel(otherwise, statement.Span()); err != nil {
		return err
	}
	if err := compiler.compileStatements(statement.Else); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}
