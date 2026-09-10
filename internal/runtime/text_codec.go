package runtime

import "strings"

// textCodec selects only implemented stateless codecs. No encoding choice reads
// a process locale, environment variable, codec file, or host registry.
func textCodec(name string) (string, *Exception) {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	switch normalized {
	case "utf-8", "utf8", "u8":
		return "utf-8", nil
	case "ascii", "us-ascii", "646":
		return "ascii", nil
	case "latin-1", "latin1", "iso-8859-1", "iso8859-1":
		return "latin-1", nil
	case "locale":
		return "", newException("PermissionError", "host locale is not configured")
	default:
		return "", newException("LookupError", "unknown encoding: "+name)
	}
}

// encodeText processes Python code points, preserving strict surrogate errors
// and applying the implemented ignore/replace handlers only on encoding failure.
func encodeText(text, codec, errors string) ([]byte, *Exception) {
	var data []byte
	index := 0
	for offset := 0; offset < len(text); index++ {
		current, width, _ := decodeStringRune(text[offset:])
		valid := current < 0xd800 || current > 0xdfff
		if codec == "ascii" {
			valid = current < 128
		}
		if codec == "latin-1" {
			valid = current < 256
		}
		if valid {
			if codec == "utf-8" {
				data = append(data, text[offset:offset+width]...)
			} else {
				data = append(data, byte(current))
			}
		} else {
			switch errors {
			case "ignore":
			case "replace":
				data = append(data, '?')
			case "strict":
				reason := "surrogates not allowed"
				if codec == "ascii" {
					reason = "ordinal not in range(128)"
				}
				if codec == "latin-1" {
					reason = "ordinal not in range(256)"
				}
				exception, _ := newUnicodeError(unicodeEncodeErrorType, []Value{&stringValue{value: codec}, &stringValue{value: text}, integerFromInt64(int64(index)), integerFromInt64(int64(index + 1)), &stringValue{value: reason}})
				return nil, exception
			default:
				return nil, newException("LookupError", "unknown error handler name '"+errors+"'")
			}
		}
		offset += width
	}
	return data, nil
}
