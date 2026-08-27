package lexer

import "fmt"

// scanNumber distinguishes leading-dot, base-prefixed, and decimal forms
// before delegating completion and boundary checks.
func (lexer *Lexer) scanNumber() (Token, error) {
	start := lexer.offset
	if lexer.source[start] == '.' {
		end, bad := lexer.scanDecimalDigits(start + 1)
		if bad >= 0 {
			return lexer.invalidNumber(start, bad, "decimal")
		}
		return lexer.finishDecimalNumber(start, end, true, false)
	}

	if lexer.source[start] == '0' && start+1 < len(lexer.source) {
		switch lexer.source[start+1] {
		case 'x', 'X':
			return lexer.scanBasedNumber(start, start+2, "hexadecimal", isASCIIHexDigit)
		case 'o', 'O':
			return lexer.scanBasedNumber(start, start+2, "octal", isASCIIOctalDigit)
		case 'b', 'B':
			return lexer.scanBasedNumber(start, start+2, "binary", isASCIIBinaryDigit)
		}
	}

	end, bad := lexer.scanDecimalDigits(start)
	if bad >= 0 {
		return lexer.invalidNumber(start, bad, "decimal")
	}
	integerEnd := end
	leadingNonzero := lexer.source[start] == '0' && hasNonzeroDigit(lexer.source[start:integerEnd])
	return lexer.finishDecimalNumber(start, end, false, leadingNonzero)
}

// scanBasedNumber validates digits and underscore groups for a binary, octal,
// or hexadecimal literal before checking its token boundary.
func (lexer *Lexer) scanBasedNumber(start, offset int, name string, validDigit func(byte) bool) (Token, error) {
	if lexer.peekByte(offset) == '_' {
		offset++
	}
	if lexer.peekByte(offset) == 0 {
		return lexer.invalidNumber(start, offset, name)
	}
	if !validDigit(lexer.peekByte(offset)) {
		if isASCIIDigit(lexer.peekByte(offset)) {
			return lexer.invalidBaseDigit(start, offset, name)
		}
		return lexer.invalidNumber(start, offset, name)
	}

	offset, bad := lexer.scanDigitGroups(offset, validDigit)
	if bad >= 0 {
		if isASCIIDigit(lexer.peekByte(offset)) {
			return lexer.invalidBaseDigit(start, offset, name)
		}
		return lexer.invalidNumber(start, bad, name)
	}
	if isASCIIDigit(lexer.peekByte(offset)) {
		return lexer.invalidBaseDigit(start, offset, name)
	}
	if errToken, err := lexer.verifyNumberEnd(start, offset, name); err != nil {
		return errToken, err
	}
	lexer.offset = offset
	lexer.lineHasCode = true
	return lexer.token(Number, start, offset), nil
}

// finishDecimalNumber scans optional fraction, exponent, and imaginary parts
// before validating leading zeros and the token boundary.
func (lexer *Lexer) finishDecimalNumber(start, offset int, hasFraction, leadingNonzero bool) (Token, error) {
	if !hasFraction && lexer.peekByte(offset) == '.' {
		hasFraction = true
		offset++
		if isASCIIDigit(lexer.peekByte(offset)) {
			var bad int
			offset, bad = lexer.scanDecimalDigits(offset)
			if bad >= 0 {
				return lexer.invalidNumber(start, bad, "decimal")
			}
		}
	}

	hasExponent := false
	char := lexer.peekByte(offset)
	if (char == 'e' || char == 'E') && !lexer.keywordAt(offset, "else") {
		hasExponent = true
		exponent := offset
		offset++
		char = lexer.peekByte(offset)
		if char == '+' || char == '-' {
			offset++
		}
		if !isASCIIDigit(lexer.peekByte(offset)) {
			return lexer.invalidNumber(start, exponent, "decimal")
		}
		var bad int
		offset, bad = lexer.scanDecimalDigits(offset)
		if bad >= 0 {
			return lexer.invalidNumber(start, bad, "decimal")
		}
	}

	imaginary := false
	if char = lexer.peekByte(offset); char == 'j' || char == 'J' {
		imaginary = true
		offset++
	}
	if leadingNonzero && !hasFraction && !hasExponent && !imaginary {
		span := lexer.span(start, offset)
		message := "leading zeros in decimal integer literals are not permitted; use an 0o prefix for octal integers"
		return lexer.failToken(SyntaxError, message, span, false)
	}

	name := "decimal"
	if imaginary {
		name = "imaginary"
	}
	if errToken, err := lexer.verifyNumberEnd(start, offset, name); err != nil {
		return errToken, err
	}
	lexer.offset = offset
	lexer.lineHasCode = true
	return lexer.token(Number, start, offset), nil
}

// scanDecimalDigits consumes a non-empty digit part and returns the underscore
// offset when separators are not followed by another digit.
func (lexer *Lexer) scanDecimalDigits(offset int) (int, int) {
	return lexer.scanDigitGroups(offset, isASCIIDigit)
}

func (lexer *Lexer) scanDigitGroups(offset int, validDigit func(byte) bool) (int, int) {
	for {
		for validDigit(lexer.peekByte(offset)) {
			offset++
		}
		if lexer.peekByte(offset) != '_' {
			return offset, -1
		}
		underscore := offset
		offset++
		if !validDigit(lexer.peekByte(offset)) {
			return offset, underscore
		}
	}
}

var numberFollowKeywords = [...]string{"and", "else", "for", "if", "in", "is", "not", "or"}

func (lexer *Lexer) verifyNumberEnd(start, offset int, name string) (Token, error) {
	if !isASCIIIdentifierContinue(lexer.peekByte(offset)) {
		return Token{}, nil
	}
	for _, keyword := range numberFollowKeywords {
		if lexer.keywordAt(offset, keyword) {
			return Token{}, nil
		}
	}
	return lexer.invalidNumber(start, offset, name)
}

// peekByte returns zero outside the source. Source validation rejects NUL, so
// zero unambiguously marks unavailable lookahead during scanning.
func (lexer *Lexer) peekByte(offset int) byte {
	if offset < 0 || offset >= len(lexer.source) {
		return 0
	}
	return lexer.source[offset]
}

func (lexer *Lexer) keywordAt(offset int, keyword string) bool {
	end := offset + len(keyword)
	if end > len(lexer.source) || lexer.source[offset:end] != keyword {
		return false
	}
	return end == len(lexer.source) || !isASCIIIdentifierContinue(lexer.source[end])
}

func (lexer *Lexer) invalidBaseDigit(start, offset int, name string) (Token, error) {
	end := offset + 1
	span := lexer.span(start, end)
	message := fmt.Sprintf("invalid digit '%c' in %s literal", lexer.source[offset], name)
	return lexer.failToken(SyntaxError, message, span, false)
}

func (lexer *Lexer) invalidNumber(start, offset int, name string) (Token, error) {
	end := offset
	if end < len(lexer.source) {
		end++
	}
	if end < start {
		end = start
	}
	span := lexer.span(start, end)
	return lexer.failToken(SyntaxError, fmt.Sprintf("invalid %s literal", name), span, false)
}

func hasNonzeroDigit(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] >= '1' && text[index] <= '9' {
			return true
		}
	}
	return false
}

func isASCIIHexDigit(char byte) bool {
	return isASCIIDigit(char) || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F'
}

func isASCIIOctalDigit(char byte) bool {
	return char >= '0' && char <= '7'
}

func isASCIIBinaryDigit(char byte) bool {
	return char == '0' || char == '1'
}
