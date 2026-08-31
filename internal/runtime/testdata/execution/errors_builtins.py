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
