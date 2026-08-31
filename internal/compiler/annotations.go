package compiler

import (
	"strconv"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

const (
	annotationsName            = "__annotations__"
	conditionalAnnotationsName = "__conditional_annotations__"
)

type deferredAnnotation struct {
	statement *compilerast.AnnAssignStmt
	name      string
	index     int
}

func (compiler *compilerState) emitFutureAnnotationsMap(span lexer.Span) error {
	if err := compiler.emit(bytecode.BuildMap, 0, span); err != nil {
		return err
	}
	return compiler.emit(
		bytecode.StoreName,
		compiler.nameIndex(annotationsName),
		span,
	)
}

func (compiler *compilerState) compileFutureAnnotation(
	statement *compilerast.AnnAssignStmt,
	name string,
) error {
	name = compiler.scope.Mangle(name)
	text, err := compiler.annotationSource(statement.Annotation)
	if err != nil {
		return err
	}
	span := statement.Span()
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(text)),
		statement.Annotation.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadName,
		compiler.nameIndex(annotationsName),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(name)),
		statement.Target.Span(),
	); err != nil {
		return err
	}
	return compiler.emit(bytecode.StoreSubscript, 0, span)
}

func (compiler *compilerState) annotationSource(expression compilerast.Expr) (string, error) {
	span := expression.Span()
	source := compiler.module.Source()
	if span.Start.Offset < 0 || span.Start.Offset > span.End.Offset || span.End.Offset > len(source) {
		return "", compiler.error(span, "annotation span is outside module source")
	}
	return source[span.Start.Offset:span.End.Offset], nil
}

// compileFunctionAnnotations creates the lazy PEP 649 annotation callable with
// its public qualified name and leaves it below the function body payload.
func (compiler *compilerState) compileFunctionAnnotations(
	statement *compilerast.FunctionDefStmt,
	qualifiedName string,
) (bool, error) {
	scope := compiler.table.ScopeFor(statement, resolver.Annotations, 0)
	if scope == nil || scope.Kind != resolver.AnnotationScope {
		return false, compiler.error(statement.Span(), "resolver has no annotation scope for %q", statement.Name)
	}
	if scope.Flags&resolver.UsesAnnotations == 0 {
		return false, nil
	}

	child := compiler.newAnnotationCompiler(
		statement,
		scope,
		qualifiedName,
		false,
	)
	if err := child.emitAnnotationFormatGuard(statement.Span()); err != nil {
		return false, err
	}
	count, err := child.compileParameterAnnotations(statement.Parameters)
	if err != nil {
		return false, err
	}
	if statement.Returns != nil {
		if err := child.compileAnnotationEntry("return", statement.Returns); err != nil {
			return false, err
		}
		count++
	}
	if err := child.emit(bytecode.BuildMap, uint32(count), statement.Span()); err != nil {
		return false, err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, statement.Span()); err != nil {
		return false, err
	}
	code, err := child.finish()
	if err != nil {
		return false, err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return false, err
	}
	return true, nil
}

// deferAnnotation records one simple-name annotation and marks its index in the
// module namespace or class cell when that statement executes.
func (compiler *compilerState) deferAnnotation(
	statement *compilerast.AnnAssignStmt,
	name string,
) error {
	name = compiler.scope.Mangle(name)
	index := len(compiler.deferredAnnotations)
	compiler.deferredAnnotations = append(compiler.deferredAnnotations, deferredAnnotation{
		statement: statement,
		name:      name,
		index:     index,
	})
	if compiler.scope.Kind == resolver.ClassScope {
		index, err := compiler.derefIndex(conditionalAnnotationsName)
		if err != nil {
			return compiler.error(statement.Span(), "%v", err)
		}
		if err := compiler.emit(bytecode.LoadDeref, index, statement.Span()); err != nil {
			return err
		}
	} else if err := compiler.emit(
		bytecode.LoadName,
		compiler.nameIndex(conditionalAnnotationsName),
		statement.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer(strconv.Itoa(index))),
		statement.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.SetAdd, 0, statement.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, statement.Span())
}

// compileDeferredAnnotations emits a module or class annotation callable after
// all reachable statements have recorded their executed annotation indexes.
func (compiler *compilerState) compileDeferredAnnotations() error {
	if len(compiler.deferredAnnotations) == 0 || !compiler.reachable {
		return nil
	}
	first := compiler.deferredAnnotations[0].statement
	scope := compiler.table.ScopeFor(first, resolver.Annotations, 0)
	if scope == nil || scope.Kind != resolver.AnnotationScope {
		return compiler.error(first.Span(), "resolver has no annotation scope")
	}

	classAnnotations := compiler.scope.Kind == resolver.ClassScope
	qualifiedName := "__annotate__"
	storedName := "__annotate__"
	if classAnnotations {
		qualifiedName = compiler.childQualifiedName("__annotate__")
		storedName = "__annotate_func__"
	}
	child := compiler.newAnnotationCompiler(
		first,
		scope,
		qualifiedName,
		classAnnotations,
	)
	if err := child.emitAnnotationFormatGuard(first.Span()); err != nil {
		return err
	}
	if err := child.emit(bytecode.BuildMap, 0, first.Span()); err != nil {
		return err
	}
	for _, annotation := range compiler.deferredAnnotations {
		if err := child.compileDeferredAnnotation(annotation); err != nil {
			return err
		}
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, first.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, first.Span()); err != nil {
		return err
	}
	return compiler.emit(
		bytecode.StoreName,
		compiler.nameIndex(storedName),
		first.Span(),
	)
}

// compileDeferredAnnotation checks whether one annotation statement executed,
// then conditionally inserts its evaluated value into the shared result map.
func (compiler *compilerState) compileDeferredAnnotation(annotation deferredAnnotation) error {
	span := annotation.statement.Span()
	skip := compiler.newLabel()
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer(strconv.Itoa(annotation.index))),
		span,
	); err != nil {
		return err
	}
	if compiler.scope.Flags&resolver.CanSeeClassScope != 0 {
		index, err := compiler.derefIndex(conditionalAnnotationsName)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		if err := compiler.emit(bytecode.LoadDeref, index, span); err != nil {
			return err
		}
	} else if err := compiler.emit(
		bytecode.LoadGlobal,
		compiler.nameIndex(conditionalAnnotationsName),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, bytecode.CompareIn, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, skip, span); err != nil {
		return err
	}
	if err := compiler.compileExpr(annotation.statement.Annotation); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(annotation.name)),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.StoreSubscript, 0, span); err != nil {
		return err
	}
	return compiler.markLabel(skip, span)
}

func (compiler *compilerState) newAnnotationCompiler(
	owner compilerast.Node,
	scope *resolver.Scope,
	qualifiedName string,
	classAnnotations bool,
) *compilerState {
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:            compiler.filename,
		module:              compiler.module,
		owner:               owner,
		table:               compiler.table,
		scope:               scope,
		codeName:            "__annotate__",
		qualifiedName:       qualifiedName,
		firstLine:           owner.Span().Start.Line,
		codeFlags:           flags,
		positionalOnlyCount: 1,
		positionalCount:     1,
		localIDs:            make(map[string]uint32),
		derefIDs:            make(map[string]uint32),
		constantIDs:         make(map[bytecode.Constant]uint32),
		nameIDs:             make(map[string]uint32),
		reachable:           true,
	}
	// CPython hides the synthetic ".format" binding from source expressions but
	// exposes it as "format" in callable metadata.
	child.addLocal("format")
	if scope.Flags&resolver.CanSeeClassScope != 0 {
		child.addFree("__classdict__")
	}
	if classAnnotations {
		child.addFree(conditionalAnnotationsName)
	}
	child.initializeScopeLayout(scope)
	return child
}

// emitAnnotationFormatGuard accepts VALUE and the internal fake-globals mode,
// then raises NotImplementedError so annotationlib can handle higher formats.
func (compiler *compilerState) emitAnnotationFormatGuard(span lexer.Span) error {
	body := compiler.newLabel()
	if err := compiler.emit(bytecode.LoadFast, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer("2")),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CompareOp, bytecode.CompareGreater, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, body, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadNotImplementedError, 0, span); err != nil {
		return err
	}
	if err := compiler.emitTerminator(bytecode.RaiseVarargs, 1, span); err != nil {
		return err
	}
	return compiler.markLabel(body, span)
}

// compileParameterAnnotations follows CPython 3.14's observable dictionary
// order: ordinary positional parameters precede positional-only parameters.
func (compiler *compilerState) compileParameterAnnotations(
	parameters compilerast.Parameters,
) (int, error) {
	count := 0
	for _, group := range [][]compilerast.Parameter{
		parameters.Positional,
		parameters.PositionalOnly,
	} {
		for _, parameter := range group {
			if parameter.Annotation == nil {
				continue
			}
			if err := compiler.compileAnnotationEntry(parameter.Name, parameter.Annotation); err != nil {
				return 0, err
			}
			count++
		}
	}
	if parameters.VarArg != nil && parameters.VarArg.Annotation != nil {
		if err := compiler.compileAnnotationEntry(
			parameters.VarArg.Name,
			parameters.VarArg.Annotation,
		); err != nil {
			return 0, err
		}
		count++
	}
	for _, parameter := range parameters.KeywordOnly {
		if parameter.Annotation == nil {
			continue
		}
		if err := compiler.compileAnnotationEntry(parameter.Name, parameter.Annotation); err != nil {
			return 0, err
		}
		count++
	}
	if parameters.KeywordVarArg != nil && parameters.KeywordVarArg.Annotation != nil {
		if err := compiler.compileAnnotationEntry(
			parameters.KeywordVarArg.Name,
			parameters.KeywordVarArg.Annotation,
		); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (compiler *compilerState) compileAnnotationEntry(name string, expression compilerast.Expr) error {
	name = compiler.scope.Mangle(name)
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(name)),
		expression.Span(),
	); err != nil {
		return err
	}
	if compiler.table.Features&resolver.FutureAnnotations == 0 {
		return compiler.compileExpr(expression)
	}
	text, err := compiler.annotationSource(expression)
	if err != nil {
		return err
	}
	return compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(text)),
		expression.Span(),
	)
}
