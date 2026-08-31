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
