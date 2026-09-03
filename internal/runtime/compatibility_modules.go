package runtime

import (
	"fmt"
	"strings"
)

// newASTBootstrapModule creates the hierarchy and constructors needed by the
// standard library's AST inspection paths.
func newASTBootstrapModule() *Module {
	module := newSystemModule("_ast", "")
	ast := newBootstrapClass("AST", nil)
	module.globals.values["AST"] = ast
	categories := make(map[string]*typeValue)
	for _, name := range []string{
		"mod", "stmt", "expr", "expr_context", "boolop", "operator", "unaryop", "cmpop",
		"comprehension", "excepthandler", "arguments", "arg", "keyword", "alias", "withitem",
		"match_case", "pattern", "type_ignore", "type_param",
	} {
		categories[name] = newBootstrapClass(name, ast)
		module.globals.values[name] = categories[name]
	}
	constructors := map[string][]string{
		"mod": {"Module", "Interactive", "Expression", "FunctionType"},
		"stmt": {
			"FunctionDef", "AsyncFunctionDef", "ClassDef", "Return", "Delete", "Assign", "TypeAlias",
			"AugAssign", "AnnAssign", "For", "AsyncFor", "While", "If", "With", "AsyncWith", "Match",
			"Raise", "Try", "TryStar", "Assert", "Import", "ImportFrom", "Global", "Nonlocal", "Expr",
			"Pass", "Break", "Continue",
		},
		"expr": {
			"BoolOp", "NamedExpr", "BinOp", "UnaryOp", "Lambda", "IfExp", "Dict", "Set", "ListComp",
			"SetComp", "DictComp", "GeneratorExp", "Await", "Yield", "YieldFrom", "Compare", "Call",
			"FormattedValue", "Interpolation", "JoinedStr", "TemplateStr", "Constant", "Attribute",
			"Subscript", "Starred", "Name", "List", "Tuple", "Slice",
		},
		"expr_context":  {"Load", "Store", "Del"},
		"boolop":        {"And", "Or"},
		"operator":      {"Add", "Sub", "Mult", "MatMult", "Div", "Mod", "Pow", "LShift", "RShift", "BitOr", "BitXor", "BitAnd", "FloorDiv"},
		"unaryop":       {"Invert", "Not", "UAdd", "USub"},
		"cmpop":         {"Eq", "NotEq", "Lt", "LtE", "Gt", "GtE", "Is", "IsNot", "In", "NotIn"},
		"excepthandler": {"ExceptHandler"},
		"pattern":       {"MatchValue", "MatchSingleton", "MatchSequence", "MatchMapping", "MatchClass", "MatchStar", "MatchAs", "MatchOr"},
		"type_ignore":   {"TypeIgnore"},
		"type_param":    {"TypeVar", "ParamSpec", "TypeVarTuple"},
	}
	astFields := map[string][]string{
		"Name": {"id", "ctx"}, "Constant": {"value", "kind"},
		"Dict": {"keys", "values"}, "Slice": {"lower", "upper", "step"},
		"Tuple": {"elts", "ctx"}, "Subscript": {"value", "slice", "ctx"},
		"Attribute": {"value", "attr", "ctx"}, "Call": {"func", "args", "keywords"},
		"Starred": {"value", "ctx"}, "BinOp": {"left", "op", "right"},
		"Compare": {"left", "ops", "comparators"}, "UnaryOp": {"op", "operand"},
		"Interpolation": {"value", "str", "conversion", "format_spec"},
		"TemplateStr":   {"values"}, "keyword": {"arg", "value"},
	}
	newASTNode := func(name string, base *typeValue) *typeValue {
		class := newBootstrapClass(name, base)
		fields := append([]string(nil), astFields[name]...)
		class.constructor = func(
			actualClass *typeValue,
			_ *frame,
			arguments []Value,
			keywords *dictValue,
		) (Value, *Exception, error) {
			if len(arguments) > len(fields) {
				return nil, newException("TypeError", name+"() takes fewer arguments"), nil
			}
			instance := &instanceValue{class: actualClass, attributes: newNamespace()}
			for index, value := range arguments {
				instance.attributes.values[fields[index]] = value
			}
			if keywords != nil {
				for _, entry := range keywords.entries {
					key, ok := entry.key.(*stringValue)
					if !ok {
						return nil, newException("TypeError", "AST keywords must be strings"), nil
					}
					instance.attributes.values[key.value] = entry.value
				}
			}
			return instance, nil, nil
		}
		return class
	}
	for category, names := range constructors {
		for _, name := range names {
			module.globals.values[name] = newASTNode(name, categories[category])
		}
	}
	module.globals.values["keyword"] = newASTNode("keyword", categories["keyword"])
	for name, value := range map[string]int64{
		"PyCF_ONLY_AST":      1024,
		"PyCF_TYPE_COMMENTS": 4096,
		"PyCF_OPTIMIZED_AST": 0x8000,
	} {
		module.globals.values[name] = newInt64(value)
	}
	return module
}

func newOpcodeBootstrapModule() *Module {
	module := newSystemModule("_opcode", "")
	for _, name := range []string{
		"has_arg", "has_const", "has_name", "has_jump", "has_free", "has_local", "has_exc",
	} {
		setNativeFunction(module, name, 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return falseSingleton, nil, nil
			})
	}
	setNativeKeywordFunction(module, "stack_effect", 1, 3,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(0), nil, nil
		})
	for _, name := range []string{
		"get_intrinsic1_descs", "get_intrinsic2_descs", "get_special_method_names", "get_nb_ops",
	} {
		setNativeFunction(module, name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &listValue{}, nil, nil
			})
	}
	setNativeFunction(module, "get_executor", 2, 2,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	return module
}

// newCollectionsBootstrapModule provides deque and defaultdict primitives used by collections.
func newCollectionsBootstrapModule() *Module {
	module := newSystemModule("_collections", "")
	dequeClass := newBootstrapClass("deque", nil)
	dequeClass.module = "collections"
	dequeClass.constructor = func(
		_ *typeValue,
		caller *frame,
		arguments []Value,
		_ *dictValue,
	) (Value, *Exception, error) {
		deque := &dequeValue{}
		if len(arguments) == 1 {
			iterator, ok := newIterator(arguments[0])
			if !ok {
				return nil, newException("TypeError", "deque argument is not iterable"), nil
			}
			for {
				value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
				if err != nil || exception != nil {
					return nil, exception, err
				}
				if !available {
					break
				}
				deque.elements = append(deque.elements, value)
			}
		}
		return deque, nil, nil
	}
	module.globals.values["deque"] = dequeClass
	module.globals.values["defaultdict"] = builtinTypeNamed("dict")
	setNativeFunction(module, "_count_elements", 2, 2,
		func(caller *frame, arguments []Value) (Value, *Exception, error) {
			var mapping *dictValue
			switch value := arguments[0].(type) {
			case *dictValue:
				mapping = value
			case *instanceValue:
				mapping = value.mapping
			}
			if mapping == nil {
				return nil, newException("TypeError", "first argument must be a mapping"), nil
			}
			iterator, exception, err := newIteratorForFrame(caller, arguments[1])
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if iterator == nil {
				return nil, newException("TypeError", "second argument must be iterable"), nil
			}
			for {
				element, available, exception, err := nextNativeIterator(caller.runtime, iterator)
				if err != nil || exception != nil {
					return nil, exception, err
				}
				if !available {
					return None, nil, nil
				}
				count := int64(0)
				if current, found, getException := mapping.get(element); getException != nil {
					return nil, getException, nil
				} else if found {
					integer, ok := integerOperand(current)
					if !ok || !integer.IsInt64() {
						return nil, newException("TypeError", "counter value must be an integer"), nil
					}
					count = integer.Int64()
				}
				if setException := mapping.set(element, newInt64(count+1)); setException != nil {
					return nil, setException, nil
				}
			}
		})
	return module
}

func newErrnoBootstrapModule() *Module {
	module := newSystemModule("errno", "")
	constants := map[string]int64{
		"EPERM": 1, "ENOENT": 2, "EINTR": 4, "EIO": 5, "EBADF": 9,
		"EAGAIN": 11, "EACCES": 13, "EEXIST": 17, "ENOTDIR": 20,
		"EISDIR": 21, "EINVAL": 22, "EMFILE": 24, "ENOSPC": 28,
		"EPIPE": 32, "ELOOP": 40, "ENOTEMPTY": 39, "ETIMEDOUT": 110,
	}
	errorCode := &dictValue{}
	for name, number := range constants {
		module.globals.values[name] = newInt64(number)
		_ = errorCode.set(newInt64(number), &stringValue{value: name})
	}
	module.globals.values["errorcode"] = errorCode
	return module
}

type dirsOnSysPathContext struct {
	runtime  *Runtime
	paths    []Value
	previous []Value
}

func (*dirsOnSysPathContext) TypeName() string { return "DirsOnSysPath" }
func (*dirsOnSysPathContext) Repr() string     { return "<DirsOnSysPath>" }
func (*dirsOnSysPathContext) isValue()         {}
func (context *dirsOnSysPathContext) attribute(name string) (Value, bool) {
	switch name {
	case "__enter__":
		return nativeFunctionNamed("DirsOnSysPath.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				path := context.runtime.modules["sys"].globals.values["path"].(*listValue)
				context.previous = append([]Value(nil), path.elements...)
				path.elements = append(append([]Value(nil), context.paths...), path.elements...)
				return context, nil, nil
			}), true
	case "__exit__":
		return nativeFunctionNamed("DirsOnSysPath.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				path := context.runtime.modules["sys"].globals.values["path"].(*listValue)
				path.elements = append([]Value(nil), context.previous...)
				return falseSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (runtime *Runtime) newDirsOnSysPath(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	for _, argument := range arguments {
		if _, ok := argument.(*stringValue); !ok {
			return nil, newException("TypeError", "paths must be strings"), nil
		}
	}
	return &dirsOnSysPathContext{
		runtime: runtime, paths: append([]Value(nil), arguments...),
	}, nil, nil
}

type moduleUncacheContext struct {
	runtime *Runtime
	names   []string
}

func (*moduleUncacheContext) TypeName() string { return "uncache" }
func (*moduleUncacheContext) Repr() string     { return "<uncache>" }
func (*moduleUncacheContext) isValue()         {}
func (context *moduleUncacheContext) attribute(name string) (Value, bool) {
	switch name {
	case "__enter__":
		return nativeFunctionNamed("uncache.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				for _, name := range context.names {
					context.runtime.deleteModule(name)
				}
				return context, nil, nil
			}), true
	case "__exit__":
		return nativeFunctionNamed("uncache.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				for _, name := range context.names {
					context.runtime.deleteModule(name)
				}
				return falseSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (runtime *Runtime) newModuleUncacheContext(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	names := make([]string, len(arguments))
	for index, argument := range arguments {
		name, ok := argument.(*stringValue)
		if !ok {
			return nil, newException("TypeError", "module names must be strings"), nil
		}
		names[index] = name.value
	}
	return &moduleUncacheContext{runtime: runtime, names: names}, nil, nil
}

type patchContextValue struct {
	runtime     *Runtime
	target      string
	replacement Value
	previous    Value
}

func (*patchContextValue) TypeName() string { return "_patch" }
func (*patchContextValue) Repr() string     { return "<unittest.mock.patch>" }
func (*patchContextValue) isValue()         {}
func (context *patchContextValue) attribute(name string) (Value, bool) {
	switch name {
	case "__enter__":
		return nativeFunctionNamed("patch.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if context.target != "builtins.__import__" {
					return nil, newException("NotImplementedError", "unsupported patch target"), nil
				}
				context.previous = context.runtime.builtins.values["__import__"]
				context.runtime.builtins.values["__import__"] = context.replacement
				context.runtime.modules["builtins"].globals.values["__import__"] = context.replacement
				return context.replacement, nil, nil
			}), true
	case "__exit__":
		return nativeFunctionNamed("patch.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if context.target == "builtins.__import__" {
					context.runtime.builtins.values["__import__"] = context.previous
					context.runtime.modules["builtins"].globals.values["__import__"] = context.previous
				}
				return falseSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (runtime *Runtime) newPatchContext(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	target, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "patch target must be a string"), nil
	}
	return &patchContextValue{
		runtime: runtime, target: target.value, replacement: arguments[1],
	}, nil, nil
}

// newSignalBootstrapModule keeps interpreter-local signal handlers without ambient OS access.
func (runtime *Runtime) newSignalBootstrapModule() *Module {
	module := newSystemModule("_signal", "")
	for name, value := range map[string]int64{
		"SIG_DFL": 0, "SIG_IGN": 1, "SIGINT": 2, "SIGTERM": 15, "NSIG": 65,
	} {
		module.globals.values[name] = newInt64(value)
	}
	defaultIntHandler := nativeFunctionNamed("default_int_handler", 2, 2,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return nil, newException("KeyboardInterrupt", ""), nil
		})
	module.globals.values["default_int_handler"] = defaultIntHandler
	if _, found := runtime.signalHandlers[2]; !found {
		runtime.signalHandlers[2] = defaultIntHandler
	}
	setNativeFunction(module, "signal", 2, 2,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			number, ok := integerOperand(arguments[0])
			if !ok || !number.IsInt64() {
				return nil, newException("TypeError", "signal number must be an integer"), nil
			}
			previous := Value(newInt64(0))
			if current, found := runtime.signalHandlers[number.Int64()]; found {
				previous = current
			}
			runtime.signalHandlers[number.Int64()] = arguments[1]
			return previous, nil, nil
		})
	setNativeFunction(module, "getsignal", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			number, ok := integerOperand(arguments[0])
			if !ok || !number.IsInt64() {
				return nil, newException("TypeError", "signal number must be an integer"), nil
			}
			if current, found := runtime.signalHandlers[number.Int64()]; found {
				return current, nil, nil
			}
			return newInt64(0), nil, nil
		})
	setNativeFunction(module, "raise_signal", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	return module
}

type dequeValue struct {
	elements []Value
}

func (*dequeValue) TypeName() string { return "collections.deque" }
func (deque *dequeValue) Repr() string {
	return "deque(" + (&listValue{elements: deque.elements}).Repr() + ")"
}
func (*dequeValue) isValue() {}

// attribute exposes the deque operations required by contextlib and unittest.
func (deque *dequeValue) attribute(name string) (Value, bool) {
	switch name {
	case "append":
		return nativeFunctionNamed("deque.append", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				deque.elements = append(deque.elements, arguments[0])
				return None, nil, nil
			}), true
	case "appendleft":
		return nativeFunctionNamed("deque.appendleft", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				deque.elements = append([]Value{arguments[0]}, deque.elements...)
				return None, nil, nil
			}), true
	case "pop":
		return nativeFunctionNamed("deque.pop", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if len(deque.elements) == 0 {
					return nil, newException("IndexError", "pop from an empty deque"), nil
				}
				last := len(deque.elements) - 1
				value := deque.elements[last]
				deque.elements = deque.elements[:last]
				return value, nil, nil
			}), true
	case "popleft":
		return nativeFunctionNamed("deque.popleft", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if len(deque.elements) == 0 {
					return nil, newException("IndexError", "pop from an empty deque"), nil
				}
				value := deque.elements[0]
				deque.elements = deque.elements[1:]
				return value, nil, nil
			}), true
	case "remove":
		return nativeFunctionNamed("deque.remove", 1, 1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				for index, value := range deque.elements {
					equal, exception, err := valuesEqualForFrame(caller, value, arguments[0])
					if err != nil || exception != nil {
						return nil, exception, err
					}
					if equal {
						copy(deque.elements[index:], deque.elements[index+1:])
						deque.elements[len(deque.elements)-1] = nil
						deque.elements = deque.elements[:len(deque.elements)-1]
						return None, nil, nil
					}
				}
				return nil, newException("ValueError", "deque.remove(x): x is not in deque"), nil
			}), true
	default:
		return nil, false
	}
}

var _ Value = (*dequeValue)(nil)

func (runtime *Runtime) newImportlibBootstrapModule() *Module {
	module := newSystemModule("importlib", "")
	module.isPackage = true
	module.searchLocations = []string{"importlib"}
	setNativeFunction(module, "invalidate_caches", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	setNativeFunction(module, "reload", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			return arguments[0], nil, nil
		})
	setNativeFunction(module, "import_module", 1, 2,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			name, ok := arguments[0].(*stringValue)
			if !ok {
				return nil, newException("TypeError", "module name must be a string"), nil
			}
			if imported, found := runtime.modules[name.value]; found {
				return imported, nil, nil
			}
			if imported, found := runtime.loadSystemModule(name.value); found {
				return imported, nil, nil
			}
			return nil, newException("ModuleNotFoundError", "No module named '"+name.value+"'"), nil
		})
	return module
}

func newItertoolsModule() *Module {
	module := newSystemModule("itertools", "")
	setNativeFunction(module, "accumulate", 1, 2, itertoolsAccumulate)
	setNativeFunction(module, "chain", 0, -1, itertoolsChain)
	setNativeFunction(module, "repeat", 1, 2, itertoolsRepeat)
	setNativeFunction(module, "count", 0, 2, itertoolsCount)
	setNativeFunction(module, "permutations", 1, 2, itertoolsPermutations)
	setNativeFunction(module, "product", 0, -1, itertoolsProduct)
	setNativeFunction(module, "starmap", 2, 2, itertoolsDeferred)
	setNativeFunction(module, "islice", 2, 4, itertoolsDeferred)
	module.globals.values["batched"] = nativeKeywordAwareFunctionNamed(
		"itertools.batched", 2, 2, itertoolsBatched,
	)
	return module
}

// itertoolsAccumulate eagerly computes the running totals while preserving an
// iterator result. This covers the numeric accumulation used by random and
// still honors an explicitly supplied Python combining function.
func itertoolsAccumulate(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	iterator, exception, err := newIteratorForFrame(caller, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException("TypeError", "accumulate argument is not iterable"), nil
	}
	results := make([]Value, 0)
	var total Value
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			break
		}
		if total == nil {
			total = value
		} else if len(arguments) == 2 && arguments[1] != None {
			total, exception, err = callValueSynchronously(
				caller, arguments[1], []Value{total, value},
			)
			if err != nil || exception != nil {
				return nil, exception, err
			}
		} else {
			total, exception = accumulatedSum(total, value)
			if exception != nil {
				return nil, exception, nil
			}
		}
		results = append(results, total)
	}
	return &sequenceIterator{sequence: &listValue{elements: results}}, nil, nil
}

// accumulatedSum adds the numeric combinations accepted by the compatibility
// implementation of itertools.accumulate.
func accumulatedSum(left Value, right Value) (Value, *Exception) {
	leftInteger, leftIsInteger := integerOperand(left)
	rightInteger, rightIsInteger := integerOperand(right)
	if leftIsInteger && rightIsInteger {
		leftInteger.Add(&leftInteger, &rightInteger)
		return &intValue{value: leftInteger}, nil
	}
	leftFloat, leftIsNumeric := numericFloat(left)
	rightFloat, rightIsNumeric := numericFloat(right)
	if leftIsNumeric && rightIsNumeric {
		return &floatValue{value: leftFloat + rightFloat}, nil
	}
	if leftText, ok := left.(*stringValue); ok {
		if rightText, ok := right.(*stringValue); ok {
			return &stringValue{value: leftText.value + rightText.value}, nil
		}
	}
	return nil, newException(
		"TypeError", "unsupported operand type(s) for +: '"+left.TypeName()+"' and '"+right.TypeName()+"'",
	)
}

// itertoolsBatched eagerly groups an iterable into fixed-length tuple batches.
func itertoolsBatched(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	length, ok := integerOperand(arguments[1])
	if !ok || !length.IsInt64() {
		return nil, newException("TypeError", "n must be an integer"), nil
	}
	if length.Sign() <= 0 {
		return nil, newException("ValueError", "n must be at least one"), nil
	}
	strict := false
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok || name.value != "strict" {
				return nil, newException("TypeError", "batched() got an unexpected keyword argument"), nil
			}
			value, exception, err := truthValueForFrame(caller, entry.value)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			strict = value
		}
	}
	iterator, exception, err := newIteratorForRuntime(caller.runtime, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException("TypeError", "batched argument is not iterable"), nil
	}
	batches := make([]Value, 0)
	for {
		batch := make([]Value, 0, int(length.Int64()))
		for range int(length.Int64()) {
			value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if !available {
				if len(batch) != 0 {
					if strict {
						return nil, newException("ValueError", "batched(): incomplete batch"), nil
					}
					batches = append(batches, &tupleValue{elements: batch})
				}
				return &sequenceIterator{sequence: &listValue{elements: batches}}, nil, nil
			}
			batch = append(batch, value)
		}
		batches = append(batches, &tupleValue{elements: batch})
	}
}

func itertoolsCount(_ *frame, arguments []Value) (Value, *Exception, error) {
	start := int64(0)
	step := int64(1)
	for index, argument := range arguments {
		integer, ok := integerOperand(argument)
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "count arguments must be integers"), nil
		}
		if index == 0 {
			start = integer.Int64()
		} else {
			step = integer.Int64()
		}
	}
	return &countIterator{value: start, step: step}, nil, nil
}

// itertoolsPermutations eagerly enumerates selections in input order.
func itertoolsPermutations(_ *frame, arguments []Value) (Value, *Exception, error) {
	elements, exception := iterableElements(arguments[0], "permutations")
	if exception != nil {
		return nil, exception, nil
	}
	length := len(elements)
	if len(arguments) == 2 {
		integer, ok := integerOperand(arguments[1])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "permutations length must be an integer"), nil
		}
		length = int(integer.Int64())
	}
	results := make([]Value, 0)
	if length >= 0 && length <= len(elements) {
		used := make([]bool, len(elements))
		selection := make([]Value, length)
		var visit func(int)
		visit = func(depth int) {
			if depth == length {
				results = append(results, &tupleValue{elements: append([]Value(nil), selection...)})
				return
			}
			for index, element := range elements {
				if used[index] {
					continue
				}
				used[index] = true
				selection[depth] = element
				visit(depth + 1)
				used[index] = false
			}
		}
		visit(0)
	}
	return &sequenceIterator{sequence: &listValue{elements: results}}, nil, nil
}

func itertoolsProduct(_ *frame, arguments []Value) (Value, *Exception, error) {
	pools := make([][]Value, len(arguments))
	for index, argument := range arguments {
		var exception *Exception
		pools[index], exception = iterableElements(argument, "product")
		if exception != nil {
			return nil, exception, nil
		}
	}
	results := make([]Value, 0)
	selection := make([]Value, len(pools))
	var visit func(int)
	visit = func(depth int) {
		if depth == len(pools) {
			results = append(results, &tupleValue{elements: append([]Value(nil), selection...)})
			return
		}
		for _, element := range pools[depth] {
			selection[depth] = element
			visit(depth + 1)
		}
	}
	visit(0)
	return &sequenceIterator{sequence: &listValue{elements: results}}, nil, nil
}

func iterableElements(value Value, operation string) ([]Value, *Exception) {
	iterator, ok := newIterator(value)
	if !ok {
		return nil, newException("TypeError", operation+" argument is not iterable")
	}
	elements := make([]Value, 0)
	for {
		element, available, exception := iterator.next()
		if exception != nil {
			return nil, exception
		}
		if !available {
			return elements, nil
		}
		elements = append(elements, element)
	}
}

// itertoolsChain drains each input iterable into one sequential iterator.
func itertoolsChain(_ *frame, arguments []Value) (Value, *Exception, error) {
	elements := make([]Value, 0)
	for _, argument := range arguments {
		iterator, ok := newIterator(argument)
		if !ok {
			return nil, newException("TypeError", "chain argument is not iterable"), nil
		}
		for {
			value, available, exception := iterator.next()
			if exception != nil {
				return nil, exception, nil
			}
			if !available {
				break
			}
			elements = append(elements, value)
		}
	}
	return &sequenceIterator{sequence: &listValue{elements: elements}}, nil, nil
}

func itertoolsRepeat(_ *frame, arguments []Value) (Value, *Exception, error) {
	remaining := int64(-1)
	if len(arguments) == 2 {
		integer, ok := integerOperand(arguments[1])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "repeat count must be an integer"), nil
		}
		remaining = integer.Int64()
	}
	return &repeatIterator{value: arguments[0], remaining: remaining}, nil, nil
}

func itertoolsDeferred(_ *frame, _ []Value) (Value, *Exception, error) {
	return &sequenceIterator{sequence: &listValue{}}, nil, nil
}

type repeatIterator struct {
	value     Value
	remaining int64
}

type countIterator struct {
	value int64
	step  int64
}

func (*countIterator) TypeName() string { return "itertools.count" }
func (*countIterator) Repr() string     { return "count(...)" }
func (*countIterator) isValue()         {}
func (iterator *countIterator) attribute(name string) (Value, bool) {
	if name != "__next__" {
		return nil, false
	}
	return nativeFunctionNamed("count.__next__", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			value := iterator.value
			iterator.value += iterator.step
			return newInt64(value), nil, nil
		}), true
}

func (iterator *countIterator) next() (Value, bool, *Exception) {
	value := iterator.value
	iterator.value += iterator.step
	return newInt64(value), true, nil
}

func (*repeatIterator) TypeName() string { return "itertools.repeat" }
func (*repeatIterator) Repr() string     { return "repeat(...)" }
func (*repeatIterator) isValue()         {}
func (iterator *repeatIterator) attribute(name string) (Value, bool) {
	if name != "__next__" {
		return nil, false
	}
	return nativeFunctionNamed("repeat.__next__", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			value, available, exception := iterator.next()
			if exception != nil {
				return nil, exception, nil
			}
			if !available {
				return nil, newException("StopIteration", ""), nil
			}
			return value, nil, nil
		}), true
}
func (iterator *repeatIterator) next() (Value, bool, *Exception) {
	if iterator.remaining == 0 {
		return nil, false, nil
	}
	if iterator.remaining > 0 {
		iterator.remaining--
	}
	return iterator.value, true, nil
}

func newOperatorModule() *Module {
	module := newSystemModule("operator", "")
	setNativeFunction(module, "eq", 2, 2, operatorEqual)
	setNativeFunction(module, "index", 1, 1, operatorIndex)
	setNativeFunction(module, "getitem", 2, 2,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			value, exception := directSubscript(arguments[0], arguments[1])
			return value, exception, nil
		})
	setNativeFunction(module, "itemgetter", 1, -1, operatorItemGetter)
	setNativeFunction(module, "attrgetter", 1, -1, operatorAttrGetter)
	return module
}

// operatorIndex accepts built-in integers or dispatches a user __index__ method.
func operatorIndex(caller *frame, arguments []Value) (Value, *Exception, error) {
	if integer, ok := integerOperand(arguments[0]); ok {
		return &intValue{value: integer}, nil, nil
	}
	if instance, ok := arguments[0].(*instanceValue); ok {
		method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__index__")
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if found {
			value, exception, err := callValueSynchronously(
				caller, method, nil,
			)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if integer, ok := integerOperand(value); ok {
				return &intValue{value: integer}, nil, nil
			}
			return nil, newException("TypeError", "__index__ returned non-int"), nil
		}
	}
	return nil, newException(
		"TypeError", "'"+arguments[0].TypeName()+"' object cannot be interpreted as an integer",
	), nil
}

// operatorAttrGetter builds a callable that resolves one or more dotted attributes.
func operatorAttrGetter(_ *frame, arguments []Value) (Value, *Exception, error) {
	names := make([]string, len(arguments))
	for index, argument := range arguments {
		name, ok := argument.(*stringValue)
		if !ok {
			return nil, newException("TypeError", "attribute name must be a string"), nil
		}
		names[index] = name.value
	}
	return nativeFunctionNamed("operator.attrgetter", 1, 1,
		func(_ *frame, values []Value) (Value, *Exception, error) {
			results := make([]Value, len(names))
			for index, name := range names {
				current := values[0]
				for _, component := range strings.Split(name, ".") {
					var found bool
					current, found = directAttribute(current, component)
					if !found {
						return nil, newException("AttributeError", "object has no attribute '"+component+"'"), nil
					}
				}
				results[index] = current
			}
			if len(results) == 1 {
				return results[0], nil, nil
			}
			return &tupleValue{elements: results}, nil, nil
		}), nil, nil
}

func operatorEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pythonBool(valuesEqual(arguments[0], arguments[1])), nil, nil
}

func operatorItemGetter(_ *frame, arguments []Value) (Value, *Exception, error) {
	indices := append([]Value(nil), arguments...)
	return nativeFunctionNamed("operator.itemgetter", 1, 1,
		func(_ *frame, values []Value) (Value, *Exception, error) {
			results := make([]Value, len(indices))
			for index, item := range indices {
				value, exception := directSubscript(values[0], item)
				if exception != nil {
					return nil, exception, nil
				}
				results[index] = value
			}
			if len(results) == 1 {
				return results[0], nil, nil
			}
			return &tupleValue{elements: results}, nil, nil
		}), nil, nil
}

// directSubscript applies built-in mapping, text, and sequence subscription.
func directSubscript(container, index Value) (Value, *Exception) {
	if instance, ok := container.(*instanceValue); ok {
		if instance.mapping != nil {
			return directSubscript(instance.mapping, index)
		}
		if instance.sequence != nil {
			return directSubscript(instance.sequence, index)
		}
		if instance.tuple != nil {
			return directSubscript(instance.tuple, index)
		}
	}
	if dictionary, ok := container.(*dictValue); ok {
		value, found, exception := dictionary.get(index)
		if exception != nil {
			return nil, exception
		}
		if !found {
			return nil, newException("KeyError", index.Repr())
		}
		return value, nil
	}
	if _, ok := container.(*stringValue); ok {
		return executeTextSubscript(container, index)
	}
	if _, ok := container.(*bytesValue); ok {
		return executeTextSubscript(container, index)
	}
	integer, ok := integerOperand(index)
	if !ok || !integer.IsInt64() {
		return nil, newException("TypeError", "indices must be integers")
	}
	var elements []Value
	switch sequence := container.(type) {
	case *listValue:
		elements = sequence.elements
	case *tupleValue:
		elements = sequence.elements
	default:
		return nil, newException("TypeError", "object is not subscriptable")
	}
	position := integer.Int64()
	if position < 0 {
		position += int64(len(elements))
	}
	if position < 0 || position >= int64(len(elements)) {
		return nil, newException("IndexError", "index out of range")
	}
	return elements[position], nil
}

func reprlibRecursiveRepr(_ *frame, _ []Value) (Value, *Exception, error) {
	return nativeFunctionNamed("reprlib.recursive_repr.decorator", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			return arguments[0], nil, nil
		}), nil, nil
}

// pkgutilResolveName imports the longest module prefix and follows the remaining attributes.
func (runtime *Runtime) pkgutilResolveName(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	target, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "resolve_name() argument must be str"), nil
	}
	moduleName, attributePath, explicit := strings.Cut(target.value, ":")
	if explicit {
		module, exception, err := runtime.importModuleSynchronously(
			moduleName, &tupleValue{elements: []Value{&stringValue{value: "*"}}},
		)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		return resolveNamedAttributes(module, attributePath)
	}
	parts := strings.Split(moduleName, ".")
	for count := len(parts); count > 0; count-- {
		prefix := strings.Join(parts[:count], ".")
		module, exception, err := runtime.importModuleSynchronously(
			prefix, &tupleValue{elements: []Value{&stringValue{value: "*"}}},
		)
		if err != nil {
			return nil, nil, err
		}
		if exception != nil {
			if exception.class == moduleNotFoundErrorType {
				continue
			}
			return nil, exception, nil
		}
		return resolveNamedAttributes(module, strings.Join(parts[count:], "."))
	}
	return nil, missingModuleException(moduleName), nil
}

func resolveNamedAttributes(owner Value, path string) (Value, *Exception, error) {
	current := owner
	if path == "" {
		return current, nil, nil
	}
	for _, name := range strings.Split(path, ".") {
		value, found := directAttribute(current, name)
		if !found {
			return nil, newException("AttributeError", missingAttributeMessage(current, name)), nil
		}
		current = value
	}
	return current, nil, nil
}

func newWeakrefModule() *Module {
	module := newSystemModule("_weakref", "")
	referenceType := newBootstrapClass("ReferenceType", nil)
	referenceType.constructor = constructWeakReference
	referenceType.namespace.values["__new__"] = nativeMethodNamed(
		"ReferenceType.__new__", 2, 3,
		func(caller *frame, arguments []Value) (Value, *Exception, error) {
			return newWeakReference(caller.runtime, arguments[1:])
		},
	)
	referenceType.namespace.values["__init__"] = nativeMethodNamed(
		"ReferenceType.__init__", 1, 3,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		},
	)
	proxyType := newBootstrapClass("ProxyType", nil)
	callableProxyType := newBootstrapClass("CallableProxyType", proxyType)
	module.globals.values["ref"] = referenceType
	module.globals.values["ReferenceType"] = referenceType
	module.globals.values["ProxyType"] = proxyType
	module.globals.values["CallableProxyType"] = callableProxyType
	setNativeFunction(module, "getweakrefcount", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(0), nil, nil
		})
	setNativeFunction(module, "getweakrefs", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &listValue{}, nil, nil
		})
	setNativeFunction(module, "proxy", 1, 2,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			return arguments[0], nil, nil
		})
	setNativeFunction(module, "_remove_dead_weakref", 2, 2,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	return module
}

type weakReferenceValue struct {
	referent   Value
	attributes *Namespace
}

func (*weakReferenceValue) TypeName() string { return "weakref.ReferenceType" }
func (*weakReferenceValue) Repr() string     { return "<weakref at 0x1>" }
func (*weakReferenceValue) isValue()         {}
func (reference *weakReferenceValue) attribute(name string) (Value, bool) {
	if reference.attributes == nil {
		return nil, false
	}
	return reference.attributes.get(name)
}

func constructWeakReference(
	_ *typeValue,
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	if keywordCount(keywords) != 0 {
		return nil, newException("TypeError", "ReferenceType() takes no keyword arguments"), nil
	}
	return newWeakReference(caller.runtime, arguments)
}

func newWeakReference(runtimeState *Runtime, arguments []Value) (Value, *Exception, error) {
	if len(arguments) < 1 || len(arguments) > 2 {
		return nil, newException("TypeError", "ReferenceType() takes one or two arguments"), nil
	}
	reference := &weakReferenceValue{referent: arguments[0], attributes: newNamespace()}
	runtimeState.weakRefs = append(runtimeState.weakRefs, reference)
	return reference, nil, nil
}

var _ Value = (*weakReferenceValue)(nil)

// newThreadModule provides interpreter-local locks and deterministic worker
// execution for the threading surfaces used by unittest.mock.
func newThreadModule() *Module {
	module := newSystemModule("_thread", "")
	identifier := func(_ *frame, _ []Value) (Value, *Exception, error) {
		return newInt64(1), nil, nil
	}
	for _, name := range []string{"get_ident", "get_native_id", "_get_main_thread_ident"} {
		setNativeFunction(module, name, 0, 0, identifier)
	}
	setNativeFunction(module, "daemon_threads_allowed", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return trueSingleton, nil, nil
		})
	setNativeFunction(module, "_is_main_interpreter", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return trueSingleton, nil, nil
		})
	setNativeFunction(module, "_shutdown", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return None, nil, nil
		})
	setNativeFunction(module, "stack_size", 0, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return newInt64(0), nil, nil
		})
	setNativeKeywordFunction(module, "_make_thread_handle", 0, -1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &threadHandleValue{}, nil, nil
		})
	module.globals.values["start_joinable_thread"] = nativeKeywordAwareFunctionNamed(
		"_thread.start_joinable_thread", 1, 1,
		func(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
			handle := &threadHandleValue{}
			if keywords != nil {
				if selected, found, exception := keywords.get(&stringValue{value: "handle"}); exception != nil {
					return nil, exception, nil
				} else if supplied, ok := selected.(*threadHandleValue); found && ok {
					handle = supplied
				}
			}
			var exception *Exception
			var err error
			if bootstrap, ok := arguments[0].(*boundMethodValue); ok && isThreadPoolBootstrap(bootstrap) {
				exception, err = runOneThreadPoolWorkItem(caller, bootstrap.self)
			} else {
				_, exception, err = callValueSynchronously(caller, arguments[0], nil)
			}
			if err != nil || exception != nil {
				return nil, exception, err
			}
			handle.done = true
			return handle, nil, nil
		})
	for _, name := range []string{"RLock", "allocate_lock"} {
		setNativeFunction(module, name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &lockValue{}, nil, nil
			})
	}
	module.globals.values["LockType"] = &typeValue{
		name: "lock", qualifiedName: "lock", module: "_thread", namespace: newNamespace(),
		constructor: func(
			_ *typeValue, _ *frame, _ []Value, _ *dictValue,
		) (Value, *Exception, error) {
			return &lockValue{}, nil, nil
		},
	}
	module.globals.values["_ThreadHandle"] = &typeValue{
		name: "_ThreadHandle", qualifiedName: "_ThreadHandle", module: "_thread",
		namespace: newNamespace(),
		constructor: func(
			_ *typeValue, _ *frame, _ []Value, _ *dictValue,
		) (Value, *Exception, error) {
			return &threadHandleValue{}, nil, nil
		},
	}
	module.globals.values["_local"] = newBootstrapClass("_local", nil)
	module.globals.values["error"] = runtimeErrorType
	module.globals.values["TIMEOUT_MAX"] = &floatValue{value: 1e9}
	return module
}

func isThreadPoolBootstrap(bootstrap *boundMethodValue) bool {
	thread, ok := bootstrap.self.(*instanceValue)
	if !ok {
		return false
	}
	target, found := thread.attributes.get("_target")
	if !found {
		return false
	}
	function, ok := target.(*functionValue)
	return ok && function.code.code.Name() == "_worker"
}

// runOneThreadPoolWorkItem executes one queued worker callback while updating
// the Thread state observed by the standard-library executor machinery.
func runOneThreadPoolWorkItem(caller *frame, threadValue Value) (*Exception, error) {
	thread, ok := threadValue.(*instanceValue)
	if !ok {
		return newException("RuntimeError", "thread bootstrap receiver is invalid"), nil
	}
	started, found := thread.attributes.get("_started")
	if found {
		if setter, found := directAttribute(started, "set"); found {
			if _, exception, err := callValueSynchronously(caller, setter, nil); err != nil || exception != nil {
				return exception, err
			}
		}
	}
	argumentsValue, found := thread.attributes.get("_args")
	arguments, ok := argumentsValue.(*tupleValue)
	if !found || !ok || len(arguments.elements) < 3 {
		return nil, nil
	}
	context := arguments.elements[1]
	queue := arguments.elements[2]
	getter, found := directAttribute(queue, "get_nowait")
	if !found {
		return newException("AttributeError", "work queue has no get_nowait"), nil
	}
	item, exception, err := callValueSynchronously(caller, getter, nil)
	if err != nil || exception != nil {
		return exception, err
	}
	runner, found := directAttribute(item, "run")
	if !found {
		return newException("AttributeError", "work item has no run"), nil
	}
	_, exception, err = callValueSynchronously(caller, runner, []Value{context})
	return exception, err
}

type threadHandleValue struct{ done bool }

func (*threadHandleValue) TypeName() string { return "_ThreadHandle" }
func (*threadHandleValue) Repr() string     { return "<_ThreadHandle>" }
func (*threadHandleValue) isValue()         {}
func (handle *threadHandleValue) attribute(name string) (Value, bool) {
	switch name {
	case "ident":
		return newInt64(1), true
	case "is_done":
		return nativeFunctionNamed("_ThreadHandle.is_done", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return pythonBool(handle.done), nil, nil
			}), true
	case "join", "_set_done":
		return nativeKeywordFunctionNamed("_ThreadHandle."+name, 0, -1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			}), true
	default:
		return nil, false
	}
}

type contextVarsValue struct {
	values map[*contextVarValue]Value
}

func (*contextVarsValue) TypeName() string { return "Context" }
func (*contextVarsValue) Repr() string     { return "<_contextvars.Context>" }
func (*contextVarsValue) isValue()         {}
func (context *contextVarsValue) attribute(name string) (Value, bool) {
	switch name {
	case "run":
		return nativeKeywordAwareFunctionNamed("Context.run", 1, -1,
			func(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
				previous := caller.runtime.currentContext
				caller.runtime.currentContext = context
				defer func() { caller.runtime.currentContext = previous }()
				return callValueSynchronouslyWithKeywords(
					caller, arguments[0], arguments[1:], keywords,
				)
			}), true
	case "copy":
		return nativeFunctionNamed("Context.copy", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return context.clone(), nil, nil
			}), true
	default:
		return nil, false
	}
}

func (context *contextVarsValue) clone() *contextVarsValue {
	cloned := &contextVarsValue{values: make(map[*contextVarValue]Value, len(context.values))}
	for variable, value := range context.values {
		cloned.values[variable] = value
	}
	return cloned
}

type contextVarValue struct {
	name       string
	defaultVal Value
}

func (*contextVarValue) TypeName() string { return "ContextVar" }
func (variable *contextVarValue) Repr() string {
	return "<ContextVar name='" + variable.name + "'>"
}
func (*contextVarValue) isValue() {}

// attribute exposes context-variable lookup and assignment in the active task context.
func (variable *contextVarValue) attribute(name string) (Value, bool) {
	switch name {
	case "name":
		return &stringValue{value: variable.name}, true
	case "get":
		return nativeFunctionNamed("ContextVar.get", 0, 1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				if value, found := caller.runtime.context().values[variable]; found {
					return value, nil, nil
				}
				if len(arguments) == 1 {
					return arguments[0], nil, nil
				}
				if variable.defaultVal != nil {
					return variable.defaultVal, nil, nil
				}
				return nil, newException("LookupError", variable.name), nil
			}), true
	case "set":
		return nativeFunctionNamed("ContextVar.set", 1, 1,
			func(caller *frame, arguments []Value) (Value, *Exception, error) {
				caller.runtime.context().values[variable] = arguments[0]
				return &contextTokenValue{}, nil, nil
			}), true
	default:
		return nil, false
	}
}

type contextTokenValue struct{}

func (*contextTokenValue) TypeName() string { return "Token" }
func (*contextTokenValue) Repr() string     { return "<Token>" }
func (*contextTokenValue) isValue()         {}

type lockValue struct {
	locked bool
}

func (*lockValue) TypeName() string { return "lock" }
func (*lockValue) Repr() string     { return "<unlocked lock>" }
func (*lockValue) isValue()         {}

// attribute exposes the lock state transitions, context-manager hooks, and
// fork reinitialization hook expected by threading.
func (lock *lockValue) attribute(name string) (Value, bool) {
	switch name {
	case "acquire":
		return nativeKeywordFunctionNamed("lock.acquire", 0, 2,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if lock.locked {
					return falseSingleton, nil, nil
				}
				lock.locked = true
				return trueSingleton, nil, nil
			}), true
	case "release":
		return nativeFunctionNamed("lock.release", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if !lock.locked {
					return nil, newException("RuntimeError", "release unlocked lock"), nil
				}
				lock.locked = false
				return None, nil, nil
			}), true
	case "locked":
		return nativeFunctionNamed("lock.locked", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return pythonBool(lock.locked), nil, nil
			}), true
	case "_at_fork_reinit":
		return nativeFunctionNamed("lock._at_fork_reinit", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			}), true
	case "__enter__":
		return nativeFunctionNamed("lock.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				lock.locked = true
				return trueSingleton, nil, nil
			}), true
	case "__exit__":
		return nativeFunctionNamed("lock.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				lock.locked = false
				return falseSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

var _ Value = (*lockValue)(nil)

// newTypesBootstrapModule publishes concrete runtime type markers used by inspect.
func newTypesBootstrapModule() *Module {
	module := newSystemModule("_types", "")
	simpleNamespace := newBootstrapClass("SimpleNamespace", nil)
	simpleNamespace.module = "types"
	simpleNamespace.constructor = func(
		class *typeValue,
		_ *frame,
		arguments []Value,
		keywords *dictValue,
	) (Value, *Exception, error) {
		if len(arguments) != 0 {
			return nil, newException("TypeError", "no positional arguments expected"), nil
		}
		instance := &instanceValue{class: class, attributes: newNamespace()}
		if keywords != nil {
			for _, entry := range keywords.entries {
				name, ok := entry.key.(*stringValue)
				if !ok {
					return nil, newException("TypeError", "keywords must be strings"), nil
				}
				instance.attributes.values[name.value] = entry.value
			}
		}
		return instance, nil, nil
	}
	types := map[string]Value{
		"FunctionType":       opaqueBuiltinTypes.function,
		"LambdaType":         opaqueBuiltinTypes.function,
		"GeneratorType":      opaqueBuiltinTypes.generator,
		"CoroutineType":      opaqueBuiltinTypes.coroutine,
		"AsyncGeneratorType": opaqueBuiltinTypes.asyncGen,
		"GenericAlias":       genericAliasType,
		"EllipsisType":       opaqueBuiltinTypes.ellipsis,
		"NoneType":           builtinTypeNamed("NoneType"),
		"CodeType": &builtinTypeValue{
			name: "code",
			matches: func(value Value) bool {
				_, ok := value.(*codeValue)
				return ok
			},
		},
		"MappingProxyType":          mappingProxyType,
		"SimpleNamespace":           simpleNamespace,
		"CellType":                  cellType,
		"MethodType":                opaqueBuiltinTypes.method,
		"BuiltinFunctionType":       opaqueType("builtin_function_or_method"),
		"BuiltinMethodType":         opaqueType("builtin_function_or_method"),
		"WrapperDescriptorType":     opaqueType("wrapper_descriptor"),
		"MethodWrapperType":         opaqueType("method-wrapper"),
		"MethodDescriptorType":      opaqueType("method_descriptor"),
		"ClassMethodDescriptorType": opaqueType("classmethod_descriptor"),
		"ModuleType": &builtinTypeValue{
			name: "module",
			matches: func(value Value) bool {
				_, ok := value.(*Module)
				return ok
			},
		},
		"TracebackType":        opaqueType("traceback"),
		"FrameType":            opaqueType("frame"),
		"GetSetDescriptorType": opaqueBuiltinTypes.getSet,
		"MemberDescriptorType": opaqueBuiltinTypes.member,
		"UnionType":            unionType,
		"NotImplementedType":   opaqueType("NotImplementedType"),
		"CapsuleType":          opaqueType("PyCapsule"),
	}
	for name, value := range types {
		module.globals.values[name] = value
	}
	return module
}

func opaqueType(name string) *builtinTypeValue {
	return &builtinTypeValue{name: name, matches: func(Value) bool { return false }}
}

func newEnumModule() *Module {
	module := newSystemModule("enum", "")
	enum := newBootstrapClass("Enum", nil)
	intEnum := newBootstrapClass("IntEnum", enum)
	strEnum := newBootstrapClass("StrEnum", enum)
	flag := newBootstrapClass("Flag", enum)
	intFlag := newBootstrapClass("IntFlag", flag)
	enum.enumKind = "enum"
	intEnum.enumKind = "int"
	strEnum.enumKind = "str"
	flag.enumKind = "enum"
	intFlag.enumKind = "int"
	convert := &descriptorValue{
		class:      builtinNamedDescriptor("classmethod"),
		kind:       classMethodDescriptor,
		callable:   nativeFunctionNamed("IntEnum._convert_", 4, 4, enumConvert),
		attributes: newNamespace(),
	}
	intEnum.namespace.values["_convert_"] = convert
	module.globals.values["Enum"] = enum
	module.globals.values["IntEnum"] = intEnum
	module.globals.values["StrEnum"] = strEnum
	module.globals.values["Flag"] = flag
	module.globals.values["IntFlag"] = intFlag
	module.globals.values["EnumType"] = newBootstrapClass("EnumType", nil)
	module.globals.values["EnumMeta"] = module.globals.values["EnumType"]
	module.globals.values["property"] = builtinNamedDescriptor("property")
	for _, name := range []string{"STRICT", "CONFORM", "EJECT", "KEEP"} {
		module.globals.values[name] = &stringValue{value: strings.ToLower(name)}
	}
	counter := int64(0)
	setNativeFunction(module, "auto", 0, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			if len(arguments) == 1 {
				return arguments[0], nil, nil
			}
			counter++
			return newInt64(counter), nil, nil
		})
	setNativeFunction(module, "global_enum", 1, 2, identityFirstArgument)
	setNativeFunction(module, "unique", 1, 1, identityFirstArgument)
	setNativeKeywordFunction(module, "_simple_enum", 1, 1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return nativeFunctionNamed("enum._simple_enum.decorator", 1, 1, identityFirstArgument), nil, nil
		})
	return module
}

func enumConvert(caller *frame, arguments []Value) (Value, *Exception, error) {
	name, nameOK := arguments[1].(*stringValue)
	moduleName, moduleOK := arguments[2].(*stringValue)
	if !nameOK || !moduleOK {
		return nil, newException("TypeError", "enum conversion names must be strings"), nil
	}
	module, found := caller.runtime.modules[moduleName.value]
	if !found {
		return nil, newException("KeyError", moduleName.Repr()), nil
	}
	base, _ := arguments[0].(*typeValue)
	class := newBootstrapClass(name.value, base)
	class.module = moduleName.value
	class.constructor = func(
		_ *typeValue,
		_ *frame,
		values []Value,
		_ *dictValue,
	) (Value, *Exception, error) {
		if len(values) != 1 {
			return nil, newException("TypeError", name.value+"() takes one argument"), nil
		}
		return values[0], nil, nil
	}
	module.globals.values[name.value] = class
	return None, nil, nil
}

// initializeEnumSubclass supplies the member transformation normally performed
// by EnumType for the lightweight bootstrap enum module.
func initializeEnumSubclass(class *typeValue) {
	kind := ""
	for _, base := range class.bases {
		for _, candidate := range base.methodResolutionOrder() {
			if candidate.enumKind != "" {
				kind = candidate.enumKind
				break
			}
		}
		if kind != "" {
			break
		}
	}
	if kind == "" {
		return
	}
	class.enumKind = kind
	members := &dictValue{}
	memberValues := make([]*instanceValue, 0)
	for _, name := range class.namespace.order {
		if strings.HasPrefix(name, "_") {
			continue
		}
		declaration, found := class.namespace.get(name)
		if !found {
			continue
		}
		switch declaration.(type) {
		case *functionValue, *nativeFunctionValue, *descriptorValue, *typeValue:
			continue
		}
		underlying := declaration
		if kind == "int" {
			if _, integer := integerOperand(declaration); !integer {
				underlying = newInt64(int64(len(memberValues)))
			}
		}
		member := &instanceValue{class: class, attributes: newNamespace()}
		member.attributes.values["_name_"] = &stringValue{value: name}
		member.attributes.values["name"] = &stringValue{value: name}
		member.attributes.values["_value_"] = underlying
		member.attributes.values["value"] = underlying
		member.attributes.values["description"] = declaration
		class.namespace.values[name] = member
		_ = members.set(&stringValue{value: name}, member)
		memberValues = append(memberValues, member)
	}
	class.namespace.values["__members__"] = members
	class.constructor = func(
		_ *typeValue,
		_ *frame,
		arguments []Value,
		_ *dictValue,
	) (Value, *Exception, error) {
		if len(arguments) != 1 {
			return nil, newException("TypeError", class.name+"() takes one argument"), nil
		}
		for _, member := range memberValues {
			if arguments[0] == member || valuesEqual(arguments[0], member.attributes.values["_value_"]) {
				return member, nil, nil
			}
			left, leftInteger := integerOperand(arguments[0])
			right, rightInteger := integerOperand(member)
			if leftInteger && rightInteger && left.Cmp(&right) == 0 {
				return member, nil, nil
			}
		}
		return nil, newException("ValueError", arguments[0].Repr()+" is not a valid "+class.name), nil
	}
}

func newBootstrapClass(name string, base *typeValue) *typeValue {
	class := &typeValue{
		name:          name,
		qualifiedName: name,
		module:        "enum",
		namespace:     newNamespace(),
	}
	if base != nil {
		class.bases = []*typeValue{base}
	}
	class.mro, _ = calculateMRO(class, class.bases)
	return class
}

func builtinNamedDescriptor(name string) *typeValue {
	for _, descriptor := range descriptorTypes {
		if descriptor.name == name {
			return descriptor
		}
	}
	panic("missing descriptor type: " + name)
}

func identityFirstArgument(_ *frame, arguments []Value) (Value, *Exception, error) {
	return arguments[0], nil, nil
}

// newWarningsModule creates isolated warning capture, filter, and registry state.
func (runtimeState *Runtime) newWarningsModule() *Module {
	module := newSystemModule("warnings", "")
	action := "default"
	if sys := runtimeState.modules["sys"]; sys != nil {
		if options, found := sys.globals.get("warnoptions"); found {
			if values, ok := options.(*listValue); ok && len(values.elements) != 0 {
				if option, ok := values.elements[0].(*stringValue); ok {
					switch {
					case strings.HasPrefix(option.value, "i"):
						action = "ignore"
					case strings.HasPrefix(option.value, "a"):
						action = "always"
					}
				}
			}
		}
	}
	state := &warningsState{action: action, seen: make(map[string]struct{})}
	runtimeState.warningState = state
	setNativeKeywordFunction(module, "simplefilter", 1, 2, state.simpleFilter)
	setNativeKeywordFunction(module, "filterwarnings", 1, 5, state.filterWarnings)
	setNativeFunction(module, "resetwarnings", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			state.filters = nil
			state.seen = make(map[string]struct{})
			return None, nil, nil
		})
	for _, name := range []string{"warn", "warn_explicit"} {
		module.globals.values[name] = nativeKeywordAwareFunctionNamed(
			"warnings."+name, 1, -1, state.warn,
		)
	}
	module.globals.values["catch_warnings"] = nativeKeywordAwareFunctionNamed(
		"warnings.catch_warnings", 0, -1, state.catchWarnings,
	)
	setNativeKeywordFunction(module, "deprecated", 0, -1,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return nativeFunctionNamed("warnings.deprecated.decorator", 1, 1, identityFirstArgument), nil, nil
		})
	module.globals.values["filters"] = &listValue{}
	module.globals.values["onceregistry"] = &dictValue{}
	module.globals.values["defaultaction"] = &stringValue{value: "default"}
	warningMessage := newBootstrapClass("WarningMessage", nil)
	details := stringList([]string{
		"message", "category", "filename", "lineno", "file", "line", "source",
	})
	warningMessage.namespace.values["_WARNING_DETAILS"] = &tupleValue{elements: details.elements}
	module.globals.values["WarningMessage"] = warningMessage
	return module
}

type warningsState struct {
	active  *listValue
	action  string
	seen    map[string]struct{}
	filters []warningFilter
}

type warningFilter struct {
	action   string
	category Value
}

func (state *warningsState) simpleFilter(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	action, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "action must be a string"), nil
	}
	category := Value(warningType)
	if len(arguments) == 2 {
		category = arguments[1]
	}
	state.filters = append([]warningFilter{{action: action.value, category: category}}, state.filters...)
	state.seen = make(map[string]struct{})
	return None, nil, nil
}

func (state *warningsState) filterWarnings(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	action, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "action must be a string"), nil
	}
	category := Value(warningType)
	if len(arguments) >= 3 {
		category = arguments[2]
	}
	state.filters = append([]warningFilter{{action: action.value, category: category}}, state.filters...)
	state.seen = make(map[string]struct{})
	return None, nil, nil
}

// warn records a warning according to the active action and source location.
func (state *warningsState) warn(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	category := Value(userWarningType)
	warning := (*Exception)(nil)
	if len(arguments) >= 2 {
		if arguments[1] != None {
			category = arguments[1]
		}
	}
	message := exceptionMessage(arguments[:1])
	switch selected := category.(type) {
	case *exceptionTypeValue:
		warning = newExceptionOfType(selected, message)
	case *typeValue:
		if !selected.isExceptionClass() || !selected.builtinExceptionBase().isSubclassOf(warningType) {
			return nil, newException("TypeError", "category must be a Warning subclass"), nil
		}
		warning = newUserException(selected, message)
	default:
		return nil, newException("TypeError", "category must be a Warning subclass"), nil
	}
	stackLevel := int64(1)
	if len(arguments) >= 3 {
		if value, ok := integerOperand(arguments[2]); ok && value.IsInt64() {
			stackLevel = value.Int64()
		}
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if name != nil && ok && name.value == "category" {
				category = entry.value
				switch selected := category.(type) {
				case *exceptionTypeValue:
					warning = newExceptionOfType(selected, message)
				case *typeValue:
					if !selected.isExceptionClass() || !selected.builtinExceptionBase().isSubclassOf(warningType) {
						return nil, newException("TypeError", "category must be a Warning subclass"), nil
					}
					warning = newUserException(selected, message)
				default:
					return nil, newException("TypeError", "category must be a Warning subclass"), nil
				}
			} else if name != nil && ok && name.value == "stacklevel" {
				if value, integer := integerOperand(entry.value); integer && value.IsInt64() {
					stackLevel = value.Int64()
				}
			}
		}
	}
	action := state.action
	for _, filter := range state.filters {
		if exceptionMatchesClass(warning, filter.category) {
			action = filter.action
			break
		}
	}
	if action == "error" {
		return nil, warning, nil
	}
	if action == "ignore" || state.active == nil {
		return None, nil, nil
	}
	origin := caller
	for current := int64(1); current < stackLevel && origin.previous != nil; current++ {
		origin = origin.previous
	}
	position := origin.position(origin.instruction - 1)
	categoryName := warning.class.name
	if warning.userClass != nil {
		categoryName = warning.userClass.name
	}
	if action != "always" {
		key := categoryName + "\x00" + message + "\x00" +
			origin.code.code.Filename() + "\x00" + fmt.Sprint(position.Start.Line)
		if _, found := state.seen[key]; found {
			return None, nil, nil
		}
		state.seen[key] = struct{}{}
	}
	state.active.elements = append(state.active.elements, &warningMessageValue{
		message: warning, category: category,
		filename: origin.code.code.Filename(), line: int64(position.Start.Line),
	})
	return None, nil, nil
}

func (state *warningsState) catchWarnings(
	_ *frame,
	_ []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	record := false
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if ok && name.value == "record" {
				record = truthValue(entry.value)
			}
		}
	}
	return &warningContextValue{state: state, record: record}, nil, nil
}

type warningContextValue struct {
	state           *warningsState
	previous        *listValue
	previousAction  string
	previousSeen    map[string]struct{}
	previousFilters []warningFilter
	record          bool
}

func (*warningContextValue) TypeName() string { return "catch_warnings" }
func (*warningContextValue) Repr() string     { return "catch_warnings()" }
func (*warningContextValue) isValue()         {}
func (context *warningContextValue) attribute(name string) (Value, bool) {
	switch name {
	case "__enter__":
		return nativeFunctionNamed("catch_warnings.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				context.previous = context.state.active
				context.previousAction = context.state.action
				context.previousSeen = context.state.seen
				context.previousFilters = append([]warningFilter(nil), context.state.filters...)
				context.state.seen = make(map[string]struct{})
				if context.record {
					context.state.active = &listValue{}
					return context.state.active, nil, nil
				}
				return None, nil, nil
			}), true
	case "__exit__":
		return nativeFunctionNamed("catch_warnings.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				context.state.active = context.previous
				context.state.action = context.previousAction
				context.state.seen = context.previousSeen
				context.state.filters = context.previousFilters
				return falseSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

type warningMessageValue struct {
	message  *Exception
	category Value
	filename string
	line     int64
}

func (*warningMessageValue) TypeName() string { return "WarningMessage" }
func (*warningMessageValue) Repr() string     { return "<warnings.WarningMessage>" }
func (*warningMessageValue) isValue()         {}

// attribute exposes the warning payload and the placeholder source location
// fields consumed by the pure-Python warnings helpers.
func (message *warningMessageValue) attribute(name string) (Value, bool) {
	switch name {
	case "message":
		return message.message, true
	case "category":
		return message.category, true
	case "filename":
		return &stringValue{value: message.filename}, true
	case "lineno":
		return newInt64(message.line), true
	case "file", "line", "source":
		return None, true
	default:
		return nil, false
	}
}

var _ Value = (*warningContextValue)(nil)

func newCodecsModule() *Module {
	module := newSystemModule("_codecs", "")
	for _, name := range []string{
		"register", "unregister", "register_error",
	} {
		setNativeFunction(module, name, 1, 1,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			})
	}
	for _, name := range []string{"lookup", "lookup_error"} {
		setNativeFunction(module, name, 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				return &codecValue{name: arguments[0]}, nil, nil
			})
	}
	setNativeFunction(module, "encode", 1, 3, codecEncode)
	setNativeFunction(module, "decode", 1, 3, codecDecode)
	return module
}

type codecValue struct {
	name Value
}

func (*codecValue) TypeName() string { return "CodecInfo" }
func (*codecValue) Repr() string     { return "<codecs.CodecInfo object>" }
func (*codecValue) isValue()         {}

func codecEncode(_ *frame, arguments []Value) (Value, *Exception, error) {
	if text, ok := arguments[0].(*stringValue); ok {
		return &bytesValue{value: text.value}, nil, nil
	}
	return arguments[0], nil, nil
}

func codecDecode(_ *frame, arguments []Value) (Value, *Exception, error) {
	if data, ok := arguments[0].(*bytesValue); ok {
		return &stringValue{value: data.value}, nil, nil
	}
	return arguments[0], nil, nil
}

var _ Value = (*codecValue)(nil)

func newFunctoolsModule() *Module {
	module := newSystemModule("_functools", "")
	module.globals.values["partial"] = partialType
	setNativeKeywordFunction(module, "_lru_cache_wrapper", 1, -1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			return arguments[0], nil, nil
		})
	setNativeFunction(module, "reduce", 2, 3, functoolsReduce)
	placeholder := &objectValue{}
	module.globals.values["Placeholder"] = placeholder
	module.globals.values["_PlaceholderType"] = builtinTypeNamed("object")
	return module
}

var partialType = &builtinTypeValue{
	name: "partial",
	matches: func(value Value) bool {
		_, ok := value.(*partialValue)
		return ok
	},
}

type partialValue struct {
	callable  Value
	arguments []Value
	keywords  *dictValue
}

func (*partialValue) TypeName() string { return "partial" }
func (*partialValue) Repr() string     { return "functools.partial(...)" }
func (*partialValue) isValue()         {}
func (partial *partialValue) attribute(name string) (Value, bool) {
	switch name {
	case "func":
		return partial.callable, true
	case "args":
		return &tupleValue{elements: append([]Value(nil), partial.arguments...)}, true
	case "keywords":
		if partial.keywords == nil {
			return &dictValue{}, true
		}
		return partial.keywords, true
	default:
		return nil, false
	}
}

// functoolsReduce folds an iterable with a supported binary callable and optional initial value.
func functoolsReduce(caller *frame, arguments []Value) (Value, *Exception, error) {
	iterator, ok := newIterator(arguments[1])
	if !ok {
		return nil, newException("TypeError", "reduce argument is not iterable"), nil
	}
	var accumulator Value
	if len(arguments) == 3 {
		accumulator = arguments[2]
	} else {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return nil, newException("TypeError", "reduce() of empty iterable"), nil
		}
		accumulator = value
	}
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return accumulator, nil, nil
		}
		accumulator, exception, err = directBuiltinCall(
			caller, arguments[0], []Value{accumulator, value},
		)
		if err != nil || exception != nil {
			return nil, exception, err
		}
	}
}

var _ valueIterator = (*repeatIterator)(nil)
var _ valueIterator = (*countIterator)(nil)
