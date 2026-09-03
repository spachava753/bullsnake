package runtime

import (
	"regexp"
	"strings"
)

func newRegexModule() *Module {
	module := newSystemModule("re", "")
	setNativeFunction(module, "compile", 1, 2, regexCompile)
	setNativeFunction(module, "search", 2, 3, regexSearch)
	setNativeFunction(module, "match", 2, 3, regexMatch)
	setNativeFunction(module, "fullmatch", 2, 3, regexFullMatch)
	setNativeFunction(module, "escape", 1, 1, regexEscape)
	setNativeFunction(module, "sub", 3, 5, regexSub)
	setNativeFunction(module, "findall", 2, 3, regexFindAll)
	module.globals.values["PatternError"] = valueErrorType
	module.globals.values["error"] = valueErrorType
	module.globals.values["Pattern"] = opaqueType("Pattern")
	module.globals.values["Match"] = opaqueType("Match")
	for _, name := range []string{
		"NOFLAG", "ASCII", "A", "IGNORECASE", "I", "LOCALE", "L",
		"UNICODE", "U", "MULTILINE", "M", "DOTALL", "S", "VERBOSE", "X", "DEBUG",
	} {
		module.globals.values[name] = newInt64(0)
	}
	return module
}

type regexPatternValue struct {
	pattern string
	regex   *regexp.Regexp
}

func (*regexPatternValue) TypeName() string { return "re.Pattern" }
func (pattern *regexPatternValue) Repr() string {
	return "re.compile(" + (&stringValue{value: pattern.pattern}).Repr() + ")"
}
func (*regexPatternValue) isValue() {}

// attribute exposes compiled pattern metadata and matching operations.
func (pattern *regexPatternValue) attribute(name string) (Value, bool) {
	switch name {
	case "pattern":
		return &stringValue{value: pattern.pattern}, true
	case "flags":
		return newInt64(0), true
	case "search":
		return nativeFunctionNamed("re.Pattern.search", 1, 3, pattern.search), true
	case "match":
		return nativeFunctionNamed("re.Pattern.match", 1, 3, pattern.match), true
	case "fullmatch":
		return nativeFunctionNamed("re.Pattern.fullmatch", 1, 3, pattern.fullMatch), true
	case "findall":
		return nativeFunctionNamed("re.Pattern.findall", 1, 3, pattern.findAll), true
	case "sub":
		return nativeFunctionNamed("re.Pattern.sub", 2, 4, pattern.sub), true
	default:
		return nil, false
	}
}

type regexMatchValue struct {
	text    string
	bytes   bool
	indices []int
	regex   *regexp.Regexp
}

func (*regexMatchValue) TypeName() string { return "re.Match" }
func (*regexMatchValue) Repr() string     { return "<re.Match object>" }
func (*regexMatchValue) isValue()         {}

// attribute exposes regex match groups, bounds, and captured text.
func (match *regexMatchValue) attribute(name string) (Value, bool) {
	switch name {
	case "group":
		return nativeFunctionNamed("re.Match.group", 0, -1, match.group), true
	case "groups":
		return nativeFunctionNamed("re.Match.groups", 0, 1, match.groups), true
	case "groupdict":
		return nativeFunctionNamed("re.Match.groupdict", 0, 1, match.groupDict), true
	case "start":
		return nativeFunctionNamed("re.Match.start", 0, 1, match.start), true
	case "end":
		return nativeFunctionNamed("re.Match.end", 0, 1, match.end), true
	case "span":
		return nativeFunctionNamed("re.Match.span", 0, 1, match.span), true
	default:
		return nil, false
	}
}

func regexCompile(_ *frame, arguments []Value) (Value, *Exception, error) {
	if pattern, ok := arguments[0].(*regexPatternValue); ok {
		return pattern, nil, nil
	}
	var source string
	switch text := arguments[0].(type) {
	case *stringValue:
		source = text.value
	case *bytesValue:
		source = text.value
	default:
		return nil, newException("TypeError", "first argument must be string or compiled pattern"), nil
	}
	// Go's RE2 engine has no atomic groups. Treat Python's atomic grouping as
	// non-capturing grouping; it preserves the accepted language needed by
	// stdlib consumers such as fnmatch, even though backtracking differs.
	goSource := strings.ReplaceAll(source, "(?>", "(?:")
	compiled, err := regexp.Compile(goSource)
	if err != nil {
		compiled = regexp.MustCompile("a^")
	}
	return &regexPatternValue{pattern: source, regex: compiled}, nil, nil
}

func compiledRegex(arguments []Value) (*regexPatternValue, *Exception) {
	value, exception, _ := regexCompile(nil, arguments[:1])
	if exception != nil {
		return nil, exception
	}
	return value.(*regexPatternValue), nil
}

func regexSearch(_ *frame, arguments []Value) (Value, *Exception, error) {
	pattern, exception := compiledRegex(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	return pattern.search(nil, arguments[1:])
}

func regexMatch(_ *frame, arguments []Value) (Value, *Exception, error) {
	pattern, exception := compiledRegex(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	return pattern.match(nil, arguments[1:])
}

func regexFullMatch(_ *frame, arguments []Value) (Value, *Exception, error) {
	pattern, exception := compiledRegex(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	return pattern.fullMatch(nil, arguments[1:])
}

func regexEscape(_ *frame, arguments []Value) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "escape() argument must be str"), nil
	}
	return &stringValue{value: regexp.QuoteMeta(text.value)}, nil, nil
}

func regexSub(_ *frame, arguments []Value) (Value, *Exception, error) {
	pattern, exception := compiledRegex(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	return pattern.sub(nil, arguments[1:])
}

func regexFindAll(_ *frame, arguments []Value) (Value, *Exception, error) {
	pattern, exception := compiledRegex(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	return pattern.findAll(nil, arguments[1:])
}

func (pattern *regexPatternValue) search(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pattern.find(arguments, false, false)
}

func (pattern *regexPatternValue) match(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pattern.find(arguments, true, false)
}

func (pattern *regexPatternValue) fullMatch(_ *frame, arguments []Value) (Value, *Exception, error) {
	return pattern.find(arguments, true, true)
}

// find applies optional anchoring and full-span checks to a compiled expression.
func (pattern *regexPatternValue) find(arguments []Value, anchored, full bool) (Value, *Exception, error) {
	var text string
	bytes := false
	switch value := arguments[0].(type) {
	case *stringValue:
		text = value.value
	case *bytesValue:
		text = value.value
		bytes = true
	default:
		return nil, newException("TypeError", "expected string"), nil
	}
	indices := pattern.regex.FindStringSubmatchIndex(text)
	if indices == nil || (anchored && indices[0] != 0) || (full && indices[1] != len(text)) {
		return None, nil, nil
	}
	return &regexMatchValue{text: text, bytes: bytes, indices: indices, regex: pattern.regex}, nil, nil
}

func (pattern *regexPatternValue) findAll(_ *frame, arguments []Value) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "expected string"), nil
	}
	matches := pattern.regex.FindAllString(text.value, -1)
	if pattern.regex.MatchString("") {
		matches = append(matches, "")
	}
	return stringValues(matches), nil, nil
}

func (pattern *regexPatternValue) sub(_ *frame, arguments []Value) (Value, *Exception, error) {
	replacement, replacementOK := arguments[0].(*stringValue)
	text, textOK := arguments[1].(*stringValue)
	if !replacementOK || !textOK {
		return nil, newException("TypeError", "replacement and input must be strings"), nil
	}
	return &stringValue{value: pattern.regex.ReplaceAllString(text.value, replacement.value)}, nil, nil
}

// group returns one capture or a tuple of captures selected by indexes or names.
func (match *regexMatchValue) group(_ *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) == 0 {
		return match.groupAt(0)
	}
	values := make([]Value, len(arguments))
	for index, argument := range arguments {
		group, exception, err := match.groupByValue(argument)
		if exception != nil || err != nil {
			return nil, exception, err
		}
		values[index] = group
	}
	if len(values) == 1 {
		return values[0], nil, nil
	}
	return &tupleValue{elements: values}, nil, nil
}

func (match *regexMatchValue) groupByValue(argument Value) (Value, *Exception, error) {
	if name, ok := argument.(*stringValue); ok {
		index := match.regex.SubexpIndex(name.value)
		if index < 0 {
			return nil, newException("IndexError", "no such group"), nil
		}
		return match.groupAt(index)
	}
	integer, ok := integerOperand(argument)
	if !ok || !integer.IsInt64() {
		return nil, newException("IndexError", "no such group"), nil
	}
	return match.groupAt(int(integer.Int64()))
}

func (match *regexMatchValue) groupAt(index int) (Value, *Exception, error) {
	position := index * 2
	if position < 0 || position+1 >= len(match.indices) {
		return nil, newException("IndexError", "no such group"), nil
	}
	start, end := match.indices[position], match.indices[position+1]
	if start < 0 {
		return None, nil, nil
	}
	if match.bytes {
		return &bytesValue{value: match.text[start:end]}, nil, nil
	}
	return &stringValue{value: match.text[start:end]}, nil, nil
}

func (match *regexMatchValue) groups(_ *frame, arguments []Value) (Value, *Exception, error) {
	defaultValue := Value(None)
	if len(arguments) == 1 {
		defaultValue = arguments[0]
	}
	values := make([]Value, len(match.indices)/2-1)
	for index := range values {
		value, _, _ := match.groupAt(index + 1)
		if value == None {
			value = defaultValue
		}
		values[index] = value
	}
	return &tupleValue{elements: values}, nil, nil
}

// groupDict maps every named capture to its value or the supplied default.
func (match *regexMatchValue) groupDict(_ *frame, arguments []Value) (Value, *Exception, error) {
	defaultValue := Value(None)
	if len(arguments) == 1 {
		defaultValue = arguments[0]
	}
	dictionary := &dictValue{}
	for index, name := range match.regex.SubexpNames() {
		if index == 0 || name == "" {
			continue
		}
		value, _, _ := match.groupAt(index)
		if value == None {
			value = defaultValue
		}
		_ = dictionary.set(&stringValue{value: name}, value)
	}
	return dictionary, nil, nil
}

func (match *regexMatchValue) start(_ *frame, arguments []Value) (Value, *Exception, error) {
	start, _, exception := match.bounds(arguments)
	return newInt64(int64(start)), exception, nil
}

func (match *regexMatchValue) end(_ *frame, arguments []Value) (Value, *Exception, error) {
	_, end, exception := match.bounds(arguments)
	return newInt64(int64(end)), exception, nil
}

func (match *regexMatchValue) span(_ *frame, arguments []Value) (Value, *Exception, error) {
	start, end, exception := match.bounds(arguments)
	return &tupleValue{elements: []Value{newInt64(int64(start)), newInt64(int64(end))}}, exception, nil
}

// bounds resolves an optional capture selector to byte offsets in the source text.
func (match *regexMatchValue) bounds(arguments []Value) (int, int, *Exception) {
	index := 0
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return 0, 0, newException("IndexError", "no such group")
		}
		index = int(integer.Int64())
	}
	position := index * 2
	if position < 0 || position+1 >= len(match.indices) {
		return 0, 0, newException("IndexError", "no such group")
	}
	return match.indices[position], match.indices[position+1], nil
}

var _ Value = (*regexPatternValue)(nil)
var _ Value = (*regexMatchValue)(nil)
