package runtime

import (
	"strings"
	"unicode"
)

// stringMethod selects a bound operation from the current string compatibility surface.
func stringMethod(value *stringValue, name string) (Value, bool) {
	if name == "splitlines" {
		return nativeKeywordAwareFunctionNamed(
			"str.splitlines", 0, 1, value.splitLinesKeyword,
		), true
	}
	var minimum, maximum int
	var function nativeFunction
	switch name {
	case "startswith":
		minimum, maximum, function = 1, 3, value.startsWith
	case "endswith":
		minimum, maximum, function = 1, 3, value.endsWith
	case "lower":
		minimum, maximum, function = 0, 0, value.lower
	case "casefold":
		minimum, maximum, function = 0, 0, value.lower
	case "upper":
		minimum, maximum, function = 0, 0, value.upper
	case "capitalize":
		minimum, maximum, function = 0, 0, value.capitalize
	case "strip":
		minimum, maximum, function = 0, 1, value.strip
	case "lstrip":
		minimum, maximum, function = 0, 1, value.leftStrip
	case "rstrip":
		minimum, maximum, function = 0, 1, value.rightStrip
	case "split":
		minimum, maximum, function = 0, 2, value.split
	case "rsplit":
		minimum, maximum, function = 0, 2, value.rsplit
	case "count":
		minimum, maximum, function = 1, 3, value.count
	case "find":
		minimum, maximum, function = 1, 3, value.find
	case "rfind":
		minimum, maximum, function = 1, 3, value.rfind
	case "replace":
		minimum, maximum, function = 2, 3, value.replace
	case "removeprefix":
		minimum, maximum, function = 1, 1, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			prefix, ok := arguments[0].(*stringValue)
			if !ok {
				return nil, newException("TypeError", "removeprefix() argument must be str"), nil
			}
			return &stringValue{value: strings.TrimPrefix(value.value, prefix.value)}, nil, nil
		}
	case "removesuffix":
		minimum, maximum, function = 1, 1, func(_ *frame, arguments []Value) (Value, *Exception, error) {
			suffix, ok := arguments[0].(*stringValue)
			if !ok {
				return nil, newException("TypeError", "removesuffix() argument must be str"), nil
			}
			return &stringValue{value: strings.TrimSuffix(value.value, suffix.value)}, nil, nil
		}
	case "join":
		minimum, maximum, function = 1, 1, value.join
	case "partition":
		minimum, maximum, function = 1, 1, value.partition
	case "isidentifier":
		minimum, maximum, function = 0, 0, value.isIdentifier
	case "isspace":
		minimum, maximum, function = 0, 0, value.isSpace
	case "isupper":
		minimum, maximum, function = 0, 0, value.isUpper
	case "isalnum":
		minimum, maximum, function = 0, 0, value.isAlnum
	case "isascii":
		minimum, maximum, function = 0, 0, value.isASCII
	case "format":
		minimum, maximum, function = 0, -1, value.format
	default:
		return nil, false
	}
	return nativeFunctionNamed("str."+name, minimum, maximum, function), true
}

func (value *stringValue) startsWith(_ *frame, arguments []Value) (Value, *Exception, error) {
	selected, exception := value.stringWindow(arguments[1:])
	if exception != nil {
		return nil, exception, nil
	}
	return matchStringPrefix(selected, arguments[0], true)
}

func (value *stringValue) endsWith(_ *frame, arguments []Value) (Value, *Exception, error) {
	selected, exception := value.stringWindow(arguments[1:])
	if exception != nil {
		return nil, exception, nil
	}
	return matchStringPrefix(selected, arguments[0], false)
}

// stringWindow normalizes optional start and end indexes for prefix checks.
func (value *stringValue) stringWindow(arguments []Value) (string, *Exception) {
	runes := []rune(value.value)
	start, end := int64(0), int64(len(runes))
	if len(arguments) >= 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok || !integer.IsInt64() {
			return "", newException("TypeError", "slice indices must be integers")
		}
		start = integer.Int64()
	}
	if len(arguments) == 2 {
		integer, ok := integerOperand(arguments[1])
		if !ok || !integer.IsInt64() {
			return "", newException("TypeError", "slice indices must be integers")
		}
		end = integer.Int64()
	}
	start = max(0, min(start, int64(len(runes))))
	end = max(start, min(end, int64(len(runes))))
	return string(runes[start:end]), nil
}

// matchStringPrefix checks one string or tuple of strings as a prefix or suffix.
func matchStringPrefix(selected string, candidate Value, prefix bool) (Value, *Exception, error) {
	match := func(text string) bool {
		if prefix {
			return strings.HasPrefix(selected, text)
		}
		return strings.HasSuffix(selected, text)
	}
	if text, ok := candidate.(*stringValue); ok {
		return pythonBool(match(text.value)), nil, nil
	}
	if choices, ok := candidate.(*tupleValue); ok {
		for _, choice := range choices.elements {
			text, stringChoice := choice.(*stringValue)
			if !stringChoice {
				return nil, newException("TypeError", "tuple must only contain str"), nil
			}
			if match(text.value) {
				return trueSingleton, nil, nil
			}
		}
		return falseSingleton, nil, nil
	}
	return nil, newException("TypeError", "prefix must be str or a tuple of str"), nil
}

func (value *stringValue) lower(_ *frame, _ []Value) (Value, *Exception, error) {
	return &stringValue{value: strings.ToLower(value.value)}, nil, nil
}

func (value *stringValue) upper(_ *frame, _ []Value) (Value, *Exception, error) {
	return &stringValue{value: strings.ToUpper(value.value)}, nil, nil
}

func (value *stringValue) isUpper(_ *frame, _ []Value) (Value, *Exception, error) {
	hasCased := false
	for _, character := range value.value {
		if unicode.IsLower(character) {
			return falseSingleton, nil, nil
		}
		if unicode.IsUpper(character) {
			hasCased = true
		}
	}
	return pythonBool(hasCased), nil, nil
}

func (value *stringValue) isAlnum(_ *frame, _ []Value) (Value, *Exception, error) {
	if value.value == "" {
		return falseSingleton, nil, nil
	}
	for _, character := range value.value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return falseSingleton, nil, nil
		}
	}
	return trueSingleton, nil, nil
}

func (value *stringValue) isASCII(_ *frame, _ []Value) (Value, *Exception, error) {
	for _, character := range value.value {
		if character > unicode.MaxASCII {
			return falseSingleton, nil, nil
		}
	}
	return trueSingleton, nil, nil
}

func (value *stringValue) capitalize(_ *frame, _ []Value) (Value, *Exception, error) {
	runes := []rune(strings.ToLower(value.value))
	if len(runes) != 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return &stringValue{value: string(runes)}, nil, nil
}

func trimCutset(arguments []Value) (string, bool, *Exception) {
	if len(arguments) == 0 || arguments[0] == None {
		return "", true, nil
	}
	characters, ok := arguments[0].(*stringValue)
	if !ok {
		return "", false, newException("TypeError", "strip arg must be None or str")
	}
	return characters.value, false, nil
}

func (value *stringValue) strip(_ *frame, arguments []Value) (Value, *Exception, error) {
	cutset, whitespace, exception := trimCutset(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	if whitespace {
		return &stringValue{value: strings.TrimSpace(value.value)}, nil, nil
	}
	return &stringValue{value: strings.Trim(value.value, cutset)}, nil, nil
}

func (value *stringValue) leftStrip(_ *frame, arguments []Value) (Value, *Exception, error) {
	cutset, whitespace, exception := trimCutset(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	if whitespace {
		return &stringValue{value: strings.TrimLeftFunc(value.value, unicode.IsSpace)}, nil, nil
	}
	return &stringValue{value: strings.TrimLeft(value.value, cutset)}, nil, nil
}

func (value *stringValue) rightStrip(_ *frame, arguments []Value) (Value, *Exception, error) {
	cutset, whitespace, exception := trimCutset(arguments)
	if exception != nil {
		return nil, exception, nil
	}
	if whitespace {
		return &stringValue{value: strings.TrimRightFunc(value.value, unicode.IsSpace)}, nil, nil
	}
	return &stringValue{value: strings.TrimRight(value.value, cutset)}, nil, nil
}

// split separates a string using whitespace or an explicit delimiter and limit.
func (value *stringValue) split(_ *frame, arguments []Value) (Value, *Exception, error) {
	var parts []string
	if len(arguments) == 0 || arguments[0] == None {
		parts = strings.Fields(value.value)
	} else {
		separator, ok := arguments[0].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "separator must be str or None"), nil
		}
		if separator.value == "" {
			return nil, newException("ValueError", "empty separator"), nil
		}
		limit := -1
		if len(arguments) == 2 {
			integer, integerOK := integerOperand(arguments[1])
			if !integerOK || !integer.IsInt64() {
				return nil, newException("TypeError", "maxsplit must be an integer"), nil
			}
			if integer.Sign() >= 0 {
				limit = int(integer.Int64()) + 1
			}
		}
		parts = strings.SplitN(value.value, separator.value, limit)
	}
	return stringValues(parts), nil, nil
}

// rsplit separates a string from the right while preserving the original part order.
func (value *stringValue) rsplit(_ *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) == 0 || arguments[0] == None {
		parts := strings.Fields(value.value)
		if len(arguments) != 2 {
			return stringValues(parts), nil, nil
		}
		limit, ok := integerOperand(arguments[1])
		if !ok || !limit.IsInt64() {
			return nil, newException("TypeError", "maxsplit must be an integer"), nil
		}
		if limit.Sign() >= 0 && int64(len(parts)) > limit.Int64()+1 {
			start := len(parts) - int(limit.Int64())
			parts = append([]string{strings.Join(parts[:start], " ")}, parts[start:]...)
		}
		return stringValues(parts), nil, nil
	}
	separator, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "separator must be str or None"), nil
	}
	if separator.value == "" {
		return nil, newException("ValueError", "empty separator"), nil
	}
	limit := int64(-1)
	if len(arguments) == 2 {
		integer, integerOK := integerOperand(arguments[1])
		if !integerOK || !integer.IsInt64() {
			return nil, newException("TypeError", "maxsplit must be an integer"), nil
		}
		limit = integer.Int64()
	}
	if limit < 0 {
		return stringValues(strings.Split(value.value, separator.value)), nil, nil
	}
	parts := []string{value.value}
	for count := int64(0); count < limit; count++ {
		position := strings.LastIndex(parts[0], separator.value)
		if position < 0 {
			break
		}
		left := parts[0][:position]
		right := parts[0][position+len(separator.value):]
		parts[0] = left
		parts = append([]string{parts[0], right}, parts[1:]...)
	}
	return stringValues(parts), nil, nil
}

func (value *stringValue) find(_ *frame, arguments []Value) (Value, *Exception, error) {
	needle, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "must be str, not "+arguments[0].TypeName()), nil
	}
	selected, exception := value.stringWindow(arguments[1:])
	if exception != nil {
		return nil, exception, nil
	}
	position := strings.Index(selected, needle.value)
	if position < 0 {
		return newInt64(-1), nil, nil
	}
	prefix := []rune(value.value)
	start := len(prefix) - len([]rune(selected))
	return newInt64(int64(start + len([]rune(selected[:position])))), nil, nil
}

func (value *stringValue) rfind(_ *frame, arguments []Value) (Value, *Exception, error) {
	needle, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "must be str, not "+arguments[0].TypeName()), nil
	}
	selected, exception := value.stringWindow(arguments[1:])
	if exception != nil {
		return nil, exception, nil
	}
	position := strings.LastIndex(selected, needle.value)
	if position < 0 {
		return newInt64(-1), nil, nil
	}
	start := len([]rune(value.value)) - len([]rune(selected))
	return newInt64(int64(start + len([]rune(selected[:position])))), nil, nil
}

// splitLines separates universal line endings with optional terminator retention.
func (value *stringValue) splitLines(_ *frame, arguments []Value) (Value, *Exception, error) {
	keepEnds := len(arguments) == 1 && truthValue(arguments[0])
	return value.splitLinesWithKeepEnds(keepEnds), nil, nil
}

// splitLinesKeyword binds the positional-or-keyword keepends parameter and
// delegates universal line splitting to the shared implementation.
func (value *stringValue) splitLinesKeyword(
	_ *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	keepEnds := false
	if len(arguments) == 1 {
		keepEnds = truthValue(arguments[0])
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok || name.value != "keepends" {
				return nil, newException(
					"TypeError", "splitlines() got an unexpected keyword argument",
				), nil
			}
			if len(arguments) == 1 {
				return nil, newException(
					"TypeError", "splitlines() got multiple values for argument 'keepends'",
				), nil
			}
			keepEnds = truthValue(entry.value)
		}
	}
	return value.splitLinesWithKeepEnds(keepEnds), nil, nil
}

func (value *stringValue) splitLinesWithKeepEnds(keepEnds bool) Value {
	parts := strings.SplitAfter(value.value, "\n")
	if len(parts) != 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if !keepEnds {
		for index := range parts {
			parts[index] = strings.TrimSuffix(parts[index], "\n")
		}
	}
	return stringValues(parts)
}

// count reports non-overlapping substring occurrences in an optional rune
// window, matching the indexing rules used by other string methods.
func (value *stringValue) count(_ *frame, arguments []Value) (Value, *Exception, error) {
	substring, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "substring must be str"), nil
	}
	runes := []rune(value.value)
	start, end := int64(0), int64(len(runes))
	for index, argument := range arguments[1:] {
		integer, integerOK := integerOperand(argument)
		if !integerOK || !integer.IsInt64() {
			return nil, newException("TypeError", "slice indices must be integers"), nil
		}
		position := integer.Int64()
		if position < 0 {
			position += int64(len(runes))
		}
		position = max(0, min(position, int64(len(runes))))
		if index == 0 {
			start = position
		} else {
			end = position
		}
	}
	if end < start {
		return newInt64(0), nil, nil
	}
	return newInt64(int64(strings.Count(string(runes[start:end]), substring.value))), nil, nil
}

// replace substitutes an optional bounded number of literal occurrences.
func (value *stringValue) replace(_ *frame, arguments []Value) (Value, *Exception, error) {
	old, oldOK := arguments[0].(*stringValue)
	newValue, newOK := arguments[1].(*stringValue)
	if !oldOK || !newOK {
		return nil, newException("TypeError", "replace arguments must be str"), nil
	}
	count := -1
	if len(arguments) == 3 {
		integer, ok := integerOperand(arguments[2])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "count must be an integer"), nil
		}
		count = int(integer.Int64())
	}
	return &stringValue{value: strings.Replace(value.value, old.value, newValue.value, count)}, nil, nil
}

// join drains an iterable, validates string elements, and inserts the receiver separator.
func (value *stringValue) join(caller *frame, arguments []Value) (Value, *Exception, error) {
	iterator, ok := newIterator(arguments[0])
	if !ok {
		return nil, newException("TypeError", "can only join an iterable"), nil
	}
	parts := make([]string, 0)
	for {
		item, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			break
		}
		text, stringItem := item.(*stringValue)
		if !stringItem {
			return nil, newException("TypeError", "sequence item is not a string"), nil
		}
		parts = append(parts, text.value)
	}
	return &stringValue{value: strings.Join(parts, value.value)}, nil, nil
}

func (value *stringValue) partition(_ *frame, arguments []Value) (Value, *Exception, error) {
	separator, ok := arguments[0].(*stringValue)
	if !ok || separator.value == "" {
		return nil, newException("ValueError", "empty separator"), nil
	}
	before, after, found := strings.Cut(value.value, separator.value)
	if !found {
		return &tupleValue{elements: []Value{value, &stringValue{}, &stringValue{}}}, nil, nil
	}
	return &tupleValue{elements: []Value{
		&stringValue{value: before}, separator, &stringValue{value: after},
	}}, nil, nil
}

// isIdentifier applies Unicode identifier start and continuation rules.
func (value *stringValue) isIdentifier(_ *frame, _ []Value) (Value, *Exception, error) {
	runes := []rune(value.value)
	if len(runes) == 0 || !(runes[0] == '_' || unicode.IsLetter(runes[0])) {
		return falseSingleton, nil, nil
	}
	for _, character := range runes[1:] {
		if character != '_' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return falseSingleton, nil, nil
		}
	}
	return trueSingleton, nil, nil
}

func (value *stringValue) isSpace(_ *frame, _ []Value) (Value, *Exception, error) {
	if value.value == "" {
		return falseSingleton, nil, nil
	}
	for _, character := range value.value {
		if !unicode.IsSpace(character) {
			return falseSingleton, nil, nil
		}
	}
	return trueSingleton, nil, nil
}

// format replaces automatic positional fields and escaped braces for the
// diagnostic strings used by the standard-library test framework.
func (value *stringValue) format(caller *frame, arguments []Value) (Value, *Exception, error) {
	var builder strings.Builder
	argument := 0
	for index := 0; index < len(value.value); {
		switch {
		case strings.HasPrefix(value.value[index:], "{{"):
			builder.WriteByte('{')
			index += 2
		case strings.HasPrefix(value.value[index:], "}}"):
			builder.WriteByte('}')
			index += 2
		case value.value[index] == '{':
			end := strings.IndexByte(value.value[index:], '}')
			if end < 0 || argument >= len(arguments) {
				return nil, newException("ValueError", "invalid format string"), nil
			}
			field := value.value[index+1 : index+end]
			text := arguments[argument].Repr()
			if !strings.Contains(field, "!r") {
				stringArgument, exception, err := pythonString(caller, arguments[argument])
				if err != nil || exception != nil {
					return nil, exception, err
				}
				text = stringArgument.value
			}
			builder.WriteString(text)
			argument++
			index += end + 1
		default:
			builder.WriteByte(value.value[index])
			index++
		}
	}
	return &stringValue{value: builder.String()}, nil, nil
}

func stringValues(values []string) *listValue {
	elements := make([]Value, len(values))
	for index, value := range values {
		elements[index] = &stringValue{value: value}
	}
	return &listValue{elements: elements}
}
