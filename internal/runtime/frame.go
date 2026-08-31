package runtime

import "github.com/spachava753/bullsnake/internal/compiler/lexer"

type handledException struct {
	exception *Exception
	start     int
	end       int
}

type importRequest struct {
	instructionName string
	requestedName   string
	names           []string
	next            int
	returnName      string
	fromNames       []string
	fromIndex       int
	fallbackName    string
}

type moduleImport struct {
	module  *Module
	request *importRequest
}

type delegationState struct {
	sendInstruction  int
	yieldInstruction int
	target           int
	closeException   *Exception
}

type frame struct {
	runtime             *Runtime
	code                *preparedCode
	instruction         int
	stack               []Value
	fastLocals          []Value
	deref               []*cellValue
	locals              *Namespace
	globals             *Namespace
	builtins            *Namespace
	previous            *frame
	classBuild          *classBuild
	instanceInit        *instanceInit
	classAnnotations    *classAnnotationLoad
	functionAnnotations *functionAnnotationLoad
	typeVar             *typeVarLoad
	typeAlias           *typeAliasLoad
	moduleImport        *moduleImport
	pendingImport       *importRequest
	generator           *generatorValue
	delegation          *delegationState
	handledExceptions   []handledException
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

func (frame *frame) pruneHandledExceptions(instruction int) {
	for len(frame.handledExceptions) != 0 {
		last := len(frame.handledExceptions) - 1
		handled := frame.handledExceptions[last]
		if instruction >= handled.start && instruction < handled.end {
			return
		}
		frame.handledExceptions[last].exception = nil
		frame.handledExceptions = frame.handledExceptions[:last]
	}
}

func activeHandledException(current *frame, instruction int) *Exception {
	for current != nil {
		current.pruneHandledExceptions(instruction)
		if count := len(current.handledExceptions); count != 0 {
			return current.handledExceptions[count-1].exception
		}
		current = current.previous
		if current != nil {
			instruction = current.instruction - 1
		}
	}
	return nil
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

func (frame *frame) discardImportedModule() {
	if frame.moduleImport == nil || frame.runtime == nil {
		return
	}
	module := frame.moduleImport.module
	if frame.runtime.modules[module.name] == module {
		delete(frame.runtime.modules, module.name)
	}
	frame.moduleImport = nil
}

func (frame *frame) failure(index int, message string) error {
	return &BytecodeError{
		Filename:    frame.code.code.Filename(),
		Instruction: index,
		Span:        frame.position(index),
		Message:     message,
	}
}
