# case: type missing arguments
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type()
# ---
# case: type invalid argument count
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type(None, None)
# ---
# case: type keyword argument
# error: TypeError
# message: "type() takes 1 or 3 arguments"
type(object=None)
# ---
# case: dynamic type invalid name
# error: TypeError
# message: "type.__new__() argument 1 must be str, not int"
type(1, (), {})
# ---
# case: dynamic type invalid bases
# error: TypeError
# message: "type.__new__() argument 2 must be tuple, not list"
type('Dynamic', [], {})
# ---
# case: dynamic type invalid namespace
# error: TypeError
# message: "type.__new__() argument 3 must be dict, not list"
type('Dynamic', (), [])
# ---
# case: dynamic type invalid base entry
# error: TypeError
# message: "class base is not a type"
type('Dynamic', (1,), {})
# ---
# case: dynamic type non-string namespace key
# error: TypeError
# message: "type namespace keys must be strings"
type('Dynamic', (), {1: None})
# ---
# case: dynamic type invalid qualified name
# error: TypeError
# message: "type __qualname__ must be a str, not int"
type('Dynamic', (), {'__qualname__': 1})
# ---
# case: isinstance missing argument
# error: TypeError
# message: "isinstance expected 2 arguments, got 1"
isinstance(None)
# ---
# case: isinstance extra argument
# error: TypeError
# message: "isinstance expected 2 arguments, got 3"
isinstance(None, type(None), type(None))
# ---
# case: isinstance keyword arguments
# error: TypeError
# message: "isinstance() takes no keyword arguments"
isinstance(obj=None, class_or_tuple=type(None))
# ---
# case: isinstance invalid class
# error: TypeError
# message: "isinstance() arg 2 must be a type, a tuple of types, or a union"
isinstance(None, 1)
# ---
# case: isinstance invalid nested class
# error: TypeError
# message: "isinstance() arg 2 must be a type, a tuple of types, or a union"
isinstance(None, (str, (int, 1)))
# ---
# case: issubclass missing argument
# error: TypeError
# message: "issubclass expected 2 arguments, got 1"
issubclass(type(None))
# ---
# case: issubclass extra argument
# error: TypeError
# message: "issubclass expected 2 arguments, got 3"
issubclass(type(None), type(None), type(None))
# ---
# case: issubclass keyword arguments
# error: TypeError
# message: "issubclass() takes no keyword arguments"
issubclass(cls=type(None), class_or_tuple=type(None))
# ---
# case: issubclass invalid first class
# error: TypeError
# message: "issubclass() arg 1 must be a class"
issubclass(None, type(None))
# ---
# case: issubclass invalid second class
# error: TypeError
# message: "issubclass() arg 2 must be a class, a tuple of classes, or a union"
issubclass(type(None), None)
# ---
# case: issubclass invalid nested class
# error: TypeError
# message: "issubclass() arg 2 must be a class, a tuple of classes, or a union"
issubclass(type(None), (str, (int, None)))
# ---
# case: list constructor extra argument
# error: TypeError
# message: "list expected at most 1 argument, got 2"
list(None, None)
# ---
# case: tuple constructor keyword argument
# error: TypeError
# message: "tuple() takes no keyword arguments"
tuple(iterable=())
# ---
# case: list constructor non-iterable
# error: TypeError
# message: "'int' object is not iterable"
list(1)
# ---
# case: tuple constructor iterator failure
# error: ValueError
# message: "iterator failed"
class FailingSequenceIterator:
    def __iter__(self):
        return self

    def __next__(self):
        raise ValueError('iterator failed')

tuple(FailingSequenceIterator())
# ---
# case: list constructor iterator lookup stop
# error: StopIteration
# message: "lookup stopped"
class StoppedSequenceLookup:
    def __iter__(self):
        raise StopIteration('lookup stopped')

list(StoppedSequenceLookup())
# ---
# case: tuple constructor generator failure
# error: ValueError
# message: "generator failed"
def failing_sequence_generator():
    yield 1
    raise ValueError('generator failed')

tuple(failing_sequence_generator())
# ---
# case: set constructor extra argument
# error: TypeError
# message: "set expected at most 1 argument, got 2"
set(None, None)
# ---
# case: set constructor keyword argument
# error: TypeError
# message: "set() takes no keyword arguments"
set(iterable=())
# ---
# case: set constructor non-iterable
# error: TypeError
# message: "'int' object is not iterable"
set(1)
# ---
# case: set constructor unhashable element
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
set(([1],))
# ---
# case: dict constructor extra argument
# error: TypeError
# message: "dict expected at most 1 argument, got 2"
dict(None, None)
# ---
# case: dict constructor short entry
# error: ValueError
# message: "dictionary update sequence element #0 has length 1; 2 is required"
dict(((1,),))
# ---
# case: dict constructor long entry
# error: ValueError
# message: "dictionary update sequence element #0 has length 3; 2 is required"
dict(((1, 2, 3),))
# ---
# case: dict constructor non-sequence entry
# error: TypeError
# message: "cannot convert dictionary update sequence element #0 to a sequence"
dict((1,))
# ---
# case: dict constructor unhashable key
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
dict((([1], 2),))
# ---
# case: object positional argument
# error: TypeError
# message: "object() takes no arguments"
object(None)
# ---
# case: object keyword argument
# error: TypeError
# message: "object() takes no arguments"
object(value=None)
# ---
# case: frozenset extra argument
# error: TypeError
# message: "frozenset expected at most 1 argument, got 2"
frozenset(None, None)
# ---
# case: frozenset keyword argument
# error: TypeError
# message: "frozenset() takes no keyword arguments"
frozenset(iterable=())
# ---
# case: frozenset non-iterable
# error: TypeError
# message: "'int' object is not iterable"
frozenset(1)
# ---
# case: frozenset unhashable element
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
frozenset(([1],))
