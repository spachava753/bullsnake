package bytecode

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// CodeFlags records execution properties of one code object.
type CodeFlags uint32

const (
	Optimized CodeFlags = 1 << iota
	NewLocals
	VarArgs
	VarKeywords
	Nested
)

var codeFlagNames = [...]struct {
	flag CodeFlags
	name string
}{
	{Optimized, "Optimized"},
	{NewLocals, "NewLocals"},
	{VarArgs, "VarArgs"},
	{VarKeywords, "VarKeywords"},
	{Nested, "Nested"},
}

// String returns the stable dump spelling of code flags.
func (flags CodeFlags) String() string {
	var names []string
	remaining := flags
	for _, entry := range codeFlagNames {
		if flags&entry.flag != 0 {
			names = append(names, entry.name)
			remaining &^= entry.flag
		}
	}
	if remaining != 0 {
		names = append(names, fmt.Sprintf("CodeFlags(%#x)", uint32(remaining)))
	}
	return strings.Join(names, ", ")
}

// CodeSpec contains the data copied into a new immutable code object.
type CodeSpec struct {
	Filename            string
	Name                string
	QualifiedName       string
	FirstLine           int
	Flags               CodeFlags
	PositionalOnlyCount int
	PositionalCount     int
	KeywordOnlyCount    int
	StackSize           int
	Instructions        []Instruction
	Positions           []lexer.Span
	Constants           []Constant
	Names               []string
	Locals              []string
	Cells               []string
	FreeVars            []string
	Children            []*Code
}

// Code is one immutable compiled module or nested function.
type Code struct {
	filename            string
	name                string
	qualifiedName       string
	firstLine           int
	flags               CodeFlags
	positionalOnlyCount int
	positionalCount     int
	keywordOnlyCount    int
	stackSize           int
	instructions        []Instruction
	positions           []lexer.Span
	constants           []Constant
	names               []string
	locals              []string
	cells               []string
	freeVars            []string
	children            []*Code
}

// NewCode copies a completed code specification.
func NewCode(spec CodeSpec) *Code {
	if len(spec.Instructions) != len(spec.Positions) {
		panic("bytecode: instruction and position counts differ")
	}
	return &Code{
		filename:            spec.Filename,
		name:                spec.Name,
		qualifiedName:       spec.QualifiedName,
		firstLine:           spec.FirstLine,
		flags:               spec.Flags,
		positionalOnlyCount: spec.PositionalOnlyCount,
		positionalCount:     spec.PositionalCount,
		keywordOnlyCount:    spec.KeywordOnlyCount,
		stackSize:           spec.StackSize,
		instructions:        slices.Clone(spec.Instructions),
		positions:           slices.Clone(spec.Positions),
		constants:           slices.Clone(spec.Constants),
		names:               slices.Clone(spec.Names),
		locals:              slices.Clone(spec.Locals),
		cells:               slices.Clone(spec.Cells),
		freeVars:            slices.Clone(spec.FreeVars),
		children:            slices.Clone(spec.Children),
	}
}

// Filename returns the source filename associated with the code.
func (code *Code) Filename() string { return code.filename }

// Name returns the code object's short name.
func (code *Code) Name() string { return code.name }

// QualifiedName returns the code object's qualified name.
func (code *Code) QualifiedName() string { return code.qualifiedName }

// FirstLine returns the first source line associated with the code.
func (code *Code) FirstLine() int { return code.firstLine }

// Flags returns the code object's execution flags.
func (code *Code) Flags() CodeFlags { return code.flags }

// PositionalOnlyCount returns the number of positional-only parameters.
func (code *Code) PositionalOnlyCount() int { return code.positionalOnlyCount }

// PositionalCount returns the total number of positional parameters.
func (code *Code) PositionalCount() int { return code.positionalCount }

// KeywordOnlyCount returns the number of keyword-only parameters.
func (code *Code) KeywordOnlyCount() int { return code.keywordOnlyCount }

// StackSize returns the maximum operand-stack depth.
func (code *Code) StackSize() int { return code.stackSize }

// Instructions returns a copy of the instruction sequence.
func (code *Code) Instructions() []Instruction { return slices.Clone(code.instructions) }

// Constants returns a copy of the constant table.
func (code *Code) Constants() []Constant { return slices.Clone(code.constants) }

// Names returns a copy of the referenced-name table.
func (code *Code) Names() []string { return slices.Clone(code.names) }

// Locals returns a copy of the fast-local name table.
func (code *Code) Locals() []string { return slices.Clone(code.locals) }

// Cells returns a copy of the cell-variable name table.
func (code *Code) Cells() []string { return slices.Clone(code.cells) }

// FreeVars returns a copy of the free-variable name table.
func (code *Code) FreeVars() []string { return slices.Clone(code.freeVars) }

// Children returns a copy of the nested code-object table.
func (code *Code) Children() []*Code { return slices.Clone(code.children) }

// Position returns the source span for one instruction index.
func (code *Code) Position(index int) (lexer.Span, bool) {
	if index < 0 || index >= len(code.positions) {
		return lexer.Span{}, false
	}
	return code.positions[index], true
}
