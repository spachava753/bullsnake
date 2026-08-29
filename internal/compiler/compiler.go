package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

type compilerState struct {
	filename string
	module   *compilerast.Module
	table    *resolver.Table
	scope    *resolver.Scope

	instructions []bytecode.Instruction
	positions    []lexer.Span
	constants    []bytecode.Constant
	constantIDs  map[bytecode.Constant]uint32
	names        []string
	nameIDs      map[string]uint32
	stackDepth   int
	maxStack     int
	reachable    bool
	labels       []*jumpLabel
}

// emit appends a fallthrough instruction after validating reachability,
// stack addressing, and the resulting operand-stack depth.
func (compiler *compilerState) emit(opcode bytecode.Opcode, operand uint32, span lexer.Span) error {
	if !compiler.reachable {
		return compiler.error(span, "cannot emit %s on an unreachable path", opcode)
	}
	if (opcode == bytecode.Copy || opcode == bytecode.Swap) && (operand == 0 || int(operand) > compiler.stackDepth) {
		return compiler.error(span, "instruction %s %d exceeds stack depth %d", opcode, operand, compiler.stackDepth)
	}
	compiler.appendInstruction(opcode, operand, span)
	compiler.stackDepth += opcode.StackEffect(operand)
	if compiler.stackDepth < 0 {
		return compiler.error(span, "instruction %s underflows the operand stack", opcode)
	}
	if compiler.stackDepth > compiler.maxStack {
		compiler.maxStack = compiler.stackDepth
	}
	return nil
}

func (compiler *compilerState) appendInstruction(opcode bytecode.Opcode, operand uint32, span lexer.Span) int {
	index := len(compiler.instructions)
	compiler.instructions = append(compiler.instructions, bytecode.Instruction{
		Opcode:  opcode,
		Operand: operand,
	})
	compiler.positions = append(compiler.positions, span)
	return index
}

func (compiler *compilerState) constantIndex(constant bytecode.Constant) uint32 {
	if index, ok := compiler.constantIDs[constant]; ok {
		return index
	}
	index := uint32(len(compiler.constants))
	compiler.constantIDs[constant] = index
	compiler.constants = append(compiler.constants, constant)
	return index
}

func (compiler *compilerState) nameIndex(name string) uint32 {
	if index, ok := compiler.nameIDs[name]; ok {
		return index
	}
	index := uint32(len(compiler.names))
	compiler.nameIDs[name] = index
	compiler.names = append(compiler.names, name)
	return index
}

// finish validates control-flow and stack invariants before constructing the
// immutable module code object.
func (compiler *compilerState) finish() (*bytecode.Code, error) {
	for _, label := range compiler.labels {
		if !label.marked {
			return nil, compiler.error(compiler.module.Span(), "unresolved jump label")
		}
	}
	if !compiler.reachable {
		return nil, compiler.error(compiler.module.Span(), "module ends on an unreachable path")
	}
	if compiler.stackDepth != 0 {
		return nil, compiler.error(
			compiler.module.Span(),
			"module leaves %d values on the operand stack",
			compiler.stackDepth,
		)
	}
	firstLine := 1
	if len(compiler.module.Body) != 0 {
		firstLine = compiler.module.Body[0].Span().Start.Line
	}
	return bytecode.NewCode(bytecode.CodeSpec{
		Filename:      compiler.filename,
		Name:          "<module>",
		QualifiedName: "<module>",
		FirstLine:     firstLine,
		StackSize:     compiler.maxStack,
		Instructions:  compiler.instructions,
		Positions:     compiler.positions,
		Constants:     compiler.constants,
		Names:         compiler.names,
	}), nil
}
