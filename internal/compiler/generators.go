package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileYieldExpression suspends with one value and leaves the value sent by
// the resumer as the expression result. Basic yield-from delegates iteration.
func (compiler *compilerState) compileYieldExpression(expression *compilerast.YieldExpr) error {
	if !expression.From {
		if expression.Value == nil {
			if err := compiler.emit(
				bytecode.LoadConst,
				compiler.constantIndex(bytecode.None()),
				expression.Span(),
			); err != nil {
				return err
			}
		} else if err := compiler.compileExpr(expression.Value); err != nil {
			return err
		}
		return compiler.emit(bytecode.YieldValue, 0, expression.Span())
	}

	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.GetIter, 0, expression.Value.Span()); err != nil {
		return err
	}
	start := compiler.newLabel()
	end := compiler.newLabel()
	if err := compiler.markLabel(start, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.ForIter, end, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.YieldValue, 0, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Jump, start, expression.Span()); err != nil {
		return err
	}
	if err := compiler.markLabel(end, expression.Span()); err != nil {
		return err
	}
	return compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.None()),
		expression.Span(),
	)
}
