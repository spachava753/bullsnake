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
# ---
# case: dictionary keys view
values = {'first': 1, 'second': 2}
keys = values.keys()
assert type(keys).__name__ == 'dict_keys'
assert repr(keys) == "dict_keys(['first', 'second'])"
assert len(keys) == 2
assert bool(keys)
assert list(keys) == ['first', 'second']
values['third'] = 3
assert len(keys) == 3
assert list(keys) == ['first', 'second', 'third']
values['first'] = 10
assert list(keys) == ['first', 'second', 'third']
first = iter(keys)
second = iter(keys)
assert next(first) == 'first'
assert next(second) == 'first'
assert list({}.keys()) == []
assert not {}.keys()
# ---
# case: string join native and retained method
assert ','.join(()) == ''
assert ','.join(('one',)) == 'one'
assert '-'.join(('one', 'two', 'three')) == 'one-two-three'
assert ''.join(('a', '', 'b')) == 'ab'
join = ' | '.join
assert callable(join)
assert join(['left', 'right']) == 'left | right'
assert getattr(':', 'join')(('a', 'b')) == 'a:b'
# ---
# case: string join generators and user iterators
join_steps = 0

def joined_values():
    global join_steps
    join_steps += 1
    yield 'first'
    join_steps += 1
    yield 'second'

assert '/'.join(joined_values()) == 'first/second'
assert join_steps == 2

class JoinedValues:
    def __iter__(self):
        yield 'alpha'
        yield 'beta'

assert '\u2603'.join(JoinedValues()) == 'alpha\u2603beta'
# ---
# case: string startswith prefixes and bounds
text = 'alpha\u2603omega'
assert text.startswith('alpha')
assert text.startswith('\u2603', 5)
assert text.startswith('omega', -5)
assert text.startswith('alpha', None, 5)
assert not text.startswith('omega', 0, -1)
assert text.startswith(('missing', 'alpha'))
assert text.startswith(('alpha', 1))
assert not text.startswith(())
assert 'abc'.startswith('', 3)
assert not 'abc'.startswith('', 4)
assert 'abc'.startswith('bc', 1, 99)
# ---
# case: retained string startswith method
startswith = 'prefix-value'.startswith
assert callable(startswith)
assert startswith('prefix')
assert not startswith('value')
assert getattr('value', 'startswith')('val')
# ---
# case: string split explicit separators
assert 'alpha.beta.gamma'.split('.') == ['alpha', 'beta', 'gamma']
assert 'alpha.beta.gamma'.split('.', 1) == ['alpha', 'beta.gamma']
assert 'alpha.beta.gamma'.split('.', 0) == ['alpha.beta.gamma']
assert 'a--b--'.split('--') == ['a', 'b', '']
assert '\u2603a\u2603b'.split('\u2603') == ['', 'a', 'b']
assert 'a.b'.split(sep='.', maxsplit=1) == ['a', 'b']
# ---
# case: string split whitespace and retained method
assert '  alpha\t beta\n gamma  '.split() == ['alpha', 'beta', 'gamma']
assert '\u00a0alpha\u2003beta\u3000'.split() == ['alpha', 'beta']
assert '  alpha beta  '.split(None, 0) == ['alpha beta  ']
assert '   '.split(None, 0) == []
assert ' a b c '.split(None, 1) == ['a', 'b c ']
split = 'left:right'.split
assert callable(split)
assert split(':') == ['left', 'right']
assert getattr('x y', 'split')() == ['x', 'y']
# ---
# case: string strip whitespace and character sets
text = '  \talpha\u2603  \n'
assert text.strip() == 'alpha\u2603'
assert '\u00a0\u2003value\u3000'.strip() == 'value'
assert 'xyxvalueyxx'.strip('xy') == 'value'
assert '\u2603value\u2603'.strip('\u2603') == 'value'
unchanged = 'value'
assert unchanged.strip() is unchanged
assert unchanged.strip('') is unchanged
# ---
# case: retained string strip method
strip = '  value  '.strip
assert callable(strip)
assert strip() == 'value'
assert getattr('..value..', 'strip')('.') == 'value'
# ---
# case: string endswith suffixes and bounds
text = 'alpha\u2603omega'
assert text.endswith('omega')
assert text.endswith('\u2603', 0, 6)
assert text.endswith('alpha', 0, 5)
assert not text.endswith('alpha', 1, 5)
assert text.endswith(('missing', 'omega'))
assert text.endswith(('omega', 1))
assert not text.endswith(())
assert 'abc'.endswith('', 3)
assert not 'abc'.endswith('', 4)
# ---
# case: retained string endswith method
endswith = 'prefix-value'.endswith
assert callable(endswith)
assert endswith('value')
assert not endswith('prefix')
assert getattr('value', 'endswith')('lue')
# ---
# case: string lower Unicode mappings
assert 'ABC'.lower() == 'abc'
assert 'Stra\u00dfe'.lower() == 'stra\u00dfe'
assert '\u0130'.lower() == 'i\u0307'
assert '\u039f\u03a3'.lower() == '\u03bf\u03c2'
assert '\ud800A'.lower() == '\ud800a'
unchanged = 'already lower'
assert unchanged.lower() is unchanged
# ---
# case: retained string lower method
lower = 'VALUE'.lower
assert callable(lower)
assert lower() == 'value'
assert getattr('Mixed', 'lower')() == 'mixed'
# ---
# case: string splitlines boundaries
text = 'a\r\nb\nc\rd\v e\f f\x1c g\x1d h\x1e i\x85 j\u2028 k\u2029'
assert text.splitlines() == [
    'a', 'b', 'c', 'd', ' e', ' f', ' g', ' h', ' i', ' j', ' k'
]
assert 'a\r\nb\n'.splitlines(True) == ['a\r\n', 'b\n']
assert 'a\r\nb\n'.splitlines(False) == ['a', 'b']
assert ''.splitlines() == []
assert '\n'.splitlines() == ['']
assert '\ud800\nvalue'.splitlines() == ['\ud800', 'value']
# ---
# case: string splitlines truth and retained method
class KeepLineEnds:
    def __init__(self):
        self.calls = 0

    def __bool__(self):
        self.calls += 1
        return True

keep = KeepLineEnds()
splitlines = 'left\nright'.splitlines
assert callable(splitlines)
assert splitlines(keepends=keep) == ['left\n', 'right']
assert keep.calls == 1
assert getattr('one\ntwo', 'splitlines')() == ['one', 'two']
# ---
# case: automatic string format fields
assert '{} not raised by {}'.format('ValueError', 'call') == 'ValueError not raised by call'
assert '{}={!r}'.format('value', [1, 2]) == 'value=[1, 2]'
assert '{{{}}}'.format('inside') == '{inside}'
assert '{!s} {!r}'.format('text', 'text') == "text 'text'"
assert '{!a}'.format('\u2603\ud800') == "'\\u2603\\ud800'"
assert 'literal'.format(unused=1) == 'literal'
# ---
# case: string format user conversion order and retained method
format_events = []

class FormattedValue:
    def __str__(self):
        format_events.append('str')
        return 'text'

    def __repr__(self):
        format_events.append('repr')
        return 'shown'

value = FormattedValue()
formatter = '{} {!r} {}'.format
assert callable(formatter)
assert formatter(value, value, 'done') == 'text shown done'
assert format_events == ['str', 'repr']
assert getattr('{}', 'format')('value') == 'value'
# ---
# case: string replace occurrences and counts
text = 'one one one'
assert text.replace('one', 'two') == 'two two two'
assert text.replace('one', 'two', 2) == 'two two one'
assert text.replace('one', 'two', count=1) == 'two one one'
assert text.replace('one', '') == '  '
assert text.replace('missing', 'value') is text
assert text.replace('one', 'two', 0) is text
assert text.replace('one', 'one') is text
# ---
# case: string replace empty pattern and retained method
assert 'A\u2603'.replace('', '-') == '-A-\u2603-'
assert '\ud800A'.replace('', '-', 2) == '-\ud800-A'
assert ''.replace('', 'value') == 'value'
assert ''.replace('missing', 'value') == ''
replace = 'a/b/c'.replace
assert callable(replace)
assert replace('/', '.', 1) == 'a.b/c'
assert getattr('x-x', 'replace')('x', 'y') == 'y-y'
# ---
# case: string removeprefix values and identity
text = 'prefix-value'
assert text.removeprefix('prefix-') == 'value'
assert '\ud800value'.removeprefix('\ud800') == 'value'
assert text.removeprefix('missing') is text
assert text.removeprefix('') is text
assert ''.removeprefix('prefix') == ''
# ---
# case: retained string removeprefix method
removeprefix = 'prefix-value'.removeprefix
assert callable(removeprefix)
assert removeprefix('prefix-') == 'value'
assert getattr('prefix', 'removeprefix')('pre') == 'fix'
# ---
# case: string count occurrences and bounds
assert 'aaaa'.count('aa') == 2
assert 'one.two.three'.count('.') == 2
assert 'abababa'.count('aba') == 2
assert 'abcabc'.count('a', 1) == 1
assert 'abcabc'.count('a', -3, 99) == 1
assert 'abc'.count('a', 4) == 0
assert 'abc'.count('', 1, 2) == 2
assert 'abc'.count('', 4) == 0
# ---
# case: string count Unicode and retained method
text = '\ud800\u2603\ud800'
assert text.count('\ud800') == 2
assert text.count('', 0, 3) == 4
count = 'a.b.c'.count
assert callable(count)
assert count('.') == 2
assert getattr('banana', 'count')('an') == 2
# ---
# case: setattr instance class and function attributes
class SetattrRecord:
    pass

record = SetattrRecord()
name = 'value'
assert setattr(record, name, 7) is None
assert record.value == 7
assert setattr(SetattrRecord, 'kind', 'entry') is None
assert record.kind == 'entry'

def decorated():
    return 'called'

assert setattr(decorated, 'marker', 11) is None
assert decorated.marker == 11
decorated.direct = 12
assert decorated.direct == 12
setter = setattr
assert callable(setter)
assert setter(record, 'other', 13) is None
assert record.other == 13
# ---
# case: setattr data descriptor
setattr_events = []

class SetattrDescriptor:
    def __set__(self, instance, value):
        setattr_events.append((instance, value))
        instance.stored = value

class SetattrOwner:
    field = SetattrDescriptor()

owner = SetattrOwner()
assert setattr(owner, 'field', 21) is None
assert setattr_events == [(owner, 21)]
assert owner.stored == 21
# ---
# case: delattr instance class and function attributes
class DelattrRecord:
    kind = 'entry'

record = DelattrRecord()
record.value = 7
assert delattr(record, 'value') is None
assert not hasattr(record, 'value')
assert delattr(DelattrRecord, 'kind') is None
assert not hasattr(DelattrRecord, 'kind')

def decorated_for_delete():
    return 'called'

decorated_for_delete.marker = 11
assert delattr(decorated_for_delete, 'marker') is None
assert not hasattr(decorated_for_delete, 'marker')
decorated_for_delete.direct = 12
del decorated_for_delete.direct
assert not hasattr(decorated_for_delete, 'direct')
deleter = delattr
assert callable(deleter)
record.other = 13
assert deleter(record, 'other') is None
assert not hasattr(record, 'other')
# ---
# case: delattr data descriptor
class DelattrDescriptor:
    def __delete__(self, instance):
        instance.deleted = True

class DelattrOwner:
    field = DelattrDescriptor()

owner = DelattrOwner()
assert delattr(owner, 'field') is None
assert owner.deleted is True
# ---
# case: dictionary update mappings and keywords
target = {'a': 1, 'keep': 0}
source = {'b': 2, 'a': 3}
assert target.update(source) is None
assert list(target.items()) == [('a', 3), ('keep', 0), ('b', 2)]
assert target.update(c=4, a=5) is None
assert list(target.items()) == [('a', 5), ('keep', 0), ('b', 2), ('c', 4)]
assert target.update() is None
assert target.update(target) is None
assert list(target.items()) == [('a', 5), ('keep', 0), ('b', 2), ('c', 4)]
# ---
# case: retained dictionary update method
update = {'a': 1}.update
assert callable(update)
assert update({'b': 2}) is None
dictionary = {}
assert getattr(dictionary, 'update')(value=7) is None
assert list(dictionary.items()) == [('value', 7)]
# ---
# case: dictionary clear and refill
values = {'a': 1, 'b': 2}
assert values.clear() is None
assert len(values) == 0
assert list(values.items()) == []
assert values.clear() is None
values['c'] = 3
values['a'] = 4
assert list(values.items()) == [('c', 3), ('a', 4)]
# ---
# case: retained dictionary clear method
values = {'a': 1}
clear = values.clear
assert callable(clear)
assert clear() is None
assert len(values) == 0
values['b'] = 2
assert getattr(values, 'clear')() is None
assert len(values) == 0
# ---
# case: dictionary shallow copy
shared = []
source = {'a': shared, 'b': 2}
copy = source.copy()
assert copy is not source
assert list(copy.items()) == [('a', shared), ('b', 2)]
assert copy['a'] is shared
copy['a'] = 3
copy['c'] = 4
assert list(source.items()) == [('a', shared), ('b', 2)]
assert list(copy.items()) == [('a', 3), ('b', 2), ('c', 4)]
# ---
# case: retained dictionary copy method
copy = {'a': 1}.copy
assert callable(copy)
assert list(copy().items()) == [('a', 1)]
assert list(getattr({'b': 2}, 'copy')().items()) == [('b', 2)]
# ---
# case: live dictionary values view
values = {'a': 1, 'b': 2}
view = values.values()
assert type(view).__name__ == 'dict_values'
assert type(iter(view)).__name__ == 'dict_valueiterator'
assert len(view) == 2
assert bool(view)
assert repr(view) == 'dict_values([1, 2])'
values['a'] = 3
assert list(view) == [3, 2]
values['c'] = 4
assert len(view) == 3
assert list(view) == [3, 2, 4]
values.clear()
assert len(view) == 0
assert not view
# ---
# case: independent retained dictionary values iterators
values = {'a': 1, 'b': 2}
method = values.values
assert callable(method)
view = method()
first = iter(view)
second = iter(view)
assert next(first) == 1
assert next(second) == 1
values['b'] = 5
assert next(first) == 5
assert next(second) == 5
assert list(getattr(values, 'values')()) == [1, 5]
# ---
# case: reversed native sequences
assert list(reversed([1, 2, 3])) == [3, 2, 1]
assert list(reversed((1, 2, 3))) == [3, 2, 1]
assert list(reversed('A\u2603\ud800')) == ['\ud800', '\u2603', 'A']
assert list(reversed(b'ab')) == [98, 97]
assert list(reversed(range(1, 8, 2))) == [7, 5, 3, 1]
assert type(reversed([])).__name__ == 'list_reverseiterator'
assert type(reversed(())).__name__ == 'reversed'
assert type(reversed(range(1))).__name__ == 'range_iterator'
# ---
# case: reversed list mutation and retained builtin
values = [1, 2]
iterator = reversed(values)
values.append(3)
assert next(iterator) == 2
assert next(iterator) == 1
assert next(iterator, 'done') == 'done'
shrinking = [1, 2, 3]
iterator = reversed(shrinking)
shrinking.pop()
assert next(iterator, 'done') == 'done'
reverse = reversed
assert callable(reverse)
assert list(reverse(['a', 'b'])) == ['b', 'a']
# ---
# case: lazy zip values
assert list(zip()) == []
assert list(zip([1, 2, 3], 'ab')) == [(1, 'a'), (2, 'b')]
assert list(zip(range(3), b'ab', (10, 20, 30))) == [
    (0, 97, 10),
    (1, 98, 20),
]
assert type(zip()).__name__ == 'zip'
assert repr(zip()) == '<zip object>'
assert next(zip(), 'done') == 'done'
total = 0
for left, right in zip([1, 2], [10, 20]):
    total += left + right
assert total == 33
# ---
# case: zip iterator composition
class Counter:
    def __init__(self):
        self.value = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.value == 2:
            raise StopIteration
        self.value += 1
        return self.value

def generate():
    yield 4
    yield 5

assert list(zip(Counter(), generate())) == [(1, 4), (2, 5)]
assert list(zip(map(lambda value: value * 2, [1, 2]),
                filter(lambda value: value > 1, [1, 2, 3]))) == [
    (2, 2),
    (4, 3),
]
assert list(enumerate(zip(['a'], ['b']))) == [(0, ('a', 'b'))]
assert list(map(lambda pair: pair[0] + pair[1], zip([1, 2], [10, 20]))) == [
    11,
    22,
]
assert list(filter(lambda pair: pair[0],
                   zip([True, False], [1, 2]))) == [(True, 1)]
# ---
# case: zip shortest input consumption
first = iter([1, 2, 3])
second = iter([10])
iterator = zip(first, second)
assert next(iterator) == (1, 10)
assert next(iterator, 'done') == 'done'
assert next(first) == 3
assert next(second, 'done') == 'done'
zip_type = zip
assert callable(zip_type)
assert list(zip_type(['a'], ['b'])) == [('a', 'b')]
