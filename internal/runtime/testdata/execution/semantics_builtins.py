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
# ---
# case: repr builtin values and protocol
assert repr(None) == 'None'
assert repr(True) == 'True'
assert repr(123) == '123'
assert repr('value') == "'value'"
assert repr(b'value') == "b'value'"
assert repr([1, 'two']) == "[1, 'two']"

representation = 'custom representation'

class RepresentedValue:
    def __repr__(self):
        return representation

value = RepresentedValue()
value.__repr__ = None
assert repr(value) is representation
# ---
# case: str builtin values and protocol
assert str() == ''
text = 'existing text'
assert str(text) is text
assert str(None) == 'None'
assert str(False) == 'False'
assert str(123) == '123'
assert str(b'value') == "b'value'"
assert str([1, 'two']) == "[1, 'two']"
assert str(ValueError('message')) == 'message'

string_value = 'custom string'

class StringValue:
    def __str__(self):
        return string_value

value = StringValue()
value.__str__ = None
assert str(value) is string_value
# ---
# case: str falls back to repr
representation = 'fallback representation'

class ReprOnlyValue:
    def __repr__(self):
        return representation

assert str(ReprOnlyValue()) is representation
# ---
# case: enumerate native iteration
assert list(enumerate(('first', 'second'))) == [
    (0, 'first'),
    (1, 'second'),
]
assert list(enumerate(['first', 'second'], 5)) == [
    (5, 'first'),
    (6, 'second'),
]
assert list(enumerate(iterable=('value',), start=100)) == [(100, 'value')]
value = enumerate((), 7)
assert type(value) is enumerate
assert isinstance(value, enumerate)
assert iter(value) is value
assert next(value, None) is None
# ---
# case: enumerate large indexes and generators
large = enumerate((10, 20), 100000000000000000000)
assert next(large) == (100000000000000000000, 10)
assert next(large) == (100000000000000000001, 20)

def generated_values():
    yield 'left'
    yield 'right'

assert list(enumerate(generated_values(), -2)) == [
    (-2, 'left'),
    (-1, 'right'),
]
# ---
# case: enumerate user iteration protocol
class EnumeratedValues:
    def __iter__(self):
        yield 4
        yield 8

class EnumeratedIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 2:
            raise StopIteration
        self.current += 1
        return self.current * 10

assert list(enumerate(EnumeratedValues(), 1)) == [(1, 4), (2, 8)]
iterator = enumerate(EnumeratedIterator(), 3)
assert next(iterator) == (3, 10)
assert next(iterator) == (4, 20)
assert next(iterator, 'done') == 'done'

many = list(enumerate(range(5000)))
assert len(many) == 5000
assert many[0] == (0, 0)
assert many[4999] == (4999, 4999)
# ---
# case: all over native values
assert all(()) is True
assert all((1, True, 'value')) is True
assert all((1, 0, 2)) is False
assert all(range(1, 5)) is True
assert all(range(0, 5)) is False
# ---
# case: all short circuits generators
all_steps = 0

def all_values():
    global all_steps
    all_steps += 1
    yield 1
    all_steps += 1
    yield 0
    all_steps += 1
    yield 1

assert all(all_values()) is False
assert all_steps == 2
assert all(value > 0 for value in (1, 2, 3)) is True
# ---
# case: all uses user iteration and truth protocols
class AllTruth:
    def __init__(self, value):
        self.value = value

    def __bool__(self):
        return self.value

class AllValues:
    def __iter__(self):
        yield AllTruth(True)
        yield AllTruth(False)
        yield AllTruth(True)

assert all(AllValues()) is False

class AllIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 2:
            raise StopIteration
        self.current += 1
        return self.current

assert all(AllIterator()) is True
# ---
# case: any over native values
assert any(()) is False
assert any((0, False, '')) is False
assert any((0, '', 3)) is True
assert any(range(0)) is False
assert any(range(0, 3)) is True
# ---
# case: any short circuits generators
any_steps = 0

def any_values():
    global any_steps
    any_steps += 1
    yield 0
    any_steps += 1
    yield 2
    any_steps += 1
    yield 0

assert any(any_values()) is True
assert any_steps == 2
assert any(value < 0 for value in (1, 2, 3)) is False
# ---
# case: any uses user truth protocol
class AnyTruth:
    def __init__(self, value):
        self.value = value

    def __len__(self):
        return self.value

assert any((AnyTruth(0), AnyTruth(2), AnyTruth(0))) is True
# ---
# case: hash fixed values and invariants
assert hash(0) == 0
assert hash(False) == 0
assert hash(1) == 1
assert hash(True) == 1
assert hash(-1) == -2
assert hash(123) == hash(123.0)
assert hash('value') == hash('value')
assert hash(b'value') == hash(b'value')
assert hash((1, 'value', None)) == hash((1, 'value', None))
assert hash(frozenset((1, 2))) == hash(frozenset((2, 1)))
assert hash(range(2, 10, 2)) == hash(range(2, 10, 2))
assert hash(type) == hash(type)
# ---
# case: hash user protocol and tuple use
class HashValue:
    def __hash__(self):
        return 12345

value = HashValue()
value.__hash__ = None
assert hash(value) == 12345

class TupleHashValue:
    def __hash__(self):
        return hash((type(self), 'case'))

first = TupleHashValue()
second = TupleHashValue()
assert hash(first) == hash(second)

class DefaultHashValue:
    pass

plain = DefaultHashValue()
assert hash(plain) == hash(plain)
# ---
# case: map native iterables and builtins
assert list(map(lambda value: value * 2, range(4))) == [0, 2, 4, 6]
assert tuple(map(len, ('a', 'bbb', ''))) == (1, 3, 0)
iterator = map(str, (1, 2))
assert type(iterator) is map
assert isinstance(iterator, map)
assert iter(iterator) is iterator
assert next(iterator) == '1'
assert next(iterator) == '2'
assert next(iterator, None) is None
# ---
# case: map is lazy and calls Python functions
map_steps = 0

def mapped_value(value):
    global map_steps
    map_steps += 1
    return value + 10

iterator = map(mapped_value, (1, 2))
assert map_steps == 0
assert next(iterator) == 11
assert map_steps == 1
assert list(iterator) == [12]
assert map_steps == 2
# ---
# case: map calls classes and consumes generator and user iterators
class MappedItem:
    def __init__(self, value):
        self.value = value

items = list(map(MappedItem, (3, 4)))
assert items[0].value == 3
assert items[1].value == 4

mapped_generator = map(lambda value: value + 1, (value for value in (5, 6)))
assert list(mapped_generator) == [6, 7]

class MappedIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 2:
            raise StopIteration
        self.current += 1
        return self.current

assert list(map(lambda value: value * 3, MappedIterator())) == [3, 6]
# ---
# case: dir current namespace
local_directory_marker = 1
local_names = dir()
assert 'local_directory_marker' in local_names
assert 'local_names' not in local_names
# ---
# case: dir class and instance names
class DirectoryBase:
    base_value = 1

    def base_method(self):
        return None

class DirectoryChild(DirectoryBase):
    child_value = 2

    def __init__(self):
        self.instance_value = 3

class_names = dir(DirectoryChild)
assert '__name__' in class_names
assert '__mro__' in class_names
assert 'base_value' in class_names
assert 'base_method' in class_names
assert 'child_value' in class_names

subject = DirectoryChild()
instance_names = dir(subject)
assert 'instance_value' in instance_names
assert 'base_value' in instance_names
assert 'child_value' in instance_names
assert 'missing_value' not in instance_names
# ---
# case: filter native values and None predicate
assert list(filter(None, (0, 1, '', 'value', False, True))) == [
    1,
    'value',
    True,
]
iterator = filter(None, ())
assert type(iterator) is filter
assert isinstance(iterator, filter)
assert iter(iterator) is iterator
assert next(iterator, None) is None
# ---
# case: filter is lazy and calls Python predicates
filter_steps = 0

def keep_even(value):
    global filter_steps
    filter_steps += 1
    return value % 2 == 0

iterator = filter(keep_even, range(5))
assert filter_steps == 0
assert next(iterator) == 0
assert filter_steps == 1
assert list(iterator) == [2, 4]
assert filter_steps == 5
# ---
# case: filter composes with dir and generator iteration
class FilteredMethods:
    def test_first(self):
        return None

    def helper(self):
        return None

    def test_second(self):
        return None

def keep_test_name(name):
    return name == 'test_first' or name == 'test_second'

method_names = list(filter(keep_test_name, dir(FilteredMethods)))
assert method_names == ['test_first', 'test_second']
assert list(filter(lambda value: value > 1, (value for value in (1, 2, 3)))) == [
    2,
    3,
]
# ---
# case: filter consumes user iterators and truth results
class FilterTruth:
    def __init__(self, value):
        self.value = value

    def __bool__(self):
        return self.value

class FilteredIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 3:
            raise StopIteration
        self.current += 1
        return self.current

def user_filter(value):
    return FilterTruth(value == 2)

assert list(filter(user_filter, FilteredIterator())) == [2]
# ---
# case: list append method
items = []
append_result = items.append('first')
assert append_result is None
assert items == ['first']
append = items.append
assert callable(append)
assert append('second') is None
assert items == ['first', 'second']
assert getattr(items, 'append')('third') is None
assert items == ['first', 'second', 'third']
# ---
# case: list pop method
items = ['first', 'second', 'third']
pop = items.pop
assert callable(pop)
assert pop() == 'third'
assert items == ['first', 'second']
assert items.pop(0) == 'first'
assert items == ['second']
numbers = [1, 2, 3]
assert numbers.pop(-2) == 2
assert numbers == [1, 3]
assert numbers.pop(False) == 1
assert numbers == [3]
# ---
# case: dictionary pop method
values = {'first': 1, 'second': 2, 'third': 3}
pop = values.pop
assert callable(pop)
assert pop('second') == 2
assert len(values) == 2
assert list(values) == ['first', 'third']
assert values['first'] == 1
assert values['third'] == 3
assert 'second' not in values
marker = object()
assert pop('missing', marker) is marker
assert list(values) == ['first', 'third']
assert values.pop('first', 99) == 1
assert list(values) == ['third']
# ---
# case: list extend method
items = [0]
extend = items.extend
assert callable(extend)
assert extend((1, 2)) is None
assert items == [0, 1, 2]
assert items.extend('ab') is None
assert items == [0, 1, 2, 'a', 'b']
items.extend(value for value in (3, 4))
assert items == [0, 1, 2, 'a', 'b', 3, 4]
copy = [5, 6]
copy.extend(copy)
assert copy == [5, 6, 5, 6]
# ---
# case: list extend consumes user iterators
class ExtendIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 3:
            raise StopIteration
        self.current += 1
        return self.current

items = []
assert items.extend(ExtendIterator()) is None
assert items == [1, 2, 3]
# ---
# case: list extend keeps values before iterator failure
def failing_extension():
    yield 'kept'
    raise ValueError('extend failed')

items = ['start']
try:
    items.extend(failing_extension())
except ValueError as error:
    assert str(error) == 'extend failed'
else:
    assert False
assert items == ['start', 'kept']
# ---
# case: abs builtin and user protocol
assert abs(-5) == 5
assert abs(True) == 1
assert type(abs(True)) is int
assert abs(-2.5) == 2.5
assert abs(-4j) == 4.0
marker = object()

class AbsoluteValue:
    def __abs__(self):
        return marker

assert abs(AbsoluteValue()) is marker
# ---
# case: dictionary get method
marker = object()
values = {'present': marker, 'other': 2}
get = values.get
assert callable(get)
assert get('present') is marker
assert get('missing') is None
assert get('missing', marker) is marker
assert list(values) == ['present', 'other']
# ---
# case: dictionary items view
values = {'first': 1, 'second': 2}
items = values.items()
assert type(items).__name__ == 'dict_items'
assert len(items) == 2
assert bool(items)
assert list(items) == [('first', 1), ('second', 2)]
values['third'] = 3
assert len(items) == 3
assert list(items) == [('first', 1), ('second', 2), ('third', 3)]
first = iter(items)
second = iter(items)
values['first'] = 10
assert next(first) == ('first', 10)
assert next(second) == ('first', 10)
assert list({}.items()) == []
assert not {}.items()
# ---
# case: set add method
values = set()
add = values.add
assert callable(add)
assert add('first') is None
assert add('second') is None
assert add('first') is None
assert len(values) == 2
assert 'first' in values
assert 'second' in values
assert list(values) == ['first', 'second']
# ---
# case: set discard method
values = {'first', 'second'}
discard = values.discard
assert callable(discard)
assert discard('first') is None
assert 'first' not in values
assert list(values) == ['second']
assert discard('missing') is None
assert list(values) == ['second']
# ---
# case: list remove method
items = [1, 2, 1, 3]
remove = items.remove
assert callable(remove)
assert remove(1) is None
assert items == [2, 1, 3]
assert getattr(items, 'remove')(3) is None
assert items == [2, 1]
# ---
# case: list remove uses identity and user equality truth
class IdentityOnly:
    def __eq__(self, other):
        raise AssertionError('identity comparison called equality')

identity = IdentityOnly()
items = [identity]
assert items.remove(identity) is None
assert items == []

comparison_values = []
truth_values = []

class RemoveTruth:
    def __init__(self, value):
        self.value = value

    def __bool__(self):
        truth_values.append(self.value)
        return self.value

class RemovableValue:
    def __init__(self, value):
        self.value = value

    def __eq__(self, other):
        comparison_values.append(self.value)
        return RemoveTruth(self.value == other)

first = RemovableValue(1)
second = RemovableValue(2)
third = RemovableValue(3)
items = [first, second, third]
assert items.remove(2) is None
assert comparison_values == [1, 2]
assert truth_values == [False, True]
assert len(items) == 2
assert items[0] is first
assert items[1] is third
# ---
# case: round native numbers
assert round(2.5) == 2
assert type(round(2.5)) is int
assert round(3.5) == 4
assert round(-2.5) == -2
assert round(True) == 1
assert type(round(True)) is int
assert round(25, -1) == 20
assert round(35, -1) == 40
assert round(123456789012345678901234567890, -5) == 123456789012345678901234600000
assert round(123, 5) == 123
assert round(2.675, 2) == 2.67
assert round(1.25, 1) == 1.2
assert round(1.35, 1) == 1.4
assert round(-0.04, 1) == 0.0
# ---
# case: round keywords and user protocol
assert round(number=12.345, ndigits=2) == 12.35
assert round(12.345, ndigits=1) == 12.3
round_calls = []

class RoundValue:
    def __round__(self, ndigits=None):
        round_calls.append(ndigits)
        return ('rounded', ndigits)

value = RoundValue()
assert round(value) == ('rounded', None)
assert round(value, None) == ('rounded', None)
assert round(value, 3) == ('rounded', 3)
assert round_calls == [None, None, 3]
