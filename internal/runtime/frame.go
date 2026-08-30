package runtime

import "github.com/spachava753/bullsnake/internal/compiler/lexer"

type frame struct {
	code         *preparedCode
	instruction  int
	stack        []Value
	fastLocals   []Value
	deref        []*cellValue
	locals       *Namespace
	globals      *Namespace
	builtins     *Namespace
	previous     *frame
	classBuild   *classBuild
	instanceInit *instanceInit
}

type threadState struct {
	current *frame
}

func (frame *frame) push(value Value) bool {
	if len(frame.stack) >= frame.code.stackSize {
		return false
	}
	frame.stack = append(frame.stack, value)
	return true
}

func (frame *frame) pop() (Value, bool) {
	if len(frame.stack) == 0 {
		return nil, false
	}
	index := len(frame.stack) - 1
	value := frame.stack[index]
	frame.stack[index] = nil
	frame.stack = frame.stack[:index]
	return value, true
}

func (frame *frame) lookupName(name string) (Value, bool) {
	if value, ok := frame.locals.get(name); ok {
		return value, true
	}
	if frame.globals != frame.locals {
		if value, ok := frame.globals.get(name); ok {
			return value, true
		}
	}
	return frame.builtins.get(name)
}

func (frame *frame) position(index int) lexer.Span {
	position, _ := frame.code.code.Position(index)
	return position
}

func (frame *frame) failure(index int, message string) error {
	return &BytecodeError{
		Filename:    frame.code.code.Filename(),
		Instruction: index,
		Span:        frame.position(index),
		Message:     message,
	}
}
