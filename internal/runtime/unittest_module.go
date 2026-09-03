package runtime

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

type unittestResultValue struct {
	testsRun int
}

type unittestMainValue struct {
	testCase *typeValue
}

func (*unittestMainValue) TypeName() string { return "builtin_function_or_method" }
func (*unittestMainValue) Repr() string     { return "<built-in function unittest.main>" }
func (*unittestMainValue) isValue()         {}

func (*unittestResultValue) TypeName() string { return "TestResult" }
func (result *unittestResultValue) Repr() string {
	return fmt.Sprintf("<unittest result run=%d>", result.testsRun)
}
func (*unittestResultValue) isValue() {}

func (result *unittestResultValue) attribute(name string) (Value, bool) {
	switch name {
	case "testsRun":
		return newInt64(int64(result.testsRun)), true
	case "wasSuccessful":
		return nativeFunctionNamed("unittest.TestResult.wasSuccessful", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return trueSingleton, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (*Runtime) newUnittestModule() *Module {
	module := newSystemModule("unittest", "")
	namespace := newNamespace()
	testCase := &typeValue{
		name:          "TestCase",
		qualifiedName: "TestCase",
		module:        "unittest",
		namespace:     namespace,
	}
	setTestCaseAssertions(namespace)
	module.globals.values["TestCase"] = testCase
	module.globals.values["__all__"] = stringList([]string{"TestCase", "main"})
	module.globals.values["main"] = &unittestMainValue{testCase: testCase}
	return module
}

func setTestCaseAssertions(namespace *Namespace) {
	setNativeMethod(namespace, "assertEqual", 3, 4, unittestAssertEqual)
	setNativeMethod(namespace, "assertNotEqual", 3, 4, unittestAssertNotEqual)
	setNativeMethod(namespace, "assertAlmostEqual", 3, 6, unittestAssertAlmostEqual)
	setNativeMethod(namespace, "assertTrue", 2, 3, unittestAssertTrue)
	setNativeMethod(namespace, "assertFalse", 2, 3, unittestAssertFalse)
	setNativeMethod(namespace, "assertIs", 3, 4, unittestAssertIs)
	setNativeMethod(namespace, "assertIsNot", 3, 4, unittestAssertIsNot)
	setNativeMethod(namespace, "assertIsNone", 2, 3, unittestAssertIsNone)
	setNativeMethod(namespace, "assertIsNotNone", 2, 3, unittestAssertIsNotNone)
	setNativeMethod(namespace, "assertIn", 3, 4, unittestAssertIn)
	setNativeMethod(namespace, "assertNotIn", 3, 4, unittestAssertNotIn)
	setNativeMethod(namespace, "assertIsInstance", 3, 4, unittestAssertIsInstance)
	setNativeMethod(namespace, "assertNotIsInstance", 3, 4, unittestAssertNotIsInstance)
	setNativeMethod(namespace, "fail", 1, 2, unittestFail)
}

func setNativeMethod(
	namespace *Namespace,
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) {
	namespace.values[name] = nativeFunctionNamed(
		"unittest.TestCase."+name,
		minimum,
		maximum,
		function,
	)
}

func executeUnittestMainCall(
	caller *frame,
	instruction int,
	base int,
	main *unittestMainValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) != 0 || (keywords != nil && len(keywords.entries) != 0) {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "unittest.main() takes no arguments"),
		}, nil
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	run := &unittestRunState{
		tests:       discoverUnittestMethods(caller.globals, main.testCase),
		instruction: instruction,
	}
	return resumeUnittestRun(caller, instruction, run)
}

type unittestMethod struct {
	class    *typeValue
	name     string
	function *functionValue
}

type unittestRunState struct {
	tests       []unittestMethod
	instruction int
	index       int
	phase       unittestPhase
	instance    *instanceValue
}

type unittestPhase uint8

const (
	unittestSetUp unittestPhase = iota
	unittestTest
	unittestTearDown
)

// discoverUnittestMethods finds direct test methods on every TestCase subclass
// in the calling module and returns a stable class-and-method ordering.
func discoverUnittestMethods(globals *Namespace, testCase *typeValue) []unittestMethod {
	var tests []unittestMethod
	for _, value := range globals.values {
		class, ok := value.(*typeValue)
		if !ok || class == testCase || !class.isSubclassOf(testCase) {
			continue
		}
		for name, value := range class.namespace.values {
			function, ok := value.(*functionValue)
			if ok && strings.HasPrefix(name, "test") {
				tests = append(tests, unittestMethod{class: class, name: name, function: function})
			}
		}
	}
	sort.Slice(tests, func(left, right int) bool {
		leftName := tests[left].class.qualifiedName + "." + tests[left].name
		rightName := tests[right].class.qualifiedName + "." + tests[right].name
		return leftName < rightName
	})
	return tests
}

// resumeUnittestRun advances one fixture/test/fixture state machine, creating
// at most one child frame or completing the result in the suspended caller.
func resumeUnittestRun(
	caller *frame,
	instruction int,
	run *unittestRunState,
) (instructionOutcome, error) {
	for run.index < len(run.tests) {
		test := run.tests[run.index]
		if run.instance == nil {
			run.instance = &instanceValue{class: test.class, attributes: newNamespace()}
		}
		var function *functionValue
		clearInstance := false
		switch run.phase {
		case unittestSetUp:
			run.phase = unittestTest
			function = unittestFixture(test.class, "setUp")
		case unittestTest:
			run.phase = unittestTearDown
			function = test.function
		case unittestTearDown:
			run.index++
			run.phase = unittestSetUp
			clearInstance = true
			function = unittestFixture(test.class, "tearDown")
		}
		if function == nil {
			if clearInstance {
				run.instance = nil
			}
			continue
		}
		methodFrame, exception, err := newUnittestMethodFrame(caller, run, function)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if clearInstance {
			run.instance = nil
		}
		return instructionOutcome{kind: called, frame: methodFrame}, nil
	}
	if err := writeUnittestSummary(caller.runtime.host.Stderr, len(run.tests)); err != nil {
		return instructionOutcome{
			kind:      raised,
			exception: hostFailure("unittest output", err),
		}, nil
	}
	return pushOutcome(caller, instruction, &unittestResultValue{testsRun: len(run.tests)})
}

func unittestFixture(class *typeValue, name string) *functionValue {
	value, found := class.lookup(name)
	if !found {
		return nil
	}
	function, _ := value.(*functionValue)
	return function
}

func newUnittestMethodFrame(
	caller *frame,
	run *unittestRunState,
	function *functionValue,
) (*frame, *Exception, error) {
	locals, exception := bindFunctionArguments(function, []Value{run.instance}, nil)
	if exception != nil {
		return nil, exception, nil
	}
	deref, initialized := initializeDeref(function.code, locals, function.closure)
	if !initialized {
		return nil, nil, function.code.failure(
			-1,
			"test method closure does not match its free variables",
		)
	}
	return &frame{
		runtime:     caller.runtime,
		code:        function.code,
		stack:       make([]Value, 0, function.code.stackSize),
		fastLocals:  locals,
		deref:       deref,
		locals:      newNamespace(),
		globals:     function.globals,
		builtins:    caller.builtins,
		previous:    caller,
		unittestRun: run,
	}, nil, nil
}

func writeUnittestSummary(writer io.Writer, count int) error {
	if writer == nil {
		return nil
	}
	testWord := "tests"
	if count == 1 {
		testWord = "test"
	}
	_, err := fmt.Fprintf(
		writer,
		"%s\n----------------------------------------------------------------------\nRan %d %s\n\nOK\n",
		strings.Repeat(".", count),
		count,
		testWord,
	)
	return err
}

func unittestAssertEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	if valuesEqual(arguments[1], arguments[2]) {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, arguments[1].Repr()+" != "+arguments[2].Repr()), nil
}

func unittestAssertNotEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	if !valuesEqual(arguments[1], arguments[2]) {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, arguments[1].Repr()+" == "+arguments[2].Repr()), nil
}

// unittestAssertAlmostEqual handles the positional places, message, and delta
// forms supported by the native call boundary.
func unittestAssertAlmostEqual(_ *frame, arguments []Value) (Value, *Exception, error) {
	left, leftOK := numericFloat(arguments[1])
	right, rightOK := numericFloat(arguments[2])
	if !leftOK || !rightOK {
		return nil, newException("TypeError", "assertAlmostEqual arguments must be numeric"), nil
	}
	places := int64(7)
	if len(arguments) >= 4 && arguments[3] != None {
		value, ok := arguments[3].(*intValue)
		if !ok || !value.value.IsInt64() {
			return nil, newException("TypeError", "places must be an integer"), nil
		}
		places = value.value.Int64()
	}
	difference := math.Abs(left - right)
	if len(arguments) == 6 && arguments[5] != None {
		delta, ok := numericFloat(arguments[5])
		if !ok {
			return nil, newException("TypeError", "delta must be numeric"), nil
		}
		if difference <= delta {
			return None, nil, nil
		}
		message := fmt.Sprintf("%s != %s within %s delta", arguments[1].Repr(), arguments[2].Repr(), arguments[5].Repr())
		return nil, unittestFailureAt(arguments, 4, message), nil
	}
	if difference <= 0.5*math.Pow10(-int(places)) {
		return None, nil, nil
	}
	message := fmt.Sprintf("%s != %s within %d places", arguments[1].Repr(), arguments[2].Repr(), places)
	return nil, unittestFailureAt(arguments, 4, message), nil
}

func unittestAssertTrue(_ *frame, arguments []Value) (Value, *Exception, error) {
	if truthValue(arguments[1]) {
		return None, nil, nil
	}
	return nil, unittestFailureAt(arguments, 2, arguments[1].Repr()+" is not true"), nil
}

func unittestAssertFalse(_ *frame, arguments []Value) (Value, *Exception, error) {
	if !truthValue(arguments[1]) {
		return None, nil, nil
	}
	return nil, unittestFailureAt(arguments, 2, arguments[1].Repr()+" is not false"), nil
}

func unittestAssertIs(_ *frame, arguments []Value) (Value, *Exception, error) {
	if arguments[1] == arguments[2] {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, arguments[1].Repr()+" is not "+arguments[2].Repr()), nil
}

func unittestAssertIsNot(_ *frame, arguments []Value) (Value, *Exception, error) {
	if arguments[1] != arguments[2] {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, "unexpectedly identical: "+arguments[1].Repr()), nil
}

func unittestAssertIsNone(_ *frame, arguments []Value) (Value, *Exception, error) {
	if arguments[1] == None {
		return None, nil, nil
	}
	return nil, unittestFailureAt(arguments, 2, arguments[1].Repr()+" is not None"), nil
}

func unittestAssertIsNotNone(_ *frame, arguments []Value) (Value, *Exception, error) {
	if arguments[1] != None {
		return None, nil, nil
	}
	return nil, unittestFailureAt(arguments, 2, "unexpectedly None"), nil
}

func unittestAssertIn(_ *frame, arguments []Value) (Value, *Exception, error) {
	contained, exception := containsValue(arguments[2], arguments[1])
	if exception != nil {
		return nil, exception, nil
	}
	if contained {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, arguments[1].Repr()+" not found in "+arguments[2].Repr()), nil
}

func unittestAssertNotIn(_ *frame, arguments []Value) (Value, *Exception, error) {
	contained, exception := containsValue(arguments[2], arguments[1])
	if exception != nil {
		return nil, exception, nil
	}
	if !contained {
		return None, nil, nil
	}
	return nil, unittestFailure(arguments, arguments[1].Repr()+" unexpectedly found in "+arguments[2].Repr()), nil
}

func unittestAssertIsInstance(_ *frame, arguments []Value) (Value, *Exception, error) {
	matches, exception := valueIsInstance(arguments[1], arguments[2])
	if exception != nil {
		return nil, exception, nil
	}
	if matches {
		return None, nil, nil
	}
	message := arguments[1].Repr() + " is not an instance of " + arguments[2].Repr()
	return nil, unittestFailure(arguments, message), nil
}

func unittestAssertNotIsInstance(_ *frame, arguments []Value) (Value, *Exception, error) {
	matches, exception := valueIsInstance(arguments[1], arguments[2])
	if exception != nil {
		return nil, exception, nil
	}
	if !matches {
		return None, nil, nil
	}
	message := arguments[1].Repr() + " is an instance of " + arguments[2].Repr()
	return nil, unittestFailure(arguments, message), nil
}

func unittestFail(_ *frame, arguments []Value) (Value, *Exception, error) {
	message := ""
	if len(arguments) == 2 {
		message = arguments[1].Repr()
		if text, ok := arguments[1].(*stringValue); ok {
			message = text.value
		}
	}
	return nil, newExceptionOfType(assertionErrorType, message), nil
}

func unittestFailure(arguments []Value, fallback string) *Exception {
	return unittestFailureAt(arguments, 3, fallback)
}

func unittestFailureAt(arguments []Value, messageIndex int, fallback string) *Exception {
	message := fallback
	if len(arguments) > messageIndex && arguments[messageIndex] != None {
		message = arguments[messageIndex].Repr()
		if text, ok := arguments[messageIndex].(*stringValue); ok {
			message = text.value
		}
	}
	return newExceptionOfType(assertionErrorType, message)
}

var _ attributeValue = (*unittestResultValue)(nil)
var _ Value = (*unittestMainValue)(nil)
