package runtime

type attributeCallKind uint8

const (
	attributeGet attributeCallKind = iota
	attributeSet
	attributeDelete
)

type attributeCall struct {
	kind        attributeCallKind
	instruction int
	returnNone  bool
}

// executeDynamicAttributeLoad applies the runtime's existing attribute rules to
// a name supplied by Python rather than stored in a bytecode name table.
func executeDynamicAttributeLoad(
	frame *frame,
	instruction int,
	owner Value,
	name string,
) (instructionOutcome, error) {
	if name == "__class__" {
		if _, instance := owner.(*instanceValue); !instance {
			class, exception := typeOf(owner)
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(frame, instruction, class)
		}
	}
	if method, found := frame.runtime.boundNativeObjectMethod(owner, name); found {
		return pushOutcome(frame, instruction, method)
	}
	if method, found := frame.runtime.boundNativeCollectionMethod(owner, name); found {
		return pushOutcome(frame, instruction, method)
	}
	if outcome, found, err := executeDescriptorSubclassAttribute(frame, instruction, owner, name); found || err != nil {
		return outcome, err
	}
	switch owner := owner.(type) {
	case *classWeakReference:
		if name == "__callback__" {
			return pushOutcome(frame, instruction, None)
		}
		return raiseOutcome(newException("AttributeError", "weak reference has no attribute '"+name+"'")), nil
	case *boundMethodValue:
		if name == "__func__" {
			return pushOutcome(frame, instruction, owner.callable)
		}
		if name == "__self__" {
			return pushOutcome(frame, instruction, owner.self)
		}
		return executeDynamicAttributeLoad(frame, instruction, owner.callable, name)
	case *hostTextStream:
		return executeHostStreamAttributeLoad(frame, instruction, owner, name)
	case *functionValue:
		return executeFunctionAttributeLoad(frame, instruction, owner, name)
	case *classMethodValue:
		return executeMethodDescriptorAttributeLoad(
			frame,
			instruction,
			owner,
			owner.callable,
			name,
		)
	case *staticMethodValue:
		return executeMethodDescriptorAttributeLoad(
			frame,
			instruction,
			owner,
			owner.callable,
			name,
		)
	case *propertyValue:
		return executePropertyAttributeLoad(frame, instruction, owner, name)
	case *templateValue:
		return executeTemplateAttributeLoad(frame, instruction, owner, name)
	case *interpolationValue:
		return executeInterpolationAttributeLoad(frame, instruction, owner, name)
	case *cmpKeyValue:
		return executeCmpKeyAttributeLoad(frame, instruction, owner, name)
	case *stringValue:
		return executeStringAttributeLoad(frame, instruction, owner, name)
	case *codeValue:
		return executeCodeAttributeLoad(frame, instruction, owner, name)
	case *frameValue:
		return executeFrameAttributeLoad(frame, instruction, owner, name)
	case *frameLocalsProxy:
		return executeFrameLocalsAttributeLoad(frame, instruction, owner, name)
	case *mappingProxyValue:
		return executeMappingProxyAttributeLoad(frame, instruction, owner, name)
	case *dictValue:
		return executeDictionaryAttributeLoad(frame, instruction, owner, name)
	case *listValue:
		return executeListAttributeLoad(frame, instruction, owner, name)
	case *setValue:
		return executeSetAttributeLoad(frame, instruction, owner, name)
	case *frozenSetValue:
		return executeFrozenSetAttributeLoad(frame, instruction, owner, name)
	case *rangeValue:
		return executeRangeAttributeLoad(frame, instruction, owner, name)
	case *superValue:
		return executeSuperAttributeLoad(frame, instruction, owner, name)
	case *Exception:
		return executeExceptionAttributeLoad(frame, instruction, owner, name)
	case *Module:
		return executeModuleAttributeLoad(frame, instruction, owner, name)
	case *nativeTypeValue:
		return executeNativeTypeAttributeLoad(frame, instruction, owner, name)
	case *exceptionTypeValue:
		return executeExceptionTypeAttributeLoad(frame, instruction, owner, name)
	case *typeValue:
		return executeClassAttributeLoad(frame, instruction, owner, name)
	case *instanceValue:
		return executeInstanceAttributeLoad(frame, instruction, owner, name)
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
		)), nil
	}
}

// executeFunctionAttributeLoad gives fixed function metadata precedence over
// arbitrary attributes assigned through STORE_ATTR or setattr.
func executeFunctionAttributeLoad(
	frame *frame,
	instruction int,
	owner *functionValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__code__":
		return pushOutcome(frame, instruction, owner.code.pythonCode())
	case "__type_params__":
		return pushOutcome(frame, instruction, owner.typeParams)
	case "__annotate__":
		if owner.annotate == nil {
			return pushOutcome(frame, instruction, None)
		}
		return pushOutcome(frame, instruction, owner.annotate)
	case "__annotations__":
		return executeFunctionAnnotationsLoad(frame, instruction, owner)
	default:
		if owner.attributes != nil {
			if value, found := owner.attributes.get(name); found {
				return pushOutcome(frame, instruction, value)
			}
		}
		return raiseOutcome(newException(
			"AttributeError",
			"'function' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeExceptionAttributeLoad(
	frame *frame,
	instruction int,
	owner *Exception,
	name string,
) (instructionOutcome, error) {
	if name == "with_traceback" {
		return pushOutcome(
			frame,
			instruction,
			&exceptionWithTracebackMethod{exception: owner},
		)
	}
	value, found := owner.attribute(name)
	if !found {
		return raiseOutcome(newException(
			"AttributeError",
			"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, value)
}

func executeModuleAttributeLoad(
	frame *frame,
	instruction int,
	owner *Module,
	name string,
) (instructionOutcome, error) {
	if name == "__dict__" {
		return pushOutcome(frame, instruction, owner.globals.asDictionary())
	}
	value, found := owner.globals.get(name)
	if !found {
		return raiseOutcome(newException(
			"AttributeError",
			"module '"+owner.name+"' has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, value)
}

func executeClassAttributeLoad(
	frame *frame,
	instruction int,
	owner *typeValue,
	name string,
) (instructionOutcome, error) {
	if name == "__annotations__" {
		return executeClassAnnotationsLoad(frame, instruction, owner)
	}
	if name == "__annotate__" {
		value, found := owner.namespace.get("__annotate__")
		if !found {
			value, found = owner.namespace.get("__annotate_func__")
		}
		if !found {
			value = None
		}
		return pushOutcome(frame, instruction, value)
	}
	return executeTypeAttributeLoad(frame, instruction, owner, name)
}

// executeTypeAttributeLoad serves computed class metadata before applying
// ordinary MRO lookup and descriptor binding.
func executeTypeAttributeLoad(
	frame *frame,
	instruction int,
	owner *typeValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__dict__":
		return pushOutcome(frame, instruction, owner.namespaceProxy())
	case "__abstractmethods__":
		if value, found := owner.namespace.get(name); found {
			return pushOutcome(frame, instruction, value)
		}
		return raiseOutcome(newException("AttributeError", name)), nil
	case "__name__":
		return pushOutcome(frame, instruction, &stringValue{value: owner.name})
	case "__qualname__":
		return pushOutcome(frame, instruction, &stringValue{value: owner.qualifiedName})
	case "__module__":
		if value, found := owner.namespace.get("__module__"); found {
			return pushOutcome(frame, instruction, value)
		}
		return pushOutcome(frame, instruction, &stringValue{value: owner.module})
	case "__bases__":
		if owner.nativeBase != nil {
			return pushOutcome(frame, instruction, &tupleValue{
				elements: []Value{owner.nativeBase},
			})
		}
		if owner.objectBase {
			return pushOutcome(
				frame,
				instruction,
				&tupleValue{elements: []Value{objectNativeType}},
			)
		}
		return pushOutcome(frame, instruction, typeTuple(owner.bases))
	case "__base__":
		if len(owner.bases) == 0 {
			if owner.nativeBase != nil {
				return pushOutcome(frame, instruction, owner.nativeBase)
			}
			if owner.objectBase {
				return pushOutcome(frame, instruction, objectNativeType)
			}
			return pushOutcome(frame, instruction, None)
		}
		return pushOutcome(frame, instruction, owner.bases[0])
	case "__mro__":
		result := typeTuple(owner.mro)
		if nativeBase := owner.nativeClassBase(); nativeBase != nil {
			result.elements = append(result.elements, nativeBase, objectNativeType)
		} else if owner.exceptionBase == nil {
			result.elements = append(result.elements, objectNativeType)
		}
		return pushOutcome(frame, instruction, result)
	}
	value, found := owner.lookup(name)
	if !found && name == "__hash__" {
		value, found = frame.runtime.nativeClassAttribute(objectNativeType, name)
	}
	if !found && owner.isSubclassOfNative(typeNativeType) {
		value, found = nativeMetaclassMethod(name)
	}
	if !found && owner.metaclass != nil {
		if method, exists := owner.metaclass.lookup(name); exists {
			if function, ok := method.(*functionValue); ok {
				return pushOutcome(frame, instruction, &boundMethodValue{callable: function, self: owner})
			}
			if bound, ok := bindMethodDescriptor(method, owner.metaclass); ok {
				return pushOutcome(frame, instruction, bound)
			}
			return pushOutcome(frame, instruction, method)
		}
	}
	if !found {
		if name == "__subclasshook__" {
			return pushOutcome(frame, instruction, defaultSubclassHook())
		}
		if name == "__subclasses__" || name == "__subclasscheck__" || name == "__instancecheck__" {
			method, _ := nativeMetaclassMethod(name)
			return pushOutcome(frame, instruction, &boundMethodValue{callable: method, self: owner})
		}
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"type object '"+owner.name+"' has no attribute '"+name+"'",
			),
		}, nil
	}
	if bound, descriptor := bindMethodDescriptor(value, owner); descriptor {
		return pushOutcome(frame, instruction, bound)
	}
	if descriptor, ok := value.(*instanceValue); ok &&
		descriptorHasSpecial(descriptor, "__get__") {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{None, owner},
		)
	}
	return pushOutcome(frame, instruction, value)
}

// executeInstanceAttributeLoad follows data descriptor, instance dictionary,
// non-data descriptor, and plain class attribute precedence in that order.
func executeInstanceAttributeLoad(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
) (instructionOutcome, error) {
	classValue, classFound := owner.class.lookup(name)
	if property, ok := classValue.(*propertyValue); ok {
		return executePropertyDescriptorCall(
			frame,
			instruction,
			attributeGet,
			property,
			owner,
			nil,
		)
	}
	descriptor, isDescriptor := classValue.(*instanceValue)
	hasGet := isDescriptor && descriptorHasSpecial(descriptor, "__get__")
	if hasGet && descriptorIsData(descriptor) {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{owner, owner.class},
		)
	}
	if value, found := owner.attributes.get(name); found {
		return pushOutcome(frame, instruction, value)
	}
	if !classFound {
		if name == "__hash__" {
			method, _ := frame.runtime.nativeClassAttribute(objectNativeType, name)
			return pushOutcome(frame, instruction, bindInstanceFunction(method, owner))
		}
		if name == "__class__" {
			return pushOutcome(frame, instruction, owner.class)
		}
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"'"+owner.class.name+"' object has no attribute '"+name+"'",
			),
		}, nil
	}
	if bound, methodDescriptor := bindMethodDescriptor(classValue, owner.class); methodDescriptor {
		return pushOutcome(frame, instruction, bound)
	}
	if hasGet {
		return executeDescriptorCall(
			frame,
			instruction,
			attributeGet,
			descriptor,
			[]Value{owner, owner.class},
		)
	}
	classValue = bindInstanceFunction(classValue, owner)
	return pushOutcome(frame, instruction, classValue)
}

// executeDynamicAttributeStore applies the mutable stores shared by STORE_ATTR
// and setattr while preserving descriptor dispatch for user instances.
func executeDynamicAttributeStore(
	frame *frame,
	instruction int,
	owner Value,
	name string,
	value Value,
) (instructionOutcome, error) {
	if state := descriptorIdentity(owner); state != nil && state.class != nil {
		if _, overridden := state.class.lookup(name); !overridden && (name == "__func__" || name == "__wrapped__" || name == "__isabstractmethod__" || name == "fget" || name == "fset" || name == "fdel") {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		state.attributes.values[name] = value
		return instructionOutcome{kind: advance}, nil
	}
	switch owner := owner.(type) {
	case *Module:
		if name == "__dict__" {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		owner.globals.store(name, value)
	case *functionValue:
		if name == "__type_params__" || name == "__annotate__" ||
			name == "__annotations__" || name == "__code__" {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		if owner.attributes == nil {
			owner.attributes = newNamespace()
		}
		owner.attributes.values[name] = value
	case *typeValue:
		if owner.immutable {
			return raiseOutcome(newException("TypeError", "cannot modify immutable type '"+owner.name+"'")), nil
		}
		if name == "__abstractmethods__" {
			return executeTruthWithCall(frame, value, &truthCall{
				instruction: instruction,
				abstractStore: &abstractMethodsStore{
					instruction: instruction, class: owner, value: value,
				},
			})
		}
		if readOnlyTypeMetadata(name) {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		owner.setAttribute(name, value)
	case *instanceValue:
		if owner.class.bufferViewClass {
			return raiseOutcome(newException("AttributeError", "memoryview attributes are read-only")), nil
		}
		return executeInstanceAttributeStore(frame, instruction, owner, name, value)
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
		)), nil
	}
	return instructionOutcome{kind: advance}, nil
}

// executeDynamicAttributeDelete removes one mutable attribute or runs an
// instance data descriptor while preserving target-specific missing messages.
func executeDynamicAttributeDelete(
	frame *frame,
	instruction int,
	owner Value,
	name string,
) (instructionOutcome, error) {
	var attributes *Namespace
	missingMessage := "'" + owner.TypeName() + "' object has no attribute '" + name + "'"
	switch owner := owner.(type) {
	case *Module:
		if name == "__dict__" {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		attributes = owner.globals
		missingMessage = "module '" + owner.name + "' has no attribute '" + name + "'"
	case *functionValue:
		if name == "__type_params__" || name == "__annotate__" ||
			name == "__annotations__" || name == "__code__" {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		attributes = owner.attributes
	case *typeValue:
		if owner.immutable {
			return raiseOutcome(newException("TypeError", "cannot modify immutable type '"+owner.name+"'")), nil
		}
		if readOnlyTypeMetadata(name) {
			return raiseOutcome(newException("AttributeError", "readonly attribute")), nil
		}
		if name == "__abstractmethods__" {
			if _, found := owner.namespace.get(name); !found {
				return raiseOutcome(newException("AttributeError", name)), nil
			}
			owner.abstract = false
		}
		attributes = owner.namespace
		missingMessage = "type object '" + owner.name + "' has no attribute '" + name + "'"
	case *instanceValue:
		if owner.class.bufferViewClass {
			return raiseOutcome(newException("AttributeError", "memoryview attributes are read-only")), nil
		}
		return executeInstanceAttributeDelete(frame, instruction, owner, name)
	default:
		return raiseOutcome(newException("AttributeError", missingMessage)), nil
	}
	if attributes == nil {
		return raiseOutcome(newException("AttributeError", missingMessage)), nil
	}
	if _, found := attributes.values[name]; !found {
		return raiseOutcome(newException("AttributeError", missingMessage)), nil
	}
	if class, ok := owner.(*typeValue); ok {
		class.deleteAttribute(name)
	} else {
		attributes.delete(name)
	}
	return instructionOutcome{kind: advance}, nil
}

// executeInstanceAttributeStore sends writes through a property or user data
// descriptor before falling back to the instance namespace.
func executeInstanceAttributeStore(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
	value Value,
) (instructionOutcome, error) {
	if classValue, found := owner.class.lookup(name); found {
		if property, ok := classValue.(*propertyValue); ok {
			return executePropertyDescriptorCall(
				frame,
				instruction,
				attributeSet,
				property,
				owner,
				value,
			)
		}
		if descriptor, ok := classValue.(*instanceValue); ok && descriptorIsData(descriptor) {
			if !descriptorHasSpecial(descriptor, "__set__") {
				return instructionOutcome{
					kind:      raised,
					exception: newException("AttributeError", "__set__"),
				}, nil
			}
			return executeDescriptorCall(
				frame,
				instruction,
				attributeSet,
				descriptor,
				[]Value{owner, value},
			)
		}
	}
	owner.attributes.values[name] = value
	return instructionOutcome{kind: advance}, nil
}

// executeInstanceAttributeDelete gives a data descriptor the first deletion
// attempt and otherwise removes only an existing instance attribute.
func executeInstanceAttributeDelete(
	frame *frame,
	instruction int,
	owner *instanceValue,
	name string,
) (instructionOutcome, error) {
	if classValue, found := owner.class.lookup(name); found {
		if property, ok := classValue.(*propertyValue); ok {
			return executePropertyDescriptorCall(
				frame,
				instruction,
				attributeDelete,
				property,
				owner,
				nil,
			)
		}
		if descriptor, ok := classValue.(*instanceValue); ok && descriptorIsData(descriptor) {
			if !descriptorHasSpecial(descriptor, "__delete__") {
				return instructionOutcome{
					kind:      raised,
					exception: newException("AttributeError", "__delete__"),
				}, nil
			}
			return executeDescriptorCall(
				frame,
				instruction,
				attributeDelete,
				descriptor,
				[]Value{owner},
			)
		}
	}
	if _, found := owner.attributes.values[name]; !found {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"AttributeError",
				"'"+owner.class.name+"' object has no attribute '"+name+"'",
			),
		}, nil
	}
	delete(owner.attributes.values, name)
	return instructionOutcome{kind: advance}, nil
}

func descriptorHasSpecial(descriptor *instanceValue, name string) bool {
	_, found := descriptor.class.lookup(name)
	return found
}

func descriptorIsData(descriptor *instanceValue) bool {
	return descriptorHasSpecial(descriptor, "__set__") ||
		descriptorHasSpecial(descriptor, "__delete__")
}

// executeDescriptorCall invokes the selected descriptor method and records
// whether its result is returned to Python or discarded after mutation.
func executeDescriptorCall(
	frame *frame,
	instruction int,
	kind attributeCallKind,
	descriptor *instanceValue,
	arguments []Value,
) (instructionOutcome, error) {
	name := "__get__"
	if kind == attributeSet {
		name = "__set__"
	} else if kind == attributeDelete {
		name = "__delete__"
	}
	method, found := lookupInstanceSpecial(descriptor, name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: newException("AttributeError", name),
		}, nil
	}
	return executeAttributeCallable(frame, instruction, kind, method, arguments)
}

func executeAttributeCallable(
	frame *frame,
	instruction int,
	kind attributeCallKind,
	callable Value,
	arguments []Value,
) (instructionOutcome, error) {
	call := &attributeCall{kind: kind, instruction: instruction}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		callable,
		arguments,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.attribute = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"descriptor special method returned without a value",
		)
	}
	return finishAttributeCall(frame, call, result)
}

func finishAttributeCall(
	frame *frame,
	call *attributeCall,
	result Value,
) (instructionOutcome, error) {
	if call.kind == attributeGet {
		return pushOutcome(frame, call.instruction, result)
	}
	if call.returnNone {
		return pushOutcome(frame, call.instruction, None)
	}
	return instructionOutcome{kind: advance}, nil
}
