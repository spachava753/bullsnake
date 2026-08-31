# Runtime errors for pure builtins.
# case: len missing argument
# error: TypeError
# message: "len() takes exactly one argument (0 given)"
len()
# ---
# case: len extra argument
# error: TypeError
# message: "len() takes exactly one argument (2 given)"
len([], [])
# ---
# case: len keyword argument
# error: TypeError
# message: "len() takes no keyword arguments"
len(obj=[])
# ---
# case: value without length protocol
# error: TypeError
# message: "object of type 'NoneType' has no len()"
len(None)
# ---
# case: disabled length protocol
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledLength:
    __len__ = None

len(DisabledLength())
# ---
# case: non-index length result
# error: TypeError
# message: "'float' object cannot be interpreted as an integer"
class FloatLength:
    def __len__(self):
        return 1.5

len(FloatLength())
# ---
# case: negative length result
# error: ValueError
# message: "__len__() should return >= 0"
class NegativeLength:
    def __len__(self):
        return -1

len(NegativeLength())
# ---
# case: overflowing length result
# error: OverflowError
# message: "cannot fit 'int' into an index-sized integer"
class HugeLength:
    def __len__(self):
        return 1 << 100

len(HugeLength())
# ---
# case: length method exception
# error: ValueError
# message: "length failed"
class FailingLength:
    def __len__(self):
        raise ValueError('length failed')

len(FailingLength())
# ---
# case: iter missing argument
# error: TypeError
# message: "iter expected at least 1 argument, got 0"
iter()
# ---
# case: iter extra arguments
# error: TypeError
# message: "iter expected at most 2 arguments, got 3"
iter([], None, None)
# ---
# case: iter keyword argument
# error: TypeError
# message: "iter() takes no keyword arguments"
iter(iterable=[])
# ---
# case: non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
iter(1)
# ---
# case: disabled iteration protocol
# error: TypeError
# message: "'DisabledIteration' object is not iterable"
class DisabledIteration:
    __iter__ = None

iter(DisabledIteration())
# ---
# case: invalid iterator result
# error: TypeError
# message: "iter() returned non-iterator of type 'int'"
class InvalidIteration:
    def __iter__(self):
        return 1

iter(InvalidIteration())
# ---
# case: iteration method exception
# error: ValueError
# message: "iteration failed"
class FailingIteration:
    def __iter__(self):
        raise ValueError('iteration failed')

iter(FailingIteration())
# ---
# case: callable sentinel iterator boundary
# error: NotImplementedError
# message: "iter() callable-sentinel form is not supported"
def produce():
    return None

iter(produce, None)
# ---
# case: getattr missing arguments
# error: TypeError
# message: "getattr expected at least 2 arguments, got 0"
getattr()
# ---
# case: getattr extra arguments
# error: TypeError
# message: "getattr expected at most 3 arguments, got 4"
getattr(None, 'value', None, None)
# ---
# case: getattr keyword argument
# error: TypeError
# message: "getattr() takes no keyword arguments"
getattr(None, name='value')
# ---
# case: getattr non-string name
# error: TypeError
# message: "attribute name must be string, not 'int'"
getattr(None, 1)
# ---
# case: getattr missing attribute without default
# error: AttributeError
# message: "'MissingAttribute' object has no attribute 'value'"
class MissingAttribute:
    pass

getattr(MissingAttribute(), 'value')
# ---
# case: getattr descriptor attribute error without default
# error: AttributeError
# message: "hidden field"
class MissingDescriptor:
    def __get__(self, instance, owner):
        raise AttributeError('hidden field')

class DescriptorOwner:
    field = MissingDescriptor()

getattr(DescriptorOwner(), 'field')
# ---
# case: getattr default preserves non-attribute errors
# error: ValueError
# message: "broken field"
class BrokenDescriptor:
    def __get__(self, instance, owner):
        raise ValueError('broken field')

class BrokenOwner:
    field = BrokenDescriptor()

getattr(BrokenOwner(), 'field', None)
# ---
# case: hasattr wrong argument count
# error: TypeError
# message: "hasattr expected 2 arguments, got 1"
hasattr(None)
# ---
# case: hasattr keyword argument
# error: TypeError
# message: "hasattr() takes no keyword arguments"
hasattr(None, name='value')
# ---
# case: hasattr non-string name
# error: TypeError
# message: "attribute name must be string, not 'int'"
hasattr(None, 1)
# ---
# case: hasattr preserves non-attribute errors
# error: ValueError
# message: "broken field"
class BrokenHasDescriptor:
    def __get__(self, instance, owner):
        raise ValueError('broken field')

class BrokenHasOwner:
    field = BrokenHasDescriptor()

hasattr(BrokenHasOwner(), 'field')
# ---
# case: callable missing argument
# error: TypeError
# message: "callable() takes exactly one argument (0 given)"
callable()
# ---
# case: callable keyword argument
# error: TypeError
# message: "callable() takes no keyword arguments"
callable(obj=None)
# ---
# case: plain instance call
# error: TypeError
# message: "'PlainCallable' object is not callable"
class PlainCallable:
    pass

PlainCallable()()
# ---
# case: disabled call method invocation
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledCall:
    __call__ = None

DisabledCall()()
# ---
# case: non-callable call method invocation
# error: TypeError
# message: "'int' object is not callable"
class InvalidCall:
    __call__ = 1

InvalidCall()()
# ---
# case: call method exception
# error: ValueError
# message: "call failed"
class FailingCall:
    def __call__(self):
        raise ValueError('call failed')

FailingCall()()
# ---
# case: classmethod missing argument
# error: TypeError
# message: "classmethod expected 1 argument, got 0"
classmethod()
# ---
# case: staticmethod extra argument
# error: TypeError
# message: "staticmethod expected 1 argument, got 2"
staticmethod(None, None)
# ---
# case: classmethod keyword argument
# error: TypeError
# message: "classmethod() takes no keyword arguments"
classmethod(function=None)
# ---
# case: staticmethod keyword argument
# error: TypeError
# message: "staticmethod() takes no keyword arguments"
staticmethod(function=None)
# ---
# case: classmethod wrapper is not callable
# error: TypeError
# message: "'classmethod' object is not callable"
def class_value(cls):
    return cls

classmethod(class_value)()
# ---
# case: non-callable classmethod payload
# error: TypeError
# message: "'int' object is not callable"
class InvalidClassMethod:
    value = classmethod(1)

InvalidClassMethod.value()
# ---
# case: class method exception
# error: ValueError
# message: "class method failed"
class FailingClassMethod:
    @classmethod
    def fail(cls):
        raise ValueError('class method failed')

FailingClassMethod.fail()
# ---
# case: bool extra arguments
# error: TypeError
# message: "bool expected at most 1 argument, got 2"
bool(1, 2)
# ---
# case: bool keyword argument
# error: TypeError
# message: "bool() takes no keyword arguments"
bool(value=1)
# ---
# case: bool method result type
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidBoolean:
    def __bool__(self):
        return 1

bool(InvalidBoolean())
# ---
# case: bool method exception
# error: ValueError
# message: "truth failed"
class FailingBoolean:
    def __bool__(self):
        raise ValueError('truth failed')

bool(FailingBoolean())
# ---
# case: repr missing argument
# error: TypeError
# message: "repr() takes exactly one argument (0 given)"
repr()
# ---
# case: repr keyword argument
# error: TypeError
# message: "repr() takes no keyword arguments"
repr(obj=None)
# ---
# case: repr method result type
# error: TypeError
# message: "__repr__ returned non-string (type int)"
class InvalidRepresentation:
    def __repr__(self):
        return 1

repr(InvalidRepresentation())
# ---
# case: repr method exception
# error: ValueError
# message: "representation failed"
class FailingRepresentation:
    def __repr__(self):
        raise ValueError('representation failed')

repr(FailingRepresentation())
# ---
# case: str extra arguments
# error: TypeError
# message: "str() takes at most 3 arguments (4 given)"
str(None, None, None, None)
# ---
# case: str encoding form boundary
# error: NotImplementedError
# message: "str() encoding form is not supported"
str(b'value', 'ascii')
# ---
# case: str method result type
# error: TypeError
# message: "__str__ returned non-string (type int)"
class InvalidString:
    def __str__(self):
        return 1

str(InvalidString())
# ---
# case: str method exception
# error: ValueError
# message: "string failed"
class FailingString:
    def __str__(self):
        raise ValueError('string failed')

str(FailingString())
# ---
# case: enumerate missing iterable
# error: TypeError
# message: "enumerate() missing required argument 'iterable'"
enumerate()
# ---
# case: enumerate extra arguments
# error: TypeError
# message: "enumerate() takes at most 2 arguments (3 given)"
enumerate((), 0, 1)
# ---
# case: enumerate invalid keyword
# error: TypeError
# message: "'unknown' is an invalid keyword argument for enumerate()"
enumerate((), unknown=1)
# ---
# case: enumerate non-integer start
# error: TypeError
# message: "'float' object cannot be interpreted as an integer"
enumerate((), 1.5)
# ---
# case: enumerate non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
enumerate(1)
# ---
# case: enumerate iterator failure
# error: ValueError
# message: "enumerate failed"
class FailingEnumeratedIterator:
    def __iter__(self):
        return self

    def __next__(self):
        raise ValueError('enumerate failed')

next(enumerate(FailingEnumeratedIterator()))
# ---
# case: all missing iterable
# error: TypeError
# message: "all() takes exactly one argument (0 given)"
all()
# ---
# case: all extra arguments
# error: TypeError
# message: "all() takes exactly one argument (2 given)"
all((), ())
# ---
# case: all keyword argument
# error: TypeError
# message: "all() takes no keyword arguments"
all(iterable=())
# ---
# case: all non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
all(1)
# ---
# case: all iterator failure
# error: ValueError
# message: "all iteration failed"
class FailingAllIterator:
    def __iter__(self):
        return self

    def __next__(self):
        raise ValueError('all iteration failed')

all(FailingAllIterator())
# ---
# case: all truth failure
# error: ValueError
# message: "all truth failed"
class FailingAllTruth:
    def __bool__(self):
        raise ValueError('all truth failed')

all((FailingAllTruth(),))
# ---
# case: any missing iterable
# error: TypeError
# message: "any() takes exactly one argument (0 given)"
any()
# ---
# case: any extra arguments
# error: TypeError
# message: "any() takes exactly one argument (2 given)"
any((), ())
# ---
# case: any keyword argument
# error: TypeError
# message: "any() takes no keyword arguments"
any(iterable=())
# ---
# case: any non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
any(1)
# ---
# case: any truth failure
# error: ValueError
# message: "any truth failed"
class FailingAnyTruth:
    def __bool__(self):
        raise ValueError('any truth failed')

any((FailingAnyTruth(),))
# ---
# case: hash missing argument
# error: TypeError
# message: "hash() takes exactly one argument (0 given)"
hash()
# ---
# case: hash extra arguments
# error: TypeError
# message: "hash() takes exactly one argument (2 given)"
hash(None, None)
# ---
# case: hash keyword argument
# error: TypeError
# message: "hash() takes no keyword arguments"
hash(obj=None)
# ---
# case: hash mutable list
# error: TypeError
# message: "unhashable type: 'list'"
hash([])
# ---
# case: hash tuple with mutable value
# error: TypeError
# message: "unhashable type: 'list'"
hash((1, []))
# ---
# case: disabled user hash
# error: TypeError
# message: "unhashable type: 'DisabledHash'"
class DisabledHash:
    __hash__ = None

hash(DisabledHash())
# ---
# case: non-integer user hash
# error: TypeError
# message: "__hash__ method should return an integer"
class InvalidHash:
    def __hash__(self):
        return 'invalid'

hash(InvalidHash())
# ---
# case: user hash failure
# error: ValueError
# message: "hash failed"
class FailingHash:
    def __hash__(self):
        raise ValueError('hash failed')

hash(FailingHash())
# ---
# case: map missing iterable
# error: TypeError
# message: "map() must have at least two arguments."
map(None)
# ---
# case: map keyword boundary
# error: NotImplementedError
# message: "map keyword arguments are not supported"
map(str, (), strict=True)
# ---
# case: map multiple iterables boundary
# error: NotImplementedError
# message: "map with multiple iterables is not supported"
map(lambda left, right: left + right, (1,), (2,))
# ---
# case: map non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
map(str, 1)
# ---
# case: map non-callable value
# error: TypeError
# message: "'NoneType' object is not callable"
next(map(None, (1,)))
# ---
# case: map callable failure
# error: ValueError
# message: "map call failed"
def failing_map(value):
    raise ValueError('map call failed')

next(map(failing_map, (1,)))
# ---
# case: map iterator failure
# error: ValueError
# message: "map iteration failed"
class FailingMappedIterator:
    def __iter__(self):
        return self

    def __next__(self):
        raise ValueError('map iteration failed')

next(map(str, FailingMappedIterator()))
# ---
# case: dir extra arguments
# error: TypeError
# message: "dir expected at most 1 argument, got 2"
dir(None, None)
# ---
# case: dir keyword argument
# error: TypeError
# message: "dir() takes no keyword arguments"
dir(obj=None)
# ---
# case: filter missing argument
# error: TypeError
# message: "filter expected 2 arguments, got 1"
filter(None)
# ---
# case: filter extra argument
# error: TypeError
# message: "filter expected 2 arguments, got 3"
filter(None, (), ())
# ---
# case: filter keyword argument
# error: TypeError
# message: "filter() takes no keyword arguments"
filter(None, iterable=())
# ---
# case: filter non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
filter(None, 1)
# ---
# case: filter non-callable predicate
# error: TypeError
# message: "'int' object is not callable"
next(filter(1, (2,)))
# ---
# case: filter predicate failure
# error: ValueError
# message: "filter predicate failed"
def failing_filter(value):
    raise ValueError('filter predicate failed')

next(filter(failing_filter, (1,)))
# ---
# case: filter truth failure
# error: ValueError
# message: "filter truth failed"
class FailingFilterTruth:
    def __bool__(self):
        raise ValueError('filter truth failed')

def failing_filter_truth(value):
    return FailingFilterTruth()

next(filter(failing_filter_truth, (1,)))
# ---
# case: list append missing argument
# error: TypeError
# message: "list.append() takes exactly one argument (0 given)"
[].append()
# ---
# case: list append extra argument
# error: TypeError
# message: "list.append() takes exactly one argument (2 given)"
[].append(1, 2)
# ---
# case: list append keyword argument
# error: TypeError
# message: "list.append() takes no keyword arguments"
[].append(object=1)
# ---
# case: list pop empty list
# error: IndexError
# message: "pop from empty list"
[].pop()
# ---
# case: list pop index out of range
# error: IndexError
# message: "pop index out of range"
[1].pop(2)
# ---
# case: list pop non-integer index
# error: TypeError
# message: "'str' object cannot be interpreted as an integer"
[1].pop('index')
# ---
# case: list pop oversized index
# error: OverflowError
# message: "Python int too large to convert to C ssize_t"
[1].pop(100000000000000000000000000000000000000000000000000)
# ---
# case: list pop extra argument
# error: TypeError
# message: "pop expected at most 1 argument, got 2"
[1].pop(0, 1)
# ---
# case: list pop keyword argument
# error: TypeError
# message: "list.pop() takes no keyword arguments"
[1].pop(index=0)
# ---
# case: dictionary pop missing argument
# error: TypeError
# message: "pop expected at least 1 argument, got 0"
{}.pop()
# ---
# case: dictionary pop missing key
# error: KeyError
# message: "'missing'"
{}.pop('missing')
# ---
# case: dictionary pop extra argument
# error: TypeError
# message: "pop expected at most 2 arguments, got 3"
{}.pop('missing', None, None)
# ---
# case: dictionary pop keyword argument
# error: TypeError
# message: "dict.pop() takes no keyword arguments"
{}.pop(key='missing')
# ---
# case: list extend missing argument
# error: TypeError
# message: "list.extend() takes exactly one argument (0 given)"
[].extend()
# ---
# case: list extend extra argument
# error: TypeError
# message: "list.extend() takes exactly one argument (2 given)"
[].extend((), ())
# ---
# case: list extend keyword argument
# error: TypeError
# message: "list.extend() takes no keyword arguments"
[].extend(iterable=())
# ---
# case: list extend non-iterable value
# error: TypeError
# message: "'int' object is not iterable"
[].extend(1)
# ---
# case: abs missing argument
# error: TypeError
# message: "abs() takes exactly one argument (0 given)"
abs()
# ---
# case: abs extra argument
# error: TypeError
# message: "abs() takes exactly one argument (2 given)"
abs(1, 2)
# ---
# case: abs keyword argument
# error: TypeError
# message: "abs() takes no keyword arguments"
abs(value=1)
# ---
# case: abs unsupported type
# error: TypeError
# message: "bad operand type for abs(): 'str'"
abs('value')
# ---
# case: abs user method failure
# error: ValueError
# message: "abs failed"
class FailingAbsolute:
    def __abs__(self):
        raise ValueError('abs failed')

abs(FailingAbsolute())
# ---
# case: dictionary get missing argument
# error: TypeError
# message: "get expected at least 1 argument, got 0"
{}.get()
# ---
# case: dictionary get extra argument
# error: TypeError
# message: "get expected at most 2 arguments, got 3"
{}.get('missing', None, None)
# ---
# case: dictionary get keyword argument
# error: TypeError
# message: "dict.get() takes no keyword arguments"
{}.get(key='missing')
# ---
# case: dictionary get unhashable key
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
{}.get([])
# ---
# case: dictionary items argument
# error: TypeError
# message: "dict.items() takes no arguments (1 given)"
{}.items(1)
# ---
# case: dictionary items keyword argument
# error: TypeError
# message: "dict.items() takes no keyword arguments"
{}.items(value=1)
# ---
# case: dictionary items iterator key mutation
# error: RuntimeError
# message: "dictionary changed size during iteration"
values = {'first': 1}
items = iter(values.items())
values['second'] = 2
next(items)
# ---
# case: dictionary items iterator replaced key set
# error: RuntimeError
# message: "dictionary keys changed during iteration"
values = {'first': 1, 'second': 2}
items = iter(values.items())
del values['first']
values['third'] = 3
next(items)
# ---
# case: set add missing argument
# error: TypeError
# message: "set.add() takes exactly one argument (0 given)"
set().add()
# ---
# case: set add extra argument
# error: TypeError
# message: "set.add() takes exactly one argument (2 given)"
set().add(1, 2)
# ---
# case: set add keyword argument
# error: TypeError
# message: "set.add() takes no keyword arguments"
set().add(element=1)
# ---
# case: set add unhashable value
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
set().add([])
# ---
# case: set discard missing argument
# error: TypeError
# message: "set.discard() takes exactly one argument (0 given)"
set().discard()
# ---
# case: set discard extra argument
# error: TypeError
# message: "set.discard() takes exactly one argument (2 given)"
set().discard(1, 2)
# ---
# case: set discard keyword argument
# error: TypeError
# message: "set.discard() takes no keyword arguments"
set().discard(element=1)
# ---
# case: set discard unhashable value
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
set().discard([])
# ---
# case: list remove missing argument
# error: TypeError
# message: "list.remove() takes exactly one argument (0 given)"
[].remove()
# ---
# case: list remove extra argument
# error: TypeError
# message: "list.remove() takes exactly one argument (2 given)"
[].remove(1, 2)
# ---
# case: list remove keyword argument
# error: TypeError
# message: "list.remove() takes no keyword arguments"
[].remove(value=1)
# ---
# case: list remove missing value
# error: ValueError
# message: "list.remove(x): x not in list"
[1, 2].remove(3)
# ---
# case: list remove equality failure
# error: ValueError
# message: "remove equality failed"
class FailingRemoveEquality:
    def __eq__(self, other):
        raise ValueError('remove equality failed')

[FailingRemoveEquality()].remove(1)
# ---
# case: list remove equality truth failure
# error: ValueError
# message: "remove truth failed"
class FailingRemoveTruth:
    def __bool__(self):
        raise ValueError('remove truth failed')

class RemoveTruthFailureValue:
    def __eq__(self, other):
        return FailingRemoveTruth()

[RemoveTruthFailureValue()].remove(1)
# ---
# case: round missing number
# error: TypeError
# message: "round() missing required argument 'number' (pos 1)"
round()
# ---
# case: round extra argument
# error: TypeError
# message: "round() takes at most 2 arguments (3 given)"
round(1, 2, 3)
# ---
# case: round unknown keyword
# error: TypeError
# message: "'unknown' is an invalid keyword argument for round()"
round(1, unknown=2)
# ---
# case: round duplicate number
# error: TypeError
# message: "round() got multiple values for argument 'number'"
round(1, number=2)
# ---
# case: round unsupported value
# error: TypeError
# message: "type str doesn't define __round__ method"
round('value')
# ---
# case: round invalid digits
# error: TypeError
# message: "'str' object cannot be interpreted as an integer"
round(1.5, 'digits')
# ---
# case: round infinite float without digits
# error: OverflowError
# message: "cannot convert float infinity to integer"
round(1e400)
# ---
# case: round user failure
# error: ValueError
# message: "round failed"
class FailingRound:
    def __round__(self, ndigits=None):
        raise ValueError('round failed')

round(FailingRound(), 2)
# ---
# case: round non-string keyword
# error: TypeError
# message: "keywords must be strings"
round(**{1: 2})
# ---
# case: dictionary keys argument
# error: TypeError
# message: "dict.keys() takes no arguments (1 given)"
{}.keys(1)
# ---
# case: dictionary keys keyword argument
# error: TypeError
# message: "dict.keys() takes no keyword arguments"
{}.keys(value=1)
# ---
# case: dictionary keys iterator key mutation
# error: RuntimeError
# message: "dictionary changed size during iteration"
values = {'first': 1}
keys = iter(values.keys())
values['second'] = 2
next(keys)
# ---
# case: dictionary keys iterator replaced key set
# error: RuntimeError
# message: "dictionary keys changed during iteration"
values = {'first': 1, 'second': 2}
keys = iter(values.keys())
del values['first']
values['third'] = 3
next(keys)
# ---
# case: string join missing iterable
# error: TypeError
# message: "str.join() takes exactly one argument (0 given)"
''.join()
# ---
# case: string join extra argument
# error: TypeError
# message: "str.join() takes exactly one argument (2 given)"
''.join((), ())
# ---
# case: string join keyword argument
# error: TypeError
# message: "str.join() takes no keyword arguments"
''.join(iterable=())
# ---
# case: string join non-iterable
# error: TypeError
# message: "can only join an iterable"
''.join(1)
# ---
# case: string join non-string item
# error: TypeError
# message: "sequence item 1: expected str instance, int found"
','.join(('first', 2, 'third'))
# ---
# case: string join iterator failure
# error: ValueError
# message: "join iteration failed"
def failing_join_values():
    yield 'kept'
    raise ValueError('join iteration failed')

','.join(failing_join_values())
# ---
# case: string startswith missing prefix
# error: TypeError
# message: "startswith() takes at least 1 argument (0 given)"
''.startswith()
# ---
# case: string startswith extra argument
# error: TypeError
# message: "startswith() takes at most 3 arguments (4 given)"
''.startswith('', 0, 0, 0)
# ---
# case: string startswith keyword argument
# error: TypeError
# message: "startswith() takes no keyword arguments"
''.startswith(prefix='')
# ---
# case: string startswith invalid prefix
# error: TypeError
# message: "startswith first arg must be str or a tuple of str, not int"
''.startswith(1)
# ---
# case: string startswith invalid tuple prefix
# error: TypeError
# message: "tuple for startswith must only contain str, not int"
'abc'.startswith(('missing', 1))
# ---
# case: string startswith invalid bound
# error: TypeError
# message: "slice indices must be integers or None or have an __index__ method"
'abc'.startswith('a', 'start')
# ---
# case: string split extra argument
# error: TypeError
# message: "split() takes at most 2 arguments (3 given)"
'a'.split(None, -1, None)
# ---
# case: string split unknown keyword
# error: TypeError
# message: "'unknown' is an invalid keyword argument for split()"
'a'.split(unknown=None)
# ---
# case: string split duplicate separator
# error: TypeError
# message: "split() got multiple values for argument 'sep'"
'a'.split('.', sep='.')
# ---
# case: string split invalid separator
# error: TypeError
# message: "must be str or None, not int"
'a'.split(1)
# ---
# case: string split empty separator
# error: ValueError
# message: "empty separator"
'a'.split('')
# ---
# case: string split invalid maximum
# error: TypeError
# message: "'str' object cannot be interpreted as an integer"
'a'.split(None, 'maximum')
# ---
# case: string strip extra argument
# error: TypeError
# message: "strip expected at most 1 argument, got 2"
'a'.strip(None, None)
# ---
# case: string strip keyword argument
# error: TypeError
# message: "str.strip() takes no keyword arguments"
'a'.strip(chars='a')
# ---
# case: string strip invalid characters
# error: TypeError
# message: "strip arg must be None or str"
'a'.strip(1)
# ---
# case: string endswith missing suffix
# error: TypeError
# message: "endswith() takes at least 1 argument (0 given)"
''.endswith()
# ---
# case: string endswith extra argument
# error: TypeError
# message: "endswith() takes at most 3 arguments (4 given)"
''.endswith('', 0, 0, 0)
# ---
# case: string endswith keyword argument
# error: TypeError
# message: "endswith() takes no keyword arguments"
''.endswith(suffix='')
# ---
# case: string endswith invalid suffix
# error: TypeError
# message: "endswith first arg must be str or a tuple of str, not int"
''.endswith(1)
# ---
# case: string endswith invalid tuple suffix
# error: TypeError
# message: "tuple for endswith must only contain str, not int"
'abc'.endswith(('missing', 1))
# ---
# case: string endswith invalid bound
# error: TypeError
# message: "slice indices must be integers or None or have an __index__ method"
'abc'.endswith('c', 'start')
# ---
# case: string lower extra argument
# error: TypeError
# message: "str.lower() takes no arguments (1 given)"
'A'.lower(1)
# ---
# case: string lower keyword argument
# error: TypeError
# message: "str.lower() takes no keyword arguments"
'A'.lower(value=1)
# ---
# case: string splitlines extra argument
# error: TypeError
# message: "splitlines() takes at most 1 argument (2 given)"
'a'.splitlines(False, False)
# ---
# case: string splitlines unknown keyword
# error: TypeError
# message: "'unknown' is an invalid keyword argument for splitlines()"
'a'.splitlines(unknown=False)
# ---
# case: string splitlines duplicate keepends
# error: TypeError
# message: "splitlines() got multiple values for argument 'keepends'"
'a'.splitlines(False, keepends=False)
# ---
# case: string splitlines invalid truth result
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidKeepLineEnds:
    def __bool__(self):
        return 1

'a'.splitlines(InvalidKeepLineEnds())
# ---
# case: string format missing automatic value
# error: IndexError
# message: "Replacement index 0 out of range for positional args tuple"
'{}'.format()
# ---
# case: string format unmatched opening brace
# error: ValueError
# message: "Single '{' encountered in format string"
'{'.format(1)
# ---
# case: string format unmatched closing brace
# error: ValueError
# message: "Single '}' encountered in format string"
'}'.format()
# ---
# case: string format unknown conversion
# error: ValueError
# message: "Unknown conversion specifier x"
'{!x}'.format(1)
# ---
# case: string format numbered field boundary
# error: NotImplementedError
# message: "numbered str.format fields are not supported"
'{0}'.format('value')
# ---
# case: string format named field boundary
# error: NotImplementedError
# message: "named str.format fields are not supported"
'{value}'.format(value='text')
# ---
# case: string format specification boundary
# error: NotImplementedError
# message: "str.format specifications are not supported"
'{:>5}'.format('text')
# ---
# case: string format custom method boundary
# error: NotImplementedError
# message: "custom __format__ methods are not supported by str.format"
class CustomFormat:
    def __format__(self, spec):
        return 'custom'

'{}'.format(CustomFormat())
# ---
# case: string replace missing arguments
# error: TypeError
# message: "replace expected at least 2 arguments, got 1"
'a'.replace('a')
# ---
# case: string replace extra argument
# error: TypeError
# message: "replace expected at most 3 arguments, got 4"
'a'.replace('a', 'b', 1, 2)
# ---
# case: string replace invalid old value
# error: TypeError
# message: "replace() argument 1 must be str, not int"
'a'.replace(1, 'b')
# ---
# case: string replace invalid new value
# error: TypeError
# message: "replace() argument 2 must be str, not int"
'a'.replace('a', 1)
# ---
# case: string replace invalid count
# error: TypeError
# message: "'str' object cannot be interpreted as an integer"
'a'.replace('a', 'b', 'one')
# ---
# case: string replace count overflow
# error: OverflowError
# message: "Python int too large to convert to C ssize_t"
'a'.replace('a', 'b', 999999999999999999999999999)
# ---
# case: string removeprefix missing argument
# error: TypeError
# message: "str.removeprefix() takes exactly one argument (0 given)"
'a'.removeprefix()
# ---
# case: string removeprefix extra argument
# error: TypeError
# message: "str.removeprefix() takes exactly one argument (2 given)"
'a'.removeprefix('a', 'b')
# ---
# case: string removeprefix invalid prefix
# error: TypeError
# message: "removeprefix() argument must be str, not int"
'a'.removeprefix(1)
# ---
# case: string removeprefix keyword argument
# error: TypeError
# message: "str.removeprefix() takes no keyword arguments"
'a'.removeprefix(prefix='a')
# ---
# case: string count missing substring
# error: TypeError
# message: "count() takes at least 1 argument (0 given)"
'a'.count()
# ---
# case: string count extra argument
# error: TypeError
# message: "count() takes at most 3 arguments (4 given)"
'a'.count('a', 0, 1, 2)
# ---
# case: string count invalid substring
# error: TypeError
# message: "must be str, not int"
'a'.count(1)
# ---
# case: string count invalid bound
# error: TypeError
# message: "slice indices must be integers or None or have an __index__ method"
'a'.count('a', 'start')
# ---
# case: string count keyword argument
# error: TypeError
# message: "count() takes no keyword arguments"
'a'.count(sub='a')
