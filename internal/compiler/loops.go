package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type loopContext struct {
	continueLabel *jumpLabel
	breakLabel    *jumpLabel
}

// compileWhileStatement keeps normal condition failure separate from break so
// only the normal path enters the optional else suite.
func (compiler *compilerState) compileWhileStatement(statement *compilerast.WhileStmt) error {
	start := compiler.newLabel()
	end := compiler.newLabel()
	normalExit := end
	if len(statement.Else) != 0 {
		normalExit = compiler.newLabel()
	}
	if err := compiler.markLabel(start, statement.Span()); err != nil {
		return err
	}
	if err := compiler.compileExpr(statement.Condition); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, normalExit, statement.Condition.Span()); err != nil {
		return err
	}

	compiler.loops = append(compiler.loops, loopContext{
		continueLabel: start,
		breakLabel:    end,
	})
	if err := compiler.compileStatements(statement.Body); err != nil {
		return err
	}
	compiler.loops = compiler.loops[:len(compiler.loops)-1]
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, start, statement.Span()); err != nil {
			return err
		}
	}

	if len(statement.Else) == 0 {
		return compiler.markLabel(end, statement.Span())
	}
	if err := compiler.markLabel(normalExit, statement.Span()); err != nil {
		return err
	}
	if err := compiler.compileStatements(statement.Else); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}
