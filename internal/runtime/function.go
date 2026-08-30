package runtime

import (
	"fmt"
	"strings"
)

type functionValue struct {
	code     *preparedCode
	globals  *Namespace
	defaults []Value
}

func (*functionValue) TypeName() string { return "function" }
func (function *functionValue) Repr() string {
	return "<function " + function.code.code.QualifiedName() + ">"
}
func (*functionValue) isValue() {}

// executeCall removes one function and its positional arguments from the
// caller, binds a fresh fast-local array, and returns a child-frame transition.
func executeCall(
	caller *frame,
	instruction int,
	argumentCount int,
) (instructionOutcome, error) {
	required := argumentCount + 1
	if argumentCount < 0 || required > len(caller.stack) {
		return instructionOutcome{}, caller.failure(instruction, "operand stack underflow")
	}
	base := len(caller.stack) - required
	callable := caller.stack[base]
	function, ok := callable.(*functionValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+callable.TypeName()+"' object is not callable",
			),
		}, nil
	}
	if exception := checkPositionalArity(function, argumentCount); exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}

	locals := make([]Value, len(function.code.locals))
	copy(locals, caller.stack[base+1:])
	defaultStart := function.code.code.PositionalCount() - len(function.defaults)
	for local := argumentCount; local < function.code.code.PositionalCount(); local++ {
		locals[local] = function.defaults[local-defaultStart]
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	child := &frame{
		code:       function.code,
		stack:      make([]Value, 0, function.code.stackSize),
		fastLocals: locals,
		locals:     newNamespace(),
		globals:    function.globals,
		builtins:   caller.builtins,
		previous:   caller,
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

// checkPositionalArity reports CPython-style too-many and missing-argument
// failures for the current required-positional-only call binder.
func checkPositionalArity(function *functionValue, actual int) *Exception {
	expected := function.code.code.PositionalCount()
	required := expected - len(function.defaults)
	name := function.code.code.QualifiedName()
	if actual >= required && actual <= expected {
		return nil
	}
	if actual > expected {
		signature := fmt.Sprintf("%d", expected)
		plural := expected != 1
		if len(function.defaults) != 0 {
			signature = fmt.Sprintf("from %d to %d", required, expected)
			plural = true
		}
		argument := "argument"
		if plural {
			argument = "arguments"
		}
		given := "were"
		if actual == 1 {
			given = "was"
		}
		return newException(
			"TypeError",
			fmt.Sprintf(
				"%s() takes %s positional %s but %d %s given",
				name,
				signature,
				argument,
				actual,
				given,
			),
		)
	}

	missing := function.code.locals[actual:required]
	argument := "arguments"
	if len(missing) == 1 {
		argument = "argument"
	}
	return newException(
		"TypeError",
		fmt.Sprintf(
			"%s() missing %d required positional %s: %s",
			name,
			len(missing),
			argument,
			formatMissingArguments(missing),
		),
	)
}

func formatMissingArguments(names []string) string {
	quoted := make([]string, len(names))
	for index, name := range names {
		quoted[index] = "'" + name + "'"
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	case 2:
		return quoted[0] + " and " + quoted[1]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") +
			", and " + quoted[len(quoted)-1]
	}
}
