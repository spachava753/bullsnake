package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileBoolean emits short-circuit jumps while preserving the selected
// operand as the value of the and/or expression.
func (compiler *compilerState) compileBoolean(expression *compilerast.BooleanExpr) error {
	if len(expression.Values) == 0 {
		return compiler.error(expression.Span(), "boolean expression has no values")
	}
	jump := bytecode.JumpIfFalseOrPop
	if expression.Op == compilerast.Or {
		jump = bytecode.JumpIfTrueOrPop
	} else if expression.Op != compilerast.And {
		return compiler.error(expression.Span(), "unknown boolean operator %s", expression.Op)
	}
	end := compiler.newLabel()
	for _, value := range expression.Values[:len(expression.Values)-1] {
		if err := compiler.compileExpr(value); err != nil {
			return err
		}
		if err := compiler.emitJump(jump, end, value.Span()); err != nil {
			return err
		}
	}
	last := expression.Values[len(expression.Values)-1]
	if err := compiler.compileExpr(last); err != nil {
		return err
	}
	return compiler.markLabel(end, expression.Span())
}

// compileConditional emits separate value-producing branches and merges them
// at an end label with one value on the operand stack.
func (compiler *compilerState) compileConditional(expression *compilerast.ConditionalExpr) error {
	otherwise := compiler.newLabel()
	end := compiler.newLabel()
	if err := compiler.compileExpr(expression.Condition); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, otherwise, expression.Condition.Span()); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression.Then); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Jump, end, expression.Then.Span()); err != nil {
		return err
	}
	if err := compiler.markLabel(otherwise, expression.Else.Span()); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression.Else); err != nil {
		return err
	}
	return compiler.markLabel(end, expression.Span())
}

// compileComparison preserves each middle operand across one comparison and
// removes it on the first false short-circuit edge.
func (compiler *compilerState) compileComparison(expression *compilerast.CompareExpr) error {
	if len(expression.Operators) == 0 || len(expression.Operators) != len(expression.Comparators) {
		return compiler.error(expression.Span(), "comparison operator/operand count mismatch")
	}
	operands := make([]uint32, len(expression.Operators))
	for index, operator := range expression.Operators {
		operand, ok := comparisonOperand(operator)
		if !ok {
			return compiler.error(expression.Span(), "unknown comparison operator %s", operator)
		}
		operands[index] = operand
	}
	if err := compiler.compileExpr(expression.Left); err != nil {
		return err
	}
	if len(operands) == 1 {
		if err := compiler.compileExpr(expression.Comparators[0]); err != nil {
			return err
		}
		return compiler.emit(bytecode.CompareOp, operands[0], expression.Span())
	}

	cleanup := compiler.newLabel()
	end := compiler.newLabel()
	for index := range len(operands) - 1 {
		if err := compiler.compileExpr(expression.Comparators[index]); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Swap, 2, expression.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Copy, 2, expression.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.CompareOp, operands[index], expression.Span()); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.JumpIfFalseOrPop, cleanup, expression.Span()); err != nil {
			return err
		}
	}
	last := len(operands) - 1
	if err := compiler.compileExpr(expression.Comparators[last]); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, operands[last], expression.Span()); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Jump, end, expression.Span()); err != nil {
		return err
	}
	if err := compiler.markLabel(cleanup, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Swap, 2, expression.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, expression.Span()); err != nil {
		return err
	}
	return compiler.markLabel(end, expression.Span())
}
