package lexer

import "fmt"

// Kind is a Python 3.14 lexical token kind. Its values follow CPython's
// Grammar/Tokens ordering to make conformance output easy to compare.
type Kind uint8

const (
	EndMarker Kind = iota
	Name
	Number
	String
	Newline
	Indent
	Dedent
	LParen
	RParen
	LSquare
	RSquare
	Colon
	Comma
	Semicolon
	Plus
	Minus
	Star
	Slash
	VBar
	Ampersand
	Less
	Greater
	Equal
	Dot
	Percent
	LBrace
	RBrace
	EqualEqual
	NotEqual
	LessEqual
	GreaterEqual
	Tilde
	Circumflex
	LeftShift
	RightShift
	DoubleStar
	PlusEqual
	MinusEqual
	StarEqual
	SlashEqual
	PercentEqual
	AmpersandEqual
	VBarEqual
	CircumflexEqual
	LeftShiftEqual
	RightShiftEqual
	DoubleStarEqual
	DoubleSlash
	DoubleSlashEqual
	At
	AtEqual
	RightArrow
	Ellipsis
	ColonEqual
	Exclamation
	Op
	TypeIgnore
	TypeComment
	SoftKeyword
	FStringStart
	FStringMiddle
	FStringEnd
	TStringStart
	TStringMiddle
	TStringEnd
	Comment
	NL
	ErrorToken
	Encoding

	kindCount
)

var kindNames = [...]string{
	"ENDMARKER",
	"NAME",
	"NUMBER",
	"STRING",
	"NEWLINE",
	"INDENT",
	"DEDENT",
	"LPAR",
	"RPAR",
	"LSQB",
	"RSQB",
	"COLON",
	"COMMA",
	"SEMI",
	"PLUS",
	"MINUS",
	"STAR",
	"SLASH",
	"VBAR",
	"AMPER",
	"LESS",
	"GREATER",
	"EQUAL",
	"DOT",
	"PERCENT",
	"LBRACE",
	"RBRACE",
	"EQEQUAL",
	"NOTEQUAL",
	"LESSEQUAL",
	"GREATEREQUAL",
	"TILDE",
	"CIRCUMFLEX",
	"LEFTSHIFT",
	"RIGHTSHIFT",
	"DOUBLESTAR",
	"PLUSEQUAL",
	"MINEQUAL",
	"STAREQUAL",
	"SLASHEQUAL",
	"PERCENTEQUAL",
	"AMPEREQUAL",
	"VBAREQUAL",
	"CIRCUMFLEXEQUAL",
	"LEFTSHIFTEQUAL",
	"RIGHTSHIFTEQUAL",
	"DOUBLESTAREQUAL",
	"DOUBLESLASH",
	"DOUBLESLASHEQUAL",
	"AT",
	"ATEQUAL",
	"RARROW",
	"ELLIPSIS",
	"COLONEQUAL",
	"EXCLAMATION",
	"OP",
	"TYPE_IGNORE",
	"TYPE_COMMENT",
	"SOFT_KEYWORD",
	"FSTRING_START",
	"FSTRING_MIDDLE",
	"FSTRING_END",
	"TSTRING_START",
	"TSTRING_MIDDLE",
	"TSTRING_END",
	"COMMENT",
	"NL",
	"ERRORTOKEN",
	"ENCODING",
}

func (kind Kind) String() string {
	if kind >= kindCount {
		return fmt.Sprintf("Kind(%d)", kind)
	}
	return kindNames[kind]
}

// Position identifies a byte boundary in UTF-8 source. Line is one-based and
// Column is the zero-based byte offset from the start of that physical line.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span is a half-open source range.
type Span struct {
	Start Position
	End   Position
}

// Token retains the exact source spelling. Synthetic NEWLINE and DEDENT tokens
// have empty Text and a zero-width byte span.
type Token struct {
	Kind Kind
	Text string
	Span Span
}

func (token Token) String() string {
	return fmt.Sprintf("%s %q %d:%d", token.Kind, token.Text, token.Span.Start.Line, token.Span.Start.Column)
}

// IsTrivia reports whether a parser can skip the token without changing the
// Python grammar token stream.
func (token Token) IsTrivia() bool {
	return token.Kind == Comment || token.Kind == NL
}

var operators = map[string]Kind{
	"(":   LParen,
	")":   RParen,
	"[":   LSquare,
	"]":   RSquare,
	":":   Colon,
	",":   Comma,
	";":   Semicolon,
	"+":   Plus,
	"-":   Minus,
	"*":   Star,
	"/":   Slash,
	"|":   VBar,
	"&":   Ampersand,
	"<":   Less,
	">":   Greater,
	"=":   Equal,
	".":   Dot,
	"%":   Percent,
	"{":   LBrace,
	"}":   RBrace,
	"~":   Tilde,
	"^":   Circumflex,
	"@":   At,
	"!":   Exclamation,
	"==":  EqualEqual,
	"!=":  NotEqual,
	"<=":  LessEqual,
	"<>":  NotEqual,
	">=":  GreaterEqual,
	"<<":  LeftShift,
	">>":  RightShift,
	"**":  DoubleStar,
	"+=":  PlusEqual,
	"-=":  MinusEqual,
	"*=":  StarEqual,
	"/=":  SlashEqual,
	"%=":  PercentEqual,
	"&=":  AmpersandEqual,
	"|=":  VBarEqual,
	"^=":  CircumflexEqual,
	"//":  DoubleSlash,
	"@=":  AtEqual,
	"->":  RightArrow,
	":=":  ColonEqual,
	"**=": DoubleStarEqual,
	"//=": DoubleSlashEqual,
	"<<=": LeftShiftEqual,
	">>=": RightShiftEqual,
	"...": Ellipsis,
}
