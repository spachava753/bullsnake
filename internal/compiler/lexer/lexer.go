package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

const (
	tabSize       = 8
	maxIndent     = 100
	maxDelimiters = 200
	maxStringMode = 150
)

type indentLevel struct {
	column    int
	alternate int
}

type indentationMeasure struct {
	start              int
	textStart          int
	column             int
	alternate          int
	continuationColumn int
}

type delimiter struct {
	char byte
	span Span
}

type stringKind uint8

const (
	fString stringKind = iota
	tString
)

type stringMode struct {
	kind           stringKind
	quote          byte
	quoteSize      int
	raw            bool
	start          int
	text           bool
	curlyDepth     int
	exprStartDepth int
	inFormatSpec   bool
}

// Lexer scans one decoded UTF-8 source unit. It owns no runtime state and may
// be discarded after compilation.
type Lexer struct {
	// filename is included in lexical errors but does not affect scanning.
	filename string
	// source is the complete immutable, decoded UTF-8 input.
	source string
	// offset is the byte position of the next unread source character.
	offset int

	// lineStarts stores the absolute byte offset of each physical line.
	lineStarts []int
	// atLineStart requests indentation handling before the next token.
	atLineStart bool
	// lineHasCode distinguishes logical NEWLINE from non-significant NL.
	lineHasCode bool

	// indents stores active primary and alternate indentation columns.
	indents []indentLevel
	// delimiters stores unmatched openers for nesting and implicit line joining.
	delimiters []delimiter
	// modes stores nested f-string and t-string scanner state.
	modes []stringMode
	// pending stores generated tokens that must be returned before more scanning.
	pending []Token

	// stopped records that a fatal lexical error has ended scanning.
	stopped bool
	// emittedEnd records that Next has returned the real ENDMARKER.
	emittedEnd bool
	// virtualEOF records a synthetic newline for a final line with no line ending.
	virtualEOF bool
}

// New constructs a lexer for source with no filename in diagnostics. It
// rejects malformed UTF-8 and null bytes before scanning begins.
func New(source string) (*Lexer, error) {
	return NewFile("", source)
}

// NewFile constructs a lexer for decoded UTF-8 source. filename is used only
// in diagnostics; NewFile performs no file I/O.
func NewFile(filename, source string) (*Lexer, error) {
	lexer := &Lexer{
		filename:    filename,
		source:      source,
		lineStarts:  makeLineStarts(source),
		atLineStart: true,
		indents:     []indentLevel{{}},
	}
	if err := lexer.validateSource(); err != nil {
		return nil, err
	}
	return lexer, nil
}

// Next returns the next token. Calling it after ENDMARKER returns ENDMARKER
// again, which keeps parser lookahead code simple.
func (lexer *Lexer) Next() (Token, error) {
	if lexer.stopped || lexer.emittedEnd {
		return lexer.zeroWidthToken(EndMarker, lexer.eofPosition()), nil
	}
	if len(lexer.pending) != 0 {
		return lexer.dequeue(), nil
	}
	if len(lexer.modes) != 0 && lexer.modes[len(lexer.modes)-1].text {
		return lexer.scanStringText()
	}
	return lexer.scanNormal()
}

// scanNormal handles queued tokens and line indentation before dispatching the
// next non-whitespace source byte.
func (lexer *Lexer) scanNormal() (Token, error) {
	for {
		if len(lexer.pending) != 0 {
			return lexer.dequeue(), nil
		}
		if lexer.atLineStart {
			token, emit, err := lexer.scanIndentation()
			if err != nil || emit {
				return token, err
			}
		}

	skipWhitespace:
		for lexer.offset < len(lexer.source) {
			switch lexer.source[lexer.offset] {
			case ' ', '\t', '\f':
				lexer.offset++
			default:
				break skipWhitespace
			}
		}
		if lexer.offset == len(lexer.source) {
			return lexer.scanEOF()
		}

		token, retry, err := lexer.scanNormalToken()
		if retry {
			continue
		}
		return token, err
	}
}

// scanNormalToken dispatches the current byte to line handling, literal
// scanning, identifier scanning, or maximal-munch operator scanning.
func (lexer *Lexer) scanNormalToken() (Token, bool, error) {
	start := lexer.offset
	current := lexer.source[start]
	if current == '#' {
		lexer.offset = lexer.lineEnd(start)
		return lexer.token(Comment, start, lexer.offset), false, nil
	}
	if size := newlineSize(lexer.source, start); size != 0 {
		return lexer.scanPhysicalNewline(start, size), false, nil
	}
	if current == '\\' {
		return lexer.scanLineContinuation(start)
	}

	switch current {
	case '.':
		if start+1 < len(lexer.source) && isASCIIDigit(lexer.source[start+1]) {
			token, err := lexer.scanNumber()
			return token, false, err
		}
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		token, err := lexer.scanNumber()
		return token, false, err
	case '\'', '"':
		token, err := lexer.scanIdentifierOrString()
		return token, false, err
	}
	if current >= utf8.RuneSelf || isASCIIIdentifierStart(current) {
		token, err := lexer.scanIdentifierOrString()
		return token, false, err
	}
	token, err := lexer.scanOperator()
	return token, false, err
}

func (lexer *Lexer) scanPhysicalNewline(start, size int) Token {
	lexer.offset += size
	lexer.atLineStart = true
	kind := NL
	if lexer.lineHasCode && len(lexer.delimiters) == 0 {
		kind = Newline
	}
	lexer.lineHasCode = false
	token := lexer.token(kind, start, lexer.offset)
	token.Span.End.Line = token.Span.Start.Line
	token.Span.End.Column = token.Span.Start.Column + size
	return token
}

func (lexer *Lexer) scanLineContinuation(start int) (Token, bool, error) {
	if size := newlineSize(lexer.source, start+1); size != 0 {
		lexer.offset += 1 + size
		if lexer.offset == len(lexer.source) {
			span := lexer.span(start, lexer.offset)
			token, err := lexer.failToken(SyntaxError, "unexpected EOF after line continuation character", span, true)
			return token, false, err
		}
		lexer.atLineStart = false
		return Token{}, true, nil
	}

	end := lexer.lineEnd(start + 1)
	span := lexer.span(start, end)
	token, err := lexer.failToken(SyntaxError, "unexpected character after line continuation character", span, false)
	return token, false, err
}

func (lexer *Lexer) lineEnd(offset int) int {
	for offset < len(lexer.source) && newlineSize(lexer.source, offset) == 0 {
		_, size := utf8.DecodeRuneInString(lexer.source[offset:])
		offset += size
	}
	return offset
}

// scanIndentation measures a physical line, ignores blank or implicitly joined
// lines, and applies significant indentation to the stack.
func (lexer *Lexer) scanIndentation() (Token, bool, error) {
	measure, token, err := lexer.measureIndentation()
	if err != nil {
		return token, true, err
	}

	lexer.atLineStart = false
	if lexer.offset == len(lexer.source) || lexer.source[lexer.offset] == '#' || newlineSize(lexer.source, lexer.offset) != 0 {
		return Token{}, false, nil
	}
	if len(lexer.delimiters) != 0 {
		return Token{}, false, nil
	}
	if measure.continuationColumn != 0 {
		measure.column = measure.continuationColumn
		measure.alternate = measure.continuationColumn
	}
	return lexer.applyIndentation(measure)
}

// measureIndentation consumes indentation across explicit continuations while
// tracking Python's primary and alternate tab-width columns.
func (lexer *Lexer) measureIndentation() (indentationMeasure, Token, error) {
	measure := indentationMeasure{start: lexer.offset, textStart: lexer.offset}
	for {
		lexer.consumeIndentationColumns(&measure)
		if lexer.offset >= len(lexer.source) || lexer.source[lexer.offset] != '\\' {
			return measure, Token{}, nil
		}
		size := newlineSize(lexer.source, lexer.offset+1)
		if size == 0 {
			return measure, Token{}, nil
		}
		if measure.continuationColumn == 0 {
			measure.continuationColumn = measure.column
		}
		lexer.offset += 1 + size
		measure.textStart = lexer.offset
		if lexer.offset == len(lexer.source) {
			span := lexer.span(lexer.offset-1-size, lexer.offset)
			token, err := lexer.failToken(SyntaxError, "unexpected EOF after line continuation character", span, true)
			return measure, token, err
		}
	}
}

func (lexer *Lexer) consumeIndentationColumns(measure *indentationMeasure) {
	for lexer.offset < len(lexer.source) {
		switch lexer.source[lexer.offset] {
		case ' ':
			measure.column++
			measure.alternate++
			lexer.offset++
		case '\t':
			measure.column = (measure.column/tabSize + 1) * tabSize
			measure.alternate++
			lexer.offset++
		case '\f':
			measure.column = 0
			measure.alternate = 0
			lexer.offset++
		default:
			return
		}
	}
}

// applyIndentation compares measured columns with the indentation stack,
// emitting one indent or queueing every dedent needed to reach a prior level.
func (lexer *Lexer) applyIndentation(measure indentationMeasure) (Token, bool, error) {
	current := lexer.indents[len(lexer.indents)-1]
	switch {
	case measure.column == current.column:
		if measure.alternate != current.alternate {
			span := lexer.span(measure.textStart, lexer.offset)
			return lexer.failIndent(TabError, "inconsistent use of tabs and spaces in indentation", span, false)
		}
		return Token{}, false, nil
	case measure.column > current.column:
		if len(lexer.indents) >= maxIndent {
			span := lexer.span(measure.start, lexer.offset)
			return lexer.failIndent(IndentationError, "too many levels of indentation", span, false)
		}
		if measure.alternate <= current.alternate {
			span := lexer.span(measure.textStart, lexer.offset)
			return lexer.failIndent(TabError, "inconsistent use of tabs and spaces in indentation", span, false)
		}
		lexer.indents = append(lexer.indents, indentLevel{column: measure.column, alternate: measure.alternate})
		return lexer.token(Indent, measure.textStart, lexer.offset), true, nil
	default:
		matched := -1
		for index := len(lexer.indents) - 1; index >= 0; index-- {
			if lexer.indents[index].column == measure.column {
				matched = index
				break
			}
		}
		if matched < 0 {
			span := lexer.span(measure.textStart, lexer.offset)
			return lexer.failIndent(IndentationError, "unindent does not match any outer indentation level", span, false)
		}
		if measure.alternate != lexer.indents[matched].alternate {
			span := lexer.span(measure.textStart, lexer.offset)
			return lexer.failIndent(TabError, "inconsistent use of tabs and spaces in indentation", span, false)
		}
		position := lexer.position(lexer.offset)
		for index := len(lexer.indents) - 1; index > matched; index-- {
			lexer.pending = append(lexer.pending, lexer.zeroWidthToken(Dedent, position))
		}
		lexer.indents = lexer.indents[:matched+1]
		return lexer.dequeue(), true, nil
	}
}

// scanOperator selects the longest operator spelling, updates delimiter state,
// and emits printable unknown characters as generic operators.
func (lexer *Lexer) scanOperator() (Token, error) {
	start := lexer.offset
	for size := 3; size >= 1; size-- {
		if start+size > len(lexer.source) {
			continue
		}
		text := lexer.source[start : start+size]
		kind, ok := operators[text]
		if !ok {
			continue
		}
		lexer.offset += size
		if errToken, err := lexer.updateDelimiters(kind, text[0], start); err != nil {
			return errToken, err
		}
		lexer.lineHasCode = true
		return lexer.token(kind, start, lexer.offset), nil
	}

	r, size := utf8.DecodeRuneInString(lexer.source[start:])
	lexer.offset += size
	if !unicode.IsPrint(r) {
		span := lexer.span(start, lexer.offset)
		return lexer.failToken(SyntaxError, fmt.Sprintf("invalid non-printable character U+%04X", r), span, false)
	}
	lexer.lineHasCode = true
	return lexer.token(Op, start, lexer.offset), nil
}

// updateDelimiters records openers, routes closers through validation, and
// enters f/t-string format-spec text after a replacement-field colon.
func (lexer *Lexer) updateDelimiters(kind Kind, char byte, start int) (Token, error) {
	switch kind {
	case LParen, LSquare, LBrace:
		if len(lexer.delimiters) >= maxDelimiters {
			span := lexer.span(start, lexer.offset)
			return lexer.failToken(SyntaxError, "too many nested parentheses", span, false)
		}
		lexer.delimiters = append(lexer.delimiters, delimiter{char: char, span: lexer.span(start, lexer.offset)})
		if mode := lexer.expressionMode(); mode != nil {
			mode.curlyDepth++
		}
	case RParen, RSquare, RBrace:
		return lexer.closeDelimiter(kind, char, start)
	case Colon:
		mode := lexer.expressionMode()
		if mode != nil && mode.curlyDepth-1 == mode.exprStartDepth {
			mode.text = true
			mode.inFormatSpec = true
		}
	}
	return Token{}, nil
}

// closeDelimiter validates the innermost opener and returns an f/t-string to
// text mode when the closer ends its current replacement field.
func (lexer *Lexer) closeDelimiter(kind Kind, char byte, start int) (Token, error) {
	mode := lexer.expressionMode()
	if mode != nil && kind == RBrace && mode.curlyDepth == 0 {
		span := lexer.span(start, lexer.offset)
		return lexer.failToken(SyntaxError, fmt.Sprintf("%c-string: single '}' is not allowed", mode.prefix()), span, false)
	}
	if len(lexer.delimiters) == 0 {
		span := lexer.span(start, lexer.offset)
		return lexer.failToken(SyntaxError, fmt.Sprintf("unmatched '%c'", char), span, false)
	}

	opener := lexer.delimiters[len(lexer.delimiters)-1]
	matches := opener.char == '(' && char == ')' ||
		opener.char == '[' && char == ']' ||
		opener.char == '{' && char == '}'
	if !matches {
		span := lexer.span(start, lexer.offset)
		message := fmt.Sprintf("closing parenthesis '%c' does not match opening parenthesis '%c'", char, opener.char)
		return lexer.failToken(SyntaxError, message, span, false)
	}
	lexer.delimiters = lexer.delimiters[:len(lexer.delimiters)-1]
	if mode != nil {
		mode.curlyDepth--
		if kind == RBrace && mode.curlyDepth == mode.exprStartDepth {
			mode.exprStartDepth--
			mode.text = true
			mode.inFormatSpec = false
		}
	}
	return Token{}, nil
}

// scanEOF reports unclosed delimiters, emits a synthetic final newline and
// remaining dedents in order, then emits ENDMARKER.
func (lexer *Lexer) scanEOF() (Token, error) {
	if len(lexer.delimiters) != 0 {
		opener := lexer.delimiters[len(lexer.delimiters)-1]
		message := fmt.Sprintf("'%c' was never closed", opener.char)
		return lexer.failToken(SyntaxError, message, opener.span, true)
	}
	lastLineHasBytes := lexer.lineStarts[len(lexer.lineStarts)-1] < len(lexer.source)
	if !lexer.virtualEOF && lastLineHasBytes {
		kind := NL
		if lexer.lineHasCode {
			kind = Newline
		}
		lexer.lineHasCode = false
		position := lexer.position(len(lexer.source))
		end := position
		end.Column++
		lexer.virtualEOF = true
		return Token{Kind: kind, Span: Span{Start: position, End: end}}, nil
	}
	if len(lexer.indents) > 1 {
		position := lexer.eofPosition()
		lexer.indents = lexer.indents[:len(lexer.indents)-1]
		return lexer.zeroWidthToken(Dedent, position), nil
	}
	lexer.emittedEnd = true
	return lexer.zeroWidthToken(EndMarker, lexer.eofPosition()), nil
}

func (lexer *Lexer) expressionMode() *stringMode {
	if len(lexer.modes) == 0 {
		return nil
	}
	mode := &lexer.modes[len(lexer.modes)-1]
	if mode.text {
		return nil
	}
	return mode
}

func (mode *stringMode) prefix() byte {
	if mode.kind == tString {
		return 't'
	}
	return 'f'
}

func (lexer *Lexer) token(kind Kind, start, end int) Token {
	return Token{Kind: kind, Text: lexer.source[start:end], Span: lexer.span(start, end)}
}

func (lexer *Lexer) zeroWidthToken(kind Kind, position Position) Token {
	return Token{Kind: kind, Span: Span{Start: position, End: position}}
}

func (lexer *Lexer) failToken(kind ErrorKind, message string, span Span, incomplete bool) (Token, error) {
	err := &Error{
		Kind:       kind,
		Message:    message,
		Filename:   lexer.filename,
		Span:       span,
		Incomplete: incomplete,
	}
	lexer.stopped = true
	text := lexer.source[span.Start.Offset:span.End.Offset]
	return Token{Kind: ErrorToken, Text: text, Span: span}, err
}

func (lexer *Lexer) failIndent(kind ErrorKind, message string, span Span, incomplete bool) (Token, bool, error) {
	token, err := lexer.failToken(kind, message, span, incomplete)
	return token, true, err
}

func (lexer *Lexer) dequeue() Token {
	token := lexer.pending[0]
	lexer.pending = lexer.pending[1:]
	return token
}

func (lexer *Lexer) span(start, end int) Span {
	return Span{Start: lexer.position(start), End: lexer.position(end)}
}

func (lexer *Lexer) position(offset int) Position {
	low := 0
	high := len(lexer.lineStarts)
	for low+1 < high {
		middle := low + (high-low)/2
		if lexer.lineStarts[middle] <= offset {
			low = middle
		} else {
			high = middle
		}
	}
	return Position{Offset: offset, Line: low + 1, Column: offset - lexer.lineStarts[low]}
}

func (lexer *Lexer) eofPosition() Position {
	position := lexer.position(len(lexer.source))
	if lexer.virtualEOF {
		position.Line++
		position.Column = 0
	}
	return position
}

func (lexer *Lexer) validateSource() *Error {
	for offset := 0; offset < len(lexer.source); {
		if lexer.source[offset] == 0 {
			span := lexer.span(offset, offset+1)
			return &Error{Kind: SyntaxError, Message: "source code cannot contain null bytes", Filename: lexer.filename, Span: span}
		}
		r, size := utf8.DecodeRuneInString(lexer.source[offset:])
		if r == utf8.RuneError && size == 1 {
			span := lexer.span(offset, offset+1)
			return &Error{Kind: EncodingError, Message: "source is not valid UTF-8", Filename: lexer.filename, Span: span}
		}
		offset += size
	}
	return nil
}

func makeLineStarts(source string) []int {
	starts := []int{0}
	for offset := 0; offset < len(source); {
		size := newlineSize(source, offset)
		if size == 0 {
			_, size = utf8.DecodeRuneInString(source[offset:])
			offset += size
			continue
		}
		offset += size
		starts = append(starts, offset)
	}
	return starts
}

// newlineSize recognizes LF, CR, and CRLF at offset and returns their byte
// width without advancing the source.
func newlineSize(source string, offset int) int {
	if offset < 0 || offset >= len(source) {
		return 0
	}
	switch source[offset] {
	case '\n':
		return 1
	case '\r':
		if offset+1 < len(source) && source[offset+1] == '\n' {
			return 2
		}
		return 1
	default:
		return 0
	}
}

func isASCIIDigit(char byte) bool {
	return char >= '0' && char <= '9'
}
