// Package resolver classifies names and validates rules that depend on their
// surrounding scopes.
package resolver

import (
	"fmt"
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// Features records module-wide behavior selected by future imports.
type Features uint8

const (
	// FutureAnnotations records from __future__ import annotations.
	FutureAnnotations Features = 1 << iota
)

// ScopePurpose distinguishes multiple scopes created by one AST node.
type ScopePurpose uint8

const (
	ModuleBody ScopePurpose = iota
	DefinitionBody
	ComprehensionBody
	Annotations
	TypeParameters
	TypeAliasValue
	TypeVariableBound
	TypeVariableDefault
)

var scopePurposeNames = [...]string{
	"ModuleBody",
	"DefinitionBody",
	"ComprehensionBody",
	"Annotations",
	"TypeParameters",
	"TypeAliasValue",
	"TypeVariableBound",
	"TypeVariableDefault",
}

// String returns the purpose name used in diagnostics and tests.
func (purpose ScopePurpose) String() string {
	if int(purpose) >= len(scopePurposeNames) {
		return fmt.Sprintf("ScopePurpose(%d)", purpose)
	}
	return scopePurposeNames[purpose]
}

// ScopeKind identifies the Python rules that apply to one scope.
type ScopeKind uint8

const (
	ModuleScope ScopeKind = iota
	FunctionScope
	ClassScope
	AnnotationScope
	TypeParametersScope
	TypeVariableScope
	TypeAliasScope
)

var scopeKindNames = [...]string{
	"Module",
	"Function",
	"Class",
	"Annotation",
	"TypeParameters",
	"TypeVariable",
	"TypeAlias",
}

// String returns the scope kind used in stable table dumps.
func (kind ScopeKind) String() string {
	if int(kind) >= len(scopeKindNames) {
		return fmt.Sprintf("ScopeKind(%d)", kind)
	}
	return scopeKindNames[kind]
}

// NameScope says where execution obtains the value of a name.
type NameScope uint8

const (
	Unresolved NameScope = iota
	Local
	Cell
	Free
	GlobalExplicit
	GlobalImplicit
)

var nameScopeNames = [...]string{
	"Unresolved",
	"Local",
	"Cell",
	"Free",
	"GlobalExplicit",
	"GlobalImplicit",
}

// String returns the resolved name classification.
func (scope NameScope) String() string {
	if int(scope) >= len(nameScopeNames) {
		return fmt.Sprintf("NameScope(%d)", scope)
	}
	return nameScopeNames[scope]
}

// SymbolFlags records how source code uses one name.
type SymbolFlags uint16

const (
	Used SymbolFlags = 1 << iota
	Assigned
	Parameter
	Imported
	Annotated
	GlobalDeclaration
	NonlocalDeclaration
	TypeParameter
	ComprehensionIterator
	FreeThroughClass
)

var symbolFlagNames = [...]struct {
	flag SymbolFlags
	name string
}{
	{Used, "Used"},
	{Assigned, "Assigned"},
	{Parameter, "Parameter"},
	{Imported, "Imported"},
	{Annotated, "Annotated"},
	{GlobalDeclaration, "GlobalDeclaration"},
	{NonlocalDeclaration, "NonlocalDeclaration"},
	{TypeParameter, "TypeParameter"},
	{ComprehensionIterator, "ComprehensionIterator"},
	{FreeThroughClass, "FreeThroughClass"},
}

// String returns a vertical-bar-separated list of symbol flags.
func (flags SymbolFlags) String() string {
	var names []string
	remaining := flags
	for _, entry := range symbolFlagNames {
		if flags&entry.flag != 0 {
			names = append(names, entry.name)
			remaining &^= entry.flag
		}
	}
	if remaining != 0 {
		names = append(names, fmt.Sprintf("SymbolFlags(%#x)", uint16(remaining)))
	}
	return strings.Join(names, "|")
}

// ScopeFlags records properties of a scope needed by validation or compilation.
type ScopeFlags uint32

const (
	Nested ScopeFlags = 1 << iota
	AsyncFunction
	Generator
	Coroutine
	ReturnsValue
	VarArgs
	VarKeywords
	Method
	CanSeeClassScope
	NeedsClassClosure
	NeedsClassDict
	UsesAnnotations
	UnevaluatedAnnotations
	ListComprehension
	SetComprehension
	DictComprehension
	GeneratorExpression
)

var scopeFlagNames = [...]struct {
	flag ScopeFlags
	name string
}{
	{Nested, "Nested"},
	{AsyncFunction, "AsyncFunction"},
	{Generator, "Generator"},
	{Coroutine, "Coroutine"},
	{Method, "Method"},
	{ReturnsValue, "ReturnsValue"},
	{VarArgs, "VarArgs"},
	{VarKeywords, "VarKeywords"},
	{CanSeeClassScope, "CanSeeClassScope"},
	{NeedsClassClosure, "NeedsClassClosure"},
	{NeedsClassDict, "NeedsClassDict"},
	{UsesAnnotations, "UsesAnnotations"},
	{UnevaluatedAnnotations, "UnevaluatedAnnotations"},
	{ListComprehension, "ListComprehension"},
	{SetComprehension, "SetComprehension"},
	{DictComprehension, "DictComprehension"},
	{GeneratorExpression, "GeneratorExpression"},
}

// String returns a vertical-bar-separated list of scope flags.
func (flags ScopeFlags) String() string {
	var names []string
	remaining := flags
	for _, entry := range scopeFlagNames {
		if flags&entry.flag != 0 {
			names = append(names, entry.name)
			remaining &^= entry.flag
		}
	}
	if remaining != 0 {
		names = append(names, fmt.Sprintf("ScopeFlags(%#x)", uint32(remaining)))
	}
	return strings.Join(names, "|")
}

// Symbol contains the recorded uses and final classification of one name.
type Symbol struct {
	Name         string
	Flags        SymbolFlags
	Resolution   NameScope
	FirstSeen    lexer.Span
	FirstBinding lexer.Span
	Declaration  lexer.Span
}

// Scope contains the symbol table for one Python scope.
type Scope struct {
	Kind        ScopeKind
	Name        string
	Range       lexer.Span
	Parent      *Scope
	Children    []*Scope
	PrivateName string
	Symbols     map[string]*Symbol
	SymbolOrder []string
	Parameters  []string
	Flags       ScopeFlags

	redirectedBindings map[string]NameScope
	returnValueSpan    lexer.Span
}

type scopeKey struct {
	node    compilerast.Node
	purpose ScopePurpose
	item    int
}

// Table is the complete resolver result for one module.
type Table struct {
	Root     *Scope
	Features Features
	scopes   map[scopeKey]*Scope
}

// ScopeFor returns the scope with the requested purpose created by node.
func (table *Table) ScopeFor(node compilerast.Node, purpose ScopePurpose, item int) *Scope {
	if table == nil {
		return nil
	}
	return table.scopes[scopeKey{node: node, purpose: purpose, item: item}]
}

// Resolve builds symbol tables and validates one parsed module.
func Resolve(filename string, module *compilerast.Module) (*Table, error) {
	root := &Scope{
		Kind:    ModuleScope,
		Name:    "top",
		Range:   module.Span(),
		Symbols: make(map[string]*Symbol),
	}
	table := &Table{
		Root: root,
		scopes: map[scopeKey]*Scope{
			{node: module, purpose: ModuleBody}: root,
		},
	}
	state := &resolver{
		filename:    filename,
		table:       table,
		current:     root,
		annotations: make(map[*Scope]*Scope),
	}
	if err := state.scanFutureFeatures(module); err != nil {
		return nil, err
	}
	if err := state.collectStatements(module.Body); err != nil {
		return nil, err
	}
	if _, err := state.analyzeScope(state.table.Root, nil, nil, nil, nil); err != nil {
		return nil, err
	}
	return state.table, nil
}
