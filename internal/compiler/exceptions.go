package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileRaiseStatement evaluates an optional exception and cause before
// terminating the current path with the matching raise operand count.
func (compiler *compilerState) compileRaiseStatement(statement *compilerast.RaiseStmt) error {
	arguments := uint32(0)
	if statement.Exception != nil {
		if err := compiler.compileExpr(statement.Exception); err != nil {
			return err
		}
		arguments = 1
	}
	if statement.Cause != nil {
		if statement.Exception == nil {
			return compiler.error(statement.Span(), "raise cause has no exception")
		}
		if err := compiler.compileExpr(statement.Cause); err != nil {
			return err
		}
		arguments = 2
	}
	return compiler.emitTerminator(bytecode.RaiseVarargs, arguments, statement.Span())
}

// compileAssertStatement keeps message evaluation on the failing edge and
// rejoins the true edge after a terminating AssertionError raise.
func (compiler *compilerState) compileAssertStatement(statement *compilerast.AssertStmt) error {
	end := compiler.newLabel()
	if err := compiler.compileExpr(statement.Condition); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, end, statement.Condition.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadAssertionError, 0, statement.Span()); err != nil {
		return err
	}
	if statement.Message != nil {
		if err := compiler.compileExpr(statement.Message); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Call, 1, statement.Span()); err != nil {
			return err
		}
	}
	if err := compiler.emitTerminator(bytecode.RaiseVarargs, 1, statement.Span()); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}
