package runtime

import "unicode/utf8"

type textUnit struct {
	text  string
	width int
}

// decodeChunk retains incomplete UTF-8 and CR prefixes between binary reads.
// Each output code point records its source byte width for logical positions.
func (stream *textWrapper) decodeChunk(data []byte, final bool) *Exception {
	stream.input = append(stream.input, data...)
	for stream.decodeOffset < len(stream.input) {
		offset := stream.decodeOffset
		remaining := stream.input[offset:]
		current, width := rune(remaining[0]), 1
		invalid, incomplete := false, false
		if stream.codec == "utf-8" {
			incomplete = !utf8.FullRune(remaining)
			if incomplete && !final {
				break
			}
			current, width = utf8.DecodeRune(remaining)
			invalid = current == utf8.RuneError && width == 1
			if incomplete {
				width = len(remaining)
			}
		} else if stream.codec == "ascii" {
			invalid = current >= 128
		}
		if invalid {
			switch stream.errors {
			case "ignore":
				stream.skipped += width
				stream.decodeOffset += width
				continue
			case "replace":
				current = utf8.RuneError
			case "strict":
				reason := "invalid start byte"
				if stream.codec == "ascii" {
					reason = "ordinal not in range(128)"
				} else if incomplete {
					reason = "unexpected end of data"
				} else if remaining[0] >= 0xc2 && remaining[0] <= 0xf4 {
					reason = "invalid continuation byte"
				}
				exception, _ := newUnicodeError(unicodeDecodeErrorType, []Value{&stringValue{value: stream.codec}, &bytesValue{value: string(stream.input)}, integerFromInt64(int64(offset)), integerFromInt64(int64(offset + width)), &stringValue{value: reason}})
				return exception
			default:
				return newException("LookupError", "unknown error handler name '"+stream.errors+"'")
			}
		}
		stream.decodeOffset += width
		unit := textUnit{text: string(current), width: width + stream.skipped}
		stream.skipped = 0
		stream.appendDecoded(unit)
	}
	if final && stream.pendingCR != nil {
		stream.appendDecoded(textUnit{})
	}
	return nil
}

// appendDecoded keeps CRLF together and records newline kinds without losing
// source widths when universal translation combines two bytes into one LF.
func (stream *textWrapper) appendDecoded(unit textUnit) {
	if stream.pendingCR != nil {
		previous := *stream.pendingCR
		stream.pendingCR = nil
		if unit.text == "\n" {
			stream.seen |= 4
			if stream.translate {
				stream.decoded = append(stream.decoded, textUnit{text: "\n", width: previous.width + unit.width})
			} else {
				stream.decoded = append(stream.decoded, previous, unit)
			}
			return
		}
		stream.seen |= 1
		if stream.translate {
			previous.text = "\n"
		}
		stream.decoded = append(stream.decoded, previous)
	}
	if unit.text == "" {
		return
	}
	if stream.universal && unit.text == "\r" {
		stream.pendingCR = &unit
		return
	}
	if stream.universal && unit.text == "\n" {
		stream.seen |= 2
	}
	stream.decoded = append(stream.decoded, unit)
}
