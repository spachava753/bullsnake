# Runtime behavior for pure builtins.
# case: lengths of built-in values
assert len('') == 0
assert len('a\u2603') == 2
assert len('\U0001f600') == 1
assert len('\ud800') == 1
assert len(b'\x00abc') == 4
assert len(()) == 0
assert len((1, 2, 3)) == 3
assert len([]) == 0
assert len([1, 2]) == 2
assert len({}) == 0
assert len({'first': 1, 'second': 2}) == 2
assert len({1, 2, 3}) == 3
# ---
# case: user length protocol
class Sized:
    def __init__(self, size):
        self.size = size

    def __len__(self):
        return self.size

value = Sized(4)
value.__len__ = None
assert len(value) == 4

class BooleanLength:
    def __len__(self):
        return True

assert len(BooleanLength()) == 1
# ---
# case: user length exception propagation
class FailingLength:
    def __len__(self):
        raise ValueError('length failed')

caught = None
try:
    len(FailingLength())
except ValueError as exception:
    caught = exception

assert caught is not None
# ---
# case: iter builtin over native values
items = [10, 20]
iterator = iter(items)
assert iter(iterator) is iterator
assert next(iterator) == 10
assert next(iterator) == 20
assert next(iterator, None) is None

text = iter('a\u2603')
assert next(text) == 'a'
assert next(text) == '\u2603'

keys = iter({'first': 1, 'second': 2})
assert next(keys) == 'first'
assert next(keys) == 'second'
# ---
# case: iter builtin over user protocol
class CountingIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 2:
            raise StopIteration
        self.current += 1
        return self.current

class CountingValues:
    def __iter__(self):
        return CountingIterator()

iterator = iter(CountingValues())
assert next(iterator) == 1
assert next(iterator) == 2
assert next(iterator, None) is None

self_iterator = CountingIterator()
assert iter(self_iterator) is self_iterator
# ---
# case: getattr builtin values and defaults
class DynamicField:
    def __get__(self, instance, owner):
        if instance is None:
            return owner
        return instance.value + 1

class Subject:
    class_value = 10
    dynamic = DynamicField()

    def __init__(self, value):
        self.value = value

    def add(self, amount):
        return self.value + amount

subject = Subject(20)
assert getattr(subject, 'value') == 20
assert getattr(subject, 'dynamic') == 21
assert getattr(subject, 'add')(2) == 22
assert getattr(Subject, 'class_value') == 10
assert getattr(Subject, 'dynamic') is Subject
marker = []
assert getattr(subject, 'missing', marker) is marker
assert getattr(Subject, 'missing', marker) is marker
# ---
# case: getattr default catches descriptor attribute error
class MissingField:
    def __get__(self, instance, owner):
        raise AttributeError('hidden field')

class MissingSubject:
    field = MissingField()

marker = []
assert getattr(MissingSubject(), 'field', marker) is marker
# ---
# case: hasattr builtin lookup
class PresentField:
    def __get__(self, instance, owner):
        return None

class HiddenField:
    def __get__(self, instance, owner):
        raise AttributeError('hidden')

class HasSubject:
    class_value = 10
    present = PresentField()
    hidden = HiddenField()

    def __init__(self):
        self.instance_value = 20

subject = HasSubject()
assert hasattr(subject, 'instance_value')
assert hasattr(subject, 'class_value')
assert hasattr(subject, 'present')
assert not hasattr(subject, 'hidden')
assert not hasattr(subject, 'missing')
assert hasattr(HasSubject, 'class_value')
assert not hasattr(HasSubject, 'missing')
# ---
# case: callable instances and builtin
class CallableValue:
    def __init__(self, offset):
        self.offset = offset

    def __call__(self, left, right=1, *, scale=1):
        return (self.offset + left + right) * scale

class InheritedCallable(CallableValue):
    pass

class PlainValue:
    pass

class DisabledCallable:
    __call__ = None

value = CallableValue(10)
value.__call__ = None
assert value(2, 3, scale=2) == 30
assert InheritedCallable(1)(2) == 4
assert callable(value)
assert callable(InheritedCallable(0))
assert callable(CallableValue)
assert callable(callable)
assert not callable(PlainValue())
assert not callable(None)
assert callable(DisabledCallable())
# ---
# case: callable instance return identity
marker = []

class ReturnsMarker:
    def __call__(self):
        return marker

assert ReturnsMarker()() is marker
# ---
# case: class and static method descriptors
class MethodBase:
    @classmethod
    def identify(cls, value):
        return cls, value

    @staticmethod
    def add(left, right):
        return left + right

class MethodChild(MethodBase):
    pass

base_class, base_value = MethodBase.identify(1)
instance_class, instance_value = MethodBase().identify(2)
child_class, child_value = MethodChild.identify(3)
child_instance_class, child_instance_value = MethodChild().identify(4)
assert base_class is MethodBase and base_value == 1
assert instance_class is MethodBase and instance_value == 2
assert child_class is MethodChild and child_value == 3
assert child_instance_class is MethodChild and child_instance_value == 4
assert MethodBase.add(10, 2) == 12
assert MethodBase().add(20, 3) == 23
assert MethodChild.add(30, 4) == 34
# ---
# case: manual method descriptor wrappers
def combine(first, second):
    return first + second

def identify_owner(owner, value):
    return owner

class ManualMethods:
    class_combine = classmethod(identify_owner)
    static_combine = staticmethod(combine)

assert ManualMethods.class_combine(5) is ManualMethods
assert ManualMethods.static_combine(5, 6) == 11
assert ManualMethods().static_combine(7, 8) == 15
static_wrapper = staticmethod(combine)
class_wrapper = classmethod(combine)
assert callable(static_wrapper)
assert not callable(class_wrapper)
assert static_wrapper(9, 10) == 19
assert static_wrapper.__func__ is combine
assert static_wrapper.__wrapped__ is combine
assert class_wrapper.__func__ is combine
assert class_wrapper.__wrapped__ is combine
# ---
# case: bool builtin truth protocol
assert bool() is False
assert bool(None) is False
assert bool(0) is False
assert bool(1) is True
assert bool('') is False
assert bool('value') is True
assert bool([]) is False
assert bool([1]) is True

class BooleanValue:
    def __bool__(self):
        return False

class LengthValue:
    def __len__(self):
        return 2

boolean_value = BooleanValue()
boolean_value.__bool__ = None
assert bool(boolean_value) is False
assert bool(LengthValue()) is True
