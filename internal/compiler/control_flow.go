package compiler

import (
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type jumpLabel struct {
	position   uint32
	depth      int
	depthSet   bool
	marked     bool
	references []int
}

func (compiler *compilerState) newLabel() *jumpLabel {
	label := &jumpLabel{}
	compiler.labels = append(compiler.labels, label)
	return label
}

// emitJump records the taken-edge depth separately from the instruction's
// fallthrough effect and leaves unconditional paths unreachable.
func (compiler *compilerState) emitJump(opcode bytecode.Opcode, label *jumpLabel, span lexer.Span) error {
	if !compiler.reachable {
		return compiler.error(span, "cannot emit %s on an unreachable path", opcode)
	}
	if label == nil {
		return compiler.error(span, "jump has no target label")
	}
	targetDepth := compiler.stackDepth
	fallthroughDepth := compiler.stackDepth
	switch opcode {
	case bytecode.Jump:
	case bytecode.PopJumpIfFalse:
		targetDepth--
		fallthroughDepth--
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		fallthroughDepth--
	case bytecode.ForIter:
		targetDepth--
		fallthroughDepth++
	default:
		return compiler.error(span, "instruction %s is not a supported jump", opcode)
	}
	if targetDepth < 0 || fallthroughDepth < 0 {
		return compiler.error(span, "instruction %s underflows the operand stack", opcode)
	}
	if err := compiler.mergeLabelDepth(label, targetDepth, span); err != nil {
		return err
	}
	instruction := compiler.appendInstruction(opcode, 0, span)
	if label.marked {
		compiler.instructions[instruction].Operand = label.position
	} else {
		label.references = append(label.references, instruction)
	}
	if opcode == bytecode.Jump {
		compiler.reachable = false
		return nil
	}
	compiler.stackDepth = fallthroughDepth
	if compiler.stackDepth > compiler.maxStack {
		compiler.maxStack = compiler.stackDepth
	}
	return nil
}

func (compiler *compilerState) mergeLabelDepth(label *jumpLabel, depth int, span lexer.Span) error {
	if !label.depthSet {
		label.depth = depth
		label.depthSet = true
		return nil
	}
	if label.depth != depth {
		return compiler.error(span, "jump target merges stack depths %d and %d", label.depth, depth)
	}
	return nil
}

// markLabel resolves pending jumps and starts the label's merged fallthrough
// path. A label with no reachable incoming edge remains a marked dead join.
func (compiler *compilerState) markLabel(label *jumpLabel, span lexer.Span) error {
	if label == nil {
		return compiler.error(span, "cannot mark a nil jump label")
	}
	if label.marked {
		return compiler.error(span, "jump label is marked more than once")
	}
	if compiler.reachable {
		if err := compiler.mergeLabelDepth(label, compiler.stackDepth, span); err != nil {
			return err
		}
	}
	label.position = uint32(len(compiler.instructions))
	label.marked = true
	for _, instruction := range label.references {
		compiler.instructions[instruction].Operand = label.position
	}
	if !label.depthSet {
		compiler.reachable = false
		return nil
	}
	compiler.reachable = true
	compiler.stackDepth = label.depth
	return nil
}
