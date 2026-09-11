package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// frameValue is the stable Python identity of a retained VM frame.
type frameValue struct {
	frame *frame
}

func (*frameValue) TypeName() string { return "frame" }
func (*frameValue) Repr() string     { return "<frame object>" }
func (*frameValue) isValue()         {}

func (current *frame) pythonFrame() *frameValue {
	if current.pythonValue == nil {
		current.pythonValue = &frameValue{frame: current}
	}
	return current.pythonValue
}

// executeGetFrame resolves depth before walking the Python caller chain, so
// index callbacks do not change which frame counts as depth zero.
func executeGetFrame(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("_getframe", arguments, keywords, 0, 1); exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	depth := Value(integerFromInt64(0))
	if len(arguments) != 0 {
		depth = arguments[0]
	}
	discardCallSegment(caller, base)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, depth)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		depth := result.(*intValue).value.Int64()
		if depth < -2147483648 || depth > 2147483647 {
			return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
		}
		target := caller
		for depth > 0 && target != nil {
			target = target.previous
			depth--
		}
		if target == nil {
			return raiseOutcome(newException("ValueError", "call stack is not deep enough")), nil
		}
		return pushOutcome(current, instruction, target.pythonFrame())
	})
}

// executeFrameAttributeLoad exposes live namespaces and caller identity without
// exposing bytecode storage or enabling frame mutation, tracing, or execution.
func executeFrameAttributeLoad(caller *frame, instruction int, value *frameValue, name string) (instructionOutcome, error) {
	target := value.frame
	switch name {
	case "f_back":
		if target.previous == nil {
			return pushOutcome(caller, instruction, None)
		}
		return pushOutcome(caller, instruction, target.previous.pythonFrame())
	case "f_globals":
		return pushOutcome(caller, instruction, target.globals.asDictionary())
	case "f_builtins":
		return pushOutcome(caller, instruction, target.builtins.asDictionary())
	case "f_locals":
		if target.classBuild != nil && target.classBuild.dictionary != nil {
			return pushOutcome(caller, instruction, target.classBuild.dictionary)
		}
		if target.code.code.Flags()&bytecode.Optimized == 0 {
			return pushOutcome(caller, instruction, target.locals.asDictionary())
		}
		return pushOutcome(caller, instruction, &frameLocalsProxy{frame: target})
	case "f_lineno":
		position := target.position(max(target.instruction-1, 0))
		return pushOutcome(caller, instruction, integerFromInt64(int64(position.Start.Line)))
	default:
		return raiseOutcome(newException("AttributeError", "'frame' object has no attribute '"+name+"'")), nil
	}
}
