package runtime

// newBuiltinClass creates an immutable runtime-owned class using ordinary C3
// ancestry. Concrete implementations attach their private instance storage.
func newBuiltinClass(module, name string, base *typeValue) *typeValue {
	class := &typeValue{name: name, qualifiedName: name, module: module, namespace: newNamespace(), immutable: true}
	class.mro = []*typeValue{class}
	if base != nil {
		class.bases = []*typeValue{base}
		class.mro = append(class.mro, base.mro...)
		base.subclasses.entries = append(base.subclasses.entries, makeWeakClass(class))
	} else {
		class.objectBase = true
	}
	return class
}

// nativeInstanceMethod binds through the same descriptor path as Python methods
// and rejects a receiver outside the defining class's ancestry.
func nativeInstanceMethod(class *typeValue, name string, call func(*frame, int, *instanceValue, []Value, *dictValue) (instructionOutcome, error)) Value {
	return &builtinFunctionValue{name: name, method: true, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		arguments = append([]Value(nil), arguments...)
		discardCallSegment(caller, base)
		if len(arguments) == 0 {
			return raiseOutcome(newException("TypeError", "unbound method "+name+"() needs an argument")), nil
		}
		self, ok := arguments[0].(*instanceValue)
		if !ok || !self.class.isSubclassOf(class) {
			return raiseOutcome(newException("TypeError", "descriptor '"+name+"' requires a '"+class.name+"' object")), nil
		}
		return call(caller, instruction, self, arguments[1:], keywords)
	}}
}
