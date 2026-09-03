package runtime

import (
	"fmt"
	"strings"
)

type pickleState struct {
	next   uint64
	values map[string]Value
}

func newPickleModule() *Module {
	module := newSystemModule("_pickle", "")
	state := &pickleState{values: make(map[string]Value)}
	for _, name := range []string{"PickleError", "PicklingError", "UnpicklingError"} {
		module.globals.values[name] = exceptionType
	}
	for _, name := range []string{"Pickler", "Unpickler", "PickleBuffer"} {
		module.globals.values[name] = &builtinTypeValue{name: name, matches: func(Value) bool { return false }}
	}
	module.globals.values["dumps"] = nativeKeywordAwareFunctionNamed("_pickle.dumps", 1, 2, state.dumps)
	module.globals.values["loads"] = nativeKeywordAwareFunctionNamed("_pickle.loads", 1, 1, state.loads)
	module.globals.values["dump"] = nativeKeywordAwareFunctionNamed("_pickle.dump", 2, 3, state.dump)
	module.globals.values["load"] = nativeKeywordAwareFunctionNamed("_pickle.load", 1, 1, state.load)
	return module
}

func (state *pickleState) dumps(
	_ *frame,
	arguments []Value,
	_ *dictValue,
) (Value, *Exception, error) {
	state.next++
	payload := fmt.Sprintf("bullsnake-pickle:%d", state.next)
	state.values[payload] = arguments[0]
	return &bytesValue{value: payload}, nil, nil
}

func (state *pickleState) loads(
	_ *frame,
	arguments []Value,
	_ *dictValue,
) (Value, *Exception, error) {
	var payload string
	switch value := arguments[0].(type) {
	case *bytesValue:
		payload = value.value
	case *bytearrayValue:
		payload = value.value
	default:
		return nil, newException("TypeError", "a bytes-like object is required"), nil
	}
	value, found := state.values[payload]
	if !found || !strings.HasPrefix(payload, "bullsnake-pickle:") {
		return nil, newException("ValueError", "invalid pickle payload"), nil
	}
	return value, nil, nil
}

func (state *pickleState) dump(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	payload, exception, err := state.dumps(caller, arguments[:1], keywords)
	if err != nil || exception != nil {
		return nil, exception, err
	}
	write, found := directAttribute(arguments[1], "write")
	if !found {
		return nil, newException("TypeError", "file must have a write attribute"), nil
	}
	_, exception, err = callValueSynchronously(caller, write, []Value{payload})
	return None, exception, err
}

func (state *pickleState) load(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	read, found := directAttribute(arguments[0], "read")
	if !found {
		return nil, newException("TypeError", "file must have a read attribute"), nil
	}
	payload, exception, err := callValueSynchronously(caller, read, nil)
	if err != nil || exception != nil {
		return nil, exception, err
	}
	return state.loads(caller, []Value{payload}, keywords)
}

func newStructModule() *Module {
	module := newSystemModule("_struct", "")
	for _, name := range []string{
		"calcsize", "pack", "pack_into", "unpack", "unpack_from", "iter_unpack", "_clearcache",
	} {
		setNativeFunction(module, name, 0, -1, structUnavailable)
	}
	module.globals.values["Struct"] = &builtinTypeValue{name: "Struct", matches: func(Value) bool { return false }}
	module.globals.values["error"] = exceptionType
	module.globals.values["__doc__"] = &stringValue{value: "Binary packing support"}
	module.globals.values["__all__"] = stringList([]string{
		"calcsize", "pack", "pack_into", "unpack", "unpack_from", "iter_unpack", "Struct", "error",
	})
	return module
}

func structUnavailable(_ *frame, _ []Value) (Value, *Exception, error) {
	return nil, newException("NotImplementedError", "binary struct operation is not implemented"), nil
}
