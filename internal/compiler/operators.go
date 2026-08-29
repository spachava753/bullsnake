package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileUnary(expression *compilerast.UnaryExpr) error {
	operand, ok := unaryOperand(expression.Op)
	if !ok {
		return compiler.error(expression.Span(), "unknown unary operator %s", expression.Op)
	}
	if err := compiler.compileExpr(expression.Operand); err != nil {
		return err
	}
	return compiler.emit(bytecode.UnaryOp, operand, expression.Span())
}

func (compiler *compilerState) compileBinary(expression *compilerast.BinaryExpr) error {
	operand, ok := binaryOperand(expression.Op)
	if !ok {
		return compiler.error(expression.Span(), "unknown binary operator %s", expression.Op)
	}
	if err := compiler.compileExpr(expression.Left); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression.Right); err != nil {
		return err
	}
	return compiler.emit(bytecode.BinaryOp, operand, expression.Span())
}

func unaryOperand(operator compilerast.UnaryOperator) (uint32, bool) {
	switch operator {
	case compilerast.Positive:
		return bytecode.UnaryPositive, true
	case compilerast.Negative:
		return bytecode.UnaryNegative, true
	case compilerast.Invert:
		return bytecode.UnaryInvert, true
	case compilerast.Not:
		return bytecode.UnaryNot, true
	default:
		return 0, false
	}
}

// binaryOperand keeps AST and bytecode enum ordering independent.
func binaryOperand(operator compilerast.BinaryOperator) (uint32, bool) {
	switch operator {
	case compilerast.Add:
		return bytecode.BinaryAdd, true
	case compilerast.Subtract:
		return bytecode.BinarySubtract, true
	case compilerast.Multiply:
		return bytecode.BinaryMultiply, true
	case compilerast.MatrixMultiply:
		return bytecode.BinaryMatrixMultiply, true
	case compilerast.Divide:
		return bytecode.BinaryDivide, true
	case compilerast.FloorDivide:
		return bytecode.BinaryFloorDivide, true
	case compilerast.Modulo:
		return bytecode.BinaryModulo, true
	case compilerast.Power:
		return bytecode.BinaryPower, true
	case compilerast.LeftShift:
		return bytecode.BinaryLeftShift, true
	case compilerast.RightShift:
		return bytecode.BinaryRightShift, true
	case compilerast.BitOr:
		return bytecode.BinaryOr, true
	case compilerast.BitXor:
		return bytecode.BinaryXor, true
	case compilerast.BitAnd:
		return bytecode.BinaryAnd, true
	default:
		return 0, false
	}
}
