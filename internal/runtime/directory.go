package runtime

import (
	"sort"
	"strconv"
)

// executeBuiltinDir returns sorted names from the current frame or from the
// implemented module, class, and instance attribute stores.
func executeBuiltinDir(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dir() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dir expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}
	names := make(map[string]struct{})
	if len(arguments) == 0 {
		collectFrameNames(caller, names)
	} else {
		collectValueNames(arguments[0], names)
	}
	discardCallSegment(caller, base)
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	values := make([]Value, len(ordered))
	for index, name := range ordered {
		values[index] = &stringValue{value: name}
	}
	return pushOutcome(caller, instruction, &listValue{elements: values})
}

// collectFrameNames includes only currently bound names from dictionary locals,
// fast locals, closure cells, and free variables visible in one frame.
func collectFrameNames(frame *frame, names map[string]struct{}) {
	if frame.locals != nil {
		collectNamespaceNames(frame.locals, names)
	}
	for index, name := range frame.code.locals {
		if index < len(frame.fastLocals) && frame.fastLocals[index] != nil {
			names[name] = struct{}{}
		}
	}
	for index, name := range frame.code.cells {
		if index < len(frame.deref) && frame.deref[index] != nil &&
			frame.deref[index].value != nil {
			names[name] = struct{}{}
		}
	}
	cellCount := len(frame.code.cells)
	for index, name := range frame.code.freeVars {
		derefIndex := cellCount + index
		if derefIndex < len(frame.deref) && frame.deref[derefIndex] != nil &&
			frame.deref[derefIndex].value != nil {
			names[name] = struct{}{}
		}
	}
}

// collectValueNames follows fixed runtime metadata and user MRO namespaces
// without invoking a custom __dir__ method.
func collectValueNames(value Value, names map[string]struct{}) {
	names["__class__"] = struct{}{}
	switch value := value.(type) {
	case *Module:
		collectNamespaceNames(value.globals, names)
	case *typeValue:
		collectTypeNames(value, names)
	case *instanceValue:
		collectNamespaceNames(value.attributes, names)
		collectTypeNames(value.class, names)
	case *nativeTypeValue:
		collectNativeTypeNames(names)
	case *exceptionTypeValue:
		collectNativeTypeNames(names)
	}
}

func collectTypeNames(class *typeValue, names map[string]struct{}) {
	for _, current := range class.mro {
		collectNamespaceNames(current.namespace, names)
	}
	for _, name := range []string{
		"__name__",
		"__qualname__",
		"__module__",
		"__base__",
		"__bases__",
		"__mro__",
		"__annotations__",
	} {
		names[name] = struct{}{}
	}
}

func collectNativeTypeNames(names map[string]struct{}) {
	for _, name := range []string{
		"__name__",
		"__qualname__",
		"__module__",
		"__base__",
		"__bases__",
		"__mro__",
	} {
		names[name] = struct{}{}
	}
}

func collectNamespaceNames(namespace *Namespace, names map[string]struct{}) {
	if namespace == nil {
		return
	}
	for name := range namespace.values {
		names[name] = struct{}{}
	}
}
