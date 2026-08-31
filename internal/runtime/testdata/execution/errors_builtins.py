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
