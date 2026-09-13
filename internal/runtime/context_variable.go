package runtime

var (
	contextVarNativeType    = nativeType("_contextvars", "ContextVar")
	contextTokenNativeType  = nativeType("_contextvars", "Token")
	contextMissingSingleton = &contextMissingValue{}
)

// contextState owns strong variable bindings for one execution context. Tokens
// retain the actual context identity, not a copy of its current bindings.
type contextState struct {
	bindings map[*contextVarValue]Value
}

type contextVarValue struct {
	name         Value
	defaultValue Value
}

func (*contextVarValue) TypeName() string { return "_contextvars.ContextVar" }
func (variable *contextVarValue) Repr() string {
	text := "<ContextVar name=" + variable.name.Repr()
	if variable.defaultValue != nil {
		text += " default=" + variable.defaultValue.Repr()
	}
	return text + ">"
}
func (*contextVarValue) isValue() {}

type contextTokenValue struct {
	context  *contextState
	variable *contextVarValue
	oldValue Value
	used     bool
}

func (*contextTokenValue) TypeName() string { return "_contextvars.Token" }
func (token *contextTokenValue) Repr() string {
	text := "<Token"
	if token.used {
		text += " used"
	}
	return text + " var=" + token.variable.Repr() + ">"
}
func (*contextTokenValue) isValue() {}

type contextMissingValue struct{}

func (*contextMissingValue) TypeName() string { return "Token.MISSING" }
func (*contextMissingValue) Repr() string     { return "<Token.MISSING>" }
func (*contextMissingValue) isValue()         {}

func initializeContextVars(_ *Runtime, module *Module) (*Exception, error) {
	module.globals.store("ContextVar", contextVarNativeType)
	module.globals.store("Token", contextTokenNativeType)
	return nil, nil
}

func (runtime *Runtime) executionContext() *contextState {
	if runtime.context == nil {
		runtime.context = &contextState{bindings: make(map[*contextVarValue]Value)}
	}
	return runtime.context
}

// executeContextVarConstructor retains a string name and optional keyword-only
// default without installing a binding in the runtime's current context.
func executeContextVarConstructor(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	if len(arguments) != 1 {
		return raiseOutcome(newException("TypeError", "ContextVar() requires one positional name")), nil
	}
	if _, ok := stringStorage(arguments[0]); !ok {
		return raiseOutcome(newException("TypeError", "context variable name must be a str")), nil
	}
	variable := &contextVarValue{name: arguments[0]}
	if keywords != nil {
		for _, entry := range keywords.entries {
			if entry.key.(*stringValue).value != "default" {
				return raiseOutcome(newException("TypeError", "ContextVar() got an unexpected keyword argument '"+entry.key.(*stringValue).value+"'")), nil
			}
			variable.defaultValue = entry.value
		}
	}
	return pushOutcome(caller, instruction, variable)
}

func executeContextVarGet(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("get", arguments, keywords, 0, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	variable := self.(*contextVarValue)
	if value, found := caller.runtime.executionContext().bindings[variable]; found {
		return pushOutcome(caller, instruction, value)
	}
	if len(arguments) != 0 {
		return pushOutcome(caller, instruction, arguments[0])
	}
	if variable.defaultValue != nil {
		return pushOutcome(caller, instruction, variable.defaultValue)
	}
	exception := newException("LookupError", "")
	exception.setArguments([]Value{variable})
	return raiseOutcome(exception), nil
}

func executeContextVarSet(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("set", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	variable := self.(*contextVarValue)
	context := caller.runtime.executionContext()
	token := &contextTokenValue{context: context, variable: variable, oldValue: context.bindings[variable]}
	context.bindings[variable] = arguments[0]
	return pushOutcome(caller, instruction, token)
}

func executeContextVarReset(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("reset", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	token, ok := arguments[0].(*contextTokenValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "expected an instance of Token, got "+arguments[0].Repr())), nil
	}
	if exception := resetContextToken(caller.runtime, self.(*contextVarValue), token); exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, None)
}

// resetContextToken validates before consuming the token. Resets restore its
// captured old binding even when tokens are used out of creation order.
func resetContextToken(runtime *Runtime, variable *contextVarValue, token *contextTokenValue) *Exception {
	if token.used {
		return newException("RuntimeError", token.Repr()+" has already been used once")
	}
	if token.variable != variable {
		return newException("ValueError", token.Repr()+" was created by a different ContextVar")
	}
	if token.context != runtime.executionContext() {
		return newException("ValueError", token.Repr()+" was created in a different Context")
	}
	token.used = true
	if token.oldValue == nil {
		delete(token.context.bindings, variable)
	} else {
		token.context.bindings[variable] = token.oldValue
	}
	return nil
}
