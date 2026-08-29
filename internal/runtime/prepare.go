package runtime

import (
	"fmt"
	"math"
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type preparedCode struct {
	code         *bytecode.Code
	instructions []bytecode.Instruction
	constants    []Value
	names        []string
	stackSize    int
}

// prepareCode copies runtime-facing tables, materializes constants, and rejects
// every invalid or unsupported instruction before execution can begin.
func prepareCode(code *bytecode.Code) (*preparedCode, error) {
	if code == nil {
		return nil, &BytecodeError{Instruction: -1, Message: "nil code object"}
	}
	prepared := &preparedCode{
		code:         code,
		instructions: code.Instructions(),
		names:        code.Names(),
		stackSize:    code.StackSize(),
	}
	if prepared.stackSize < 0 {
		return nil, prepared.failure(-1, "negative operand stack size %d", prepared.stackSize)
	}

	constants := code.Constants()
	prepared.constants = make([]Value, len(constants))
	for index, constant := range constants {
		value, err := materializeConstant(constant)
		if err != nil {
			return nil, prepared.failure(-1, "constant %d: %s", index, err)
		}
		prepared.constants[index] = value
	}
	if err := prepared.validateInstructions(); err != nil {
		return nil, err
	}
	return prepared, nil
}

// materializeConstant converts every compiler literal descriptor into its
// runtime object while rejecting malformed strings and integer descriptors.
func materializeConstant(constant bytecode.Constant) (Value, error) {
	switch constant.Kind {
	case bytecode.NoneConstant:
		return None, nil
	case bytecode.BoolConstant:
		if constant.Bool {
			return trueSingleton, nil
		}
		return falseSingleton, nil
	case bytecode.EllipsisConstant:
		return ellipsisSingleton, nil
	case bytecode.IntegerConstant:
		value, ok := new(big.Int).SetString(constant.Text, 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer %q", constant.Text)
		}
		return &intValue{value: *value}, nil
	case bytecode.FloatConstant:
		return &floatValue{value: math.Float64frombits(constant.Bits)}, nil
	case bytecode.ImaginaryConstant:
		return &complexValue{imaginary: math.Float64frombits(constant.Bits)}, nil
	case bytecode.StringConstant:
		if !validStringEncoding(constant.Text) {
			return nil, fmt.Errorf("invalid string constant encoding")
		}
		return &stringValue{value: constant.Text}, nil
	case bytecode.BytesConstant:
		return &bytesValue{value: constant.Text}, nil
	default:
		return nil, fmt.Errorf("unsupported constant %s", constant)
	}
}

// validateInstructions checks the supported linear control flow and proves that
// every reachable instruction respects the code object's declared stack size.
func (code *preparedCode) validateInstructions() error {
	depth := 0
	returned := false
	for index, instruction := range code.instructions {
		if returned {
			return code.failure(index, "instruction follows RETURN_VALUE")
		}
		if err := code.validateOperand(index, instruction); err != nil {
			return err
		}
		pops, pushes := instructionStackUse(instruction)
		if depth < pops {
			return code.failure(index, "operand stack underflow")
		}
		depth += pushes - pops
		if depth > code.stackSize {
			return code.failure(
				index,
				"operand stack exceeds declared size %d",
				code.stackSize,
			)
		}
		if instruction.Opcode == bytecode.ReturnValue {
			if depth != 0 {
				return code.failure(index, "RETURN_VALUE leaves %d values on the operand stack", depth)
			}
			returned = true
		}
	}
	if !returned {
		return code.failure(len(code.instructions), "code falls through without RETURN_VALUE")
	}
	return nil
}

// validateOperand checks table bounds and limits opcode variants to the
// behavior implemented by the current runtime slice.
func (code *preparedCode) validateOperand(index int, instruction bytecode.Instruction) error {
	switch instruction.Opcode {
	case bytecode.Nop, bytecode.PopTop, bytecode.ReturnValue:
		return nil
	case bytecode.LoadConst:
		if uint64(instruction.Operand) >= uint64(len(code.constants)) {
			return code.failure(index, "constant index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.LoadName, bytecode.StoreName:
		if uint64(instruction.Operand) >= uint64(len(code.names)) {
			return code.failure(index, "name index %d out of range", instruction.Operand)
		}
		return nil
	case bytecode.UnaryOp:
		if instruction.Operand > bytecode.UnaryNot {
			return code.failure(
				index,
				"unsupported UNARY_OP operand %d",
				instruction.Operand,
			)
		}
		return nil
	case bytecode.BinaryOp:
		switch instruction.Operand {
		case bytecode.BinaryAdd,
			bytecode.BinarySubtract,
			bytecode.BinaryMultiply,
			bytecode.BinaryOr,
			bytecode.BinaryXor,
			bytecode.BinaryAnd:
			return nil
		default:
			return code.failure(
				index,
				"unsupported BINARY_OP operand %d",
				instruction.Operand,
			)
		}
	default:
		return code.failure(index, "unsupported opcode %s", instruction.Opcode)
	}
}

func instructionStackUse(instruction bytecode.Instruction) (pops, pushes int) {
	switch instruction.Opcode {
	case bytecode.LoadConst, bytecode.LoadName:
		return 0, 1
	case bytecode.StoreName, bytecode.PopTop, bytecode.ReturnValue:
		return 1, 0
	case bytecode.UnaryOp:
		return 1, 1
	case bytecode.BinaryOp:
		return 2, 1
	default:
		return 0, 0
	}
}

func (code *preparedCode) failure(index int, format string, arguments ...any) error {
	var span lexer.Span
	if position, ok := code.code.Position(index); ok {
		span = position
	}
	return &BytecodeError{
		Filename:    code.code.Filename(),
		Instruction: index,
		Span:        span,
		Message:     fmt.Sprintf(format, arguments...),
	}
}
