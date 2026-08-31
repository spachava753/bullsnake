# Runtime execution cases for user-defined object protocols.
# case: user truth protocols
class Decision:
    def __init__(self, value):
        self.value = value
        self.calls = 0
    def __bool__(self):
        self.calls = self.calls + 1
        return self.value

accepted = Decision(True)
rejected = Decision(False)
if accepted:
    branch = 'accepted'
else:
    branch = missing
if rejected:
    missing
else:
    other_branch = 'rejected'
negated = not rejected
selected_and = accepted and 'right'
selected_or = rejected or 'fallback'
conditional = 10 if accepted else missing
assert accepted

class Sized:
    def __init__(self, size):
        self.size = size
    def __len__(self):
        return self.size

empty = Sized(0)
nonempty = Sized(3)
empty_result = not empty
if nonempty:
    sized_branch = True
else:
    sized_branch = missing

class Both:
    def __bool__(self):
        return False
    def __len__(self):
        return 4

precedence = not Both()

class BaseTruth:
    def __bool__(self):
        return False

class ChildTruth(BaseTruth):
    pass

inherited = not ChildTruth()

class Ordinary:
    pass

def false_function():
    return False

ordinary = Ordinary()
ordinary.__bool__ = false_function
instance_attribute_ignored = ordinary and True

assert branch == 'accepted'
assert other_branch == 'rejected'
assert negated is True
assert selected_and == 'right'
assert selected_or == 'fallback'
assert conditional == 10
assert accepted.calls == 4
assert rejected.calls == 3
assert empty_result is True
assert sized_branch is True
assert precedence is True
assert inherited is True
assert instance_attribute_ignored is True
# ---
# case: truth method exceptions are catchable
class BrokenTruth:
    def __bool__(self):
        raise ValueError('truth failed')

try:
    if BrokenTruth():
        missing
except ValueError as error:
    caught = f'{error!r}'

assert caught == 'ValueError("truth failed")'
# ---
# case: user iteration protocols
class CounterIterator:
    def __init__(self, stop):
        self.current = 0
        self.stop = stop
        self.iter_calls = 0
    def __iter__(self):
        self.iter_calls = self.iter_calls + 1
        return self
    def __next__(self):
        if self.current >= self.stop:
            raise StopIteration(self.current)
        self.current = self.current + 1
        return self.current

iterator = CounterIterator(3)
total = 0
for item in iterator:
    total = total + item
else:
    completed = True

class Values:
    def __init__(self, stop):
        self.stop = stop
    def __iter__(self):
        return CounterIterator(self.stop)

nested_total = 0
for outer in Values(2):
    for inner in Values(2):
        nested_total = nested_total + outer * 10 + inner

class BaseIterator:
    def __iter__(self):
        return self
    def __next__(self):
        raise StopIteration

class InheritedIterator(BaseIterator):
    pass

for absent in InheritedIterator():
    missing
else:
    inherited_complete = True

assert total == 6
assert completed is True
assert iterator.iter_calls == 1
assert nested_total == 66
assert inherited_complete is True
# ---
# case: builtin next with user iterators
class ManualIterator:
    def __init__(self):
        self.position = 0
    def __next__(self):
        self.position = self.position + 1
        if self.position > 2:
            raise StopIteration(42)
        return self.position * 10

manual = ManualIterator()
first = next(manual)
second = next(manual)
default = []
defaulted = next(manual, default)
same_default = defaulted is default
try:
    next(manual)
except StopIteration as error:
    exhausted_value = error.value

assert first == 10
assert second == 20
assert same_default is True
assert exhausted_value == 42, f'exhausted={exhausted_value!r}'
# ---
# case: iterator exception boundaries
class End(StopIteration):
    pass

class SubclassEndingIterator:
    def __iter__(self):
        return self
    def __next__(self):
        raise End(7)

for absent in SubclassEndingIterator():
    missing
else:
    subclass_complete = True

class OneIterator:
    def __init__(self):
        self.done = False
    def __iter__(self):
        return self
    def __next__(self):
        if self.done:
            raise StopIteration
        self.done = True
        return 1

try:
    for item in OneIterator():
        raise StopIteration('body failure')
except StopIteration as error:
    body_error = error.value

class FailingIterator:
    def __iter__(self):
        return self
    def __next__(self):
        raise ValueError('next failed')

try:
    for item in FailingIterator():
        missing
except ValueError as error:
    next_error = f'{error!r}'

assert subclass_complete is True, f'subclass_complete={subclass_complete!r}'
assert body_error == 'body failure', f'body_error={body_error!r}'
assert next_error == 'ValueError("next failed")', f'next_error={next_error!r}'
# ---
# case: user containment protocols
class Bag:
    def __init__(self, value):
        self.value = value
        self.calls = 0
    def __contains__(self, needle):
        self.calls = self.calls + 1
        return needle == self.value

bag = Bag(3)
present = 3 in bag
absent = 4 in bag
not_present = 4 not in bag

class TruthResult:
    def __init__(self, value):
        self.value = value
        self.calls = 0
    def __bool__(self):
        self.calls = self.calls + 1
        return self.value

class DeferredBag:
    def __init__(self, result):
        self.result = result
    def __contains__(self, needle):
        return self.result

truth_result = TruthResult(True)
deferred = 'item' in DeferredBag(truth_result)

class BaseBag:
    def __contains__(self, needle):
        return needle == 'base'

class ChildBag(BaseBag):
    pass

inherited = 'base' in ChildBag()

class ContainsBeforeIter:
    def __contains__(self, needle):
        return False
    def __iter__(self):
        raise ValueError('iteration should not run')

precedence = 'value' not in ContainsBeforeIter()

assert present is True
assert absent is False
assert not_present is True
assert bag.calls == 3
assert deferred is True
assert truth_result.calls == 1
assert inherited is True
assert precedence is True
# ---
# case: containment exceptions are catchable
class BrokenContainer:
    def __contains__(self, needle):
        raise ValueError('contains failed')

try:
    result = 1 in BrokenContainer()
except ValueError as error:
    contains_error = f'{error!r}'

assert contains_error == 'ValueError("contains failed")'
# ---
# case: user subscription protocols
class Store:
    def __init__(self):
        self.values = {}
        self.gets = 0
        self.sets = 0
        self.deletes = 0
    def __getitem__(self, key):
        self.gets = self.gets + 1
        return self.values[key]
    def __setitem__(self, key, value):
        self.sets = self.sets + 1
        self.values[key] = value
        return 'ignored set result'
    def __delitem__(self, key):
        self.deletes = self.deletes + 1
        del self.values[key]
        return 'ignored delete result'

store = Store()
store['answer'] = 40
initial = store['answer']
store['answer'] += 2
updated = store['answer']
store[(1, 2)] = 'tuple key'
tuple_value = store[(1, 2)]
del store['answer']
missing_after_delete = 'answer' not in store.values

class SliceEcho:
    def __getitem__(self, key):
        return key

slice_value = SliceEcho()[1:4:2]

class BaseStore:
    def __getitem__(self, key):
        return key * 2

class ChildStore(BaseStore):
    pass

inherited = ChildStore()[6]

assert initial == 40
assert updated == 42
assert tuple_value == 'tuple key'
assert f'{slice_value!r}' == 'slice(1, 4, 2)'
assert missing_after_delete is True
assert store.gets == 4
assert store.sets == 3
assert store.deletes == 1
assert inherited == 12
# ---
# case: subscription exceptions are catchable
class BrokenSubscription:
    def __getitem__(self, key):
        raise ValueError('get failed')
    def __setitem__(self, key, value):
        raise ValueError('set failed')
    def __delitem__(self, key):
        raise ValueError('delete failed')

broken = BrokenSubscription()
errors = []
try:
    broken[0]
except ValueError as error:
    errors = [*errors, f'{error!r}']
try:
    broken[0] = 1
except ValueError as error:
    errors = [*errors, f'{error!r}']
try:
    del broken[0]
except ValueError as error:
    errors = [*errors, f'{error!r}']

assert errors[0] == 'ValueError("get failed")'
assert errors[1] == 'ValueError("set failed")'
assert errors[2] == 'ValueError("delete failed")'
# ---
# case: user equality protocols
class EqualValue:
    def __init__(self, value):
        self.value = value
    def __eq__(self, other):
        return self.value == other.value

same = EqualValue(4) == EqualValue(4)
different = EqualValue(4) == EqualValue(5)
implicit_not_equal = EqualValue(4) != EqualValue(5)

class RawEquality:
    def __eq__(self, other):
        return 'raw equality result'
    def __ne__(self, other):
        return 'raw inequality result'

raw_equal = RawEquality() == RawEquality()
raw_not_equal = RawEquality() != RawEquality()

class TruthResultForNe:
    def __init__(self):
        self.calls = 0
    def __bool__(self):
        self.calls = self.calls + 1
        return False

class ImplicitNe:
    def __init__(self, result):
        self.result = result
    def __eq__(self, other):
        return self.result

truth_result_for_ne = TruthResultForNe()
inverted_user_truth = ImplicitNe(truth_result_for_ne) != ImplicitNe(None)

assert same is True
assert different is False
assert implicit_not_equal is True
assert raw_equal == 'raw equality result'
assert raw_not_equal == 'raw inequality result'
assert inverted_user_truth is True
assert truth_result_for_ne.calls == 1
# ---
# case: equality reflection and identity fallback
order = 0
class LeftEquality:
    def __eq__(self, other):
        global order
        order = order * 10 + 1
        return NotImplemented

class RightEquality:
    def __eq__(self, other):
        global order
        order = order * 10 + 2
        return True

reflected = LeftEquality() == RightEquality()
reflected_order = order

class BaseEquality:
    def __eq__(self, other):
        return 'base equality'

class ChildEquality(BaseEquality):
    def __eq__(self, other):
        return 'child equality'

subclass_first = BaseEquality() == ChildEquality()

class DeclinesEquality:
    def __eq__(self, other):
        return NotImplemented

same_object = DeclinesEquality()
identity_equal = same_object == same_object
identity_unequal = DeclinesEquality() != DeclinesEquality()
not_implemented_repr = f'{NotImplemented!r}'

class InstanceOnlyEquality:
    pass

def always_equal(other):
    return True

first_instance = InstanceOnlyEquality()
second_instance = InstanceOnlyEquality()
first_instance.__eq__ = always_equal
instance_attribute_ignored = first_instance == second_instance

assert reflected is True
assert reflected_order == 12
assert subclass_first == 'child equality'
assert identity_equal is True
assert identity_unequal is True
assert not_implemented_repr == 'NotImplemented'
assert instance_attribute_ignored is False
# ---
# case: equality exceptions are catchable
class BrokenEquality:
    def __eq__(self, other):
        raise ValueError('equality failed')

try:
    result = BrokenEquality() == BrokenEquality()
except ValueError as error:
    equality_error = f'{error!r}'

assert equality_error == 'ValueError("equality failed")'
# ---
# case: user ordering protocols
class OrderedValue:
    def __init__(self, value):
        self.value = value
    def __lt__(self, other):
        return self.value < other.value
    def __le__(self, other):
        return self.value <= other.value
    def __gt__(self, other):
        return self.value > other.value
    def __ge__(self, other):
        return self.value >= other.value

less = OrderedValue(1) < OrderedValue(2)
less_equal = OrderedValue(2) <= OrderedValue(2)
greater = OrderedValue(3) > OrderedValue(2)
greater_equal = OrderedValue(3) >= OrderedValue(3)

class RawOrdering:
    def __lt__(self, other):
        return 'raw ordering result'

raw_ordering = RawOrdering() < RawOrdering()

class OrderingTruth:
    def __init__(self, value):
        self.value = value
        self.calls = 0
    def __bool__(self):
        self.calls = self.calls + 1
        return self.value

class ChainedOrdering:
    def __init__(self, result):
        self.result = result
    def __lt__(self, other):
        return self.result

first_truth = OrderingTruth(True)
second_truth = OrderingTruth(False)
chained = (
    ChainedOrdering(first_truth)
    < ChainedOrdering(second_truth)
    < ChainedOrdering(OrderingTruth(True))
)

assert less is True
assert less_equal is True
assert greater is True
assert greater_equal is True
assert raw_ordering == 'raw ordering result'
assert chained is second_truth
assert first_truth.calls == 1
assert second_truth.calls == 0
# ---
# case: reflected ordering priority
ordering_order = 0
class LeftOrdering:
    def __lt__(self, other):
        global ordering_order
        ordering_order = ordering_order * 10 + 1
        return NotImplemented

class RightOrdering:
    def __gt__(self, other):
        global ordering_order
        ordering_order = ordering_order * 10 + 2
        return True

reflected_ordering = LeftOrdering() < RightOrdering()
reflected_ordering_order = ordering_order

class OrderingBase:
    def __lt__(self, other):
        return 'base ordering'

class OrderingChild(OrderingBase):
    def __gt__(self, other):
        return 'child reflected ordering'

subclass_ordering = OrderingBase() < OrderingChild()

assert reflected_ordering is True
assert reflected_ordering_order == 12
assert subclass_ordering == 'child reflected ordering'
# ---
# case: ordering exceptions are catchable
class BrokenOrdering:
    def __lt__(self, other):
        raise ValueError('ordering failed')

try:
    result = BrokenOrdering() < BrokenOrdering()
except ValueError as error:
    ordering_error = f'{error!r}'

assert ordering_error == 'ValueError("ordering failed")'
# ---
# case: user unary numeric protocols
class UnaryValue:
    def __init__(self, value):
        self.value = value
    def __pos__(self):
        return ('positive', self.value)
    def __neg__(self):
        return ('negative', self.value)
    def __invert__(self):
        return ('inverted', self.value)

value = UnaryValue(7)
positive = +value
negative = -value
inverted = ~value

class UnaryBase:
    def __neg__(self):
        return 'inherited negative'

class UnaryChild(UnaryBase):
    pass

inherited = -UnaryChild()

assert positive == ('positive', 7)
assert negative == ('negative', 7)
assert inverted == ('inverted', 7)
assert inherited == 'inherited negative'
# ---
# case: unary numeric exceptions are catchable
class BrokenUnary:
    def __neg__(self):
        raise ValueError('unary failed')

try:
    result = -BrokenUnary()
except ValueError as error:
    unary_error = f'{error!r}'

assert unary_error == 'ValueError("unary failed")'
# ---
# case: user binary numeric protocols
class BinaryValue:
    def __add__(self, other):
        return 'add'
    def __sub__(self, other):
        return 'subtract'
    def __mul__(self, other):
        return 'multiply'
    def __matmul__(self, other):
        return 'matrix multiply'
    def __truediv__(self, other):
        return 'divide'
    def __floordiv__(self, other):
        return 'floor divide'
    def __mod__(self, other):
        return 'modulo'
    def __pow__(self, other):
        return 'power'
    def __lshift__(self, other):
        return 'left shift'
    def __rshift__(self, other):
        return 'right shift'
    def __or__(self, other):
        return 'or'
    def __xor__(self, other):
        return 'xor'
    def __and__(self, other):
        return 'and'

binary = BinaryValue()
added = binary + 1
subtracted = binary - 1
multiplied = binary * 1
matrix_multiplied = binary @ 1
divided = binary / 1
floor_divided = binary // 1
modulo = binary % 1
powered = binary ** 1
left_shifted = binary << 1
right_shifted = binary >> 1
ored = binary | 1
xored = binary ^ 1
anded = binary & 1

assert added == 'add'
assert subtracted == 'subtract'
assert multiplied == 'multiply'
assert matrix_multiplied == 'matrix multiply'
assert divided == 'divide'
assert floor_divided == 'floor divide'
assert modulo == 'modulo'
assert powered == 'power'
assert left_shifted == 'left shift'
assert right_shifted == 'right shift'
assert ored == 'or'
assert xored == 'xor'
assert anded == 'and'
# ---
# case: reflected and in-place binary protocols
binary_order = 0
class LeftBinary:
    def __add__(self, other):
        global binary_order
        binary_order = binary_order * 10 + 1
        return NotImplemented

class RightBinary:
    def __radd__(self, other):
        global binary_order
        binary_order = binary_order * 10 + 2
        return 'reflected add'

reflected = LeftBinary() + RightBinary()
reflected_order = binary_order

class BinaryBase:
    def __add__(self, other):
        return 'base add'

class BinaryChild(BinaryBase):
    def __radd__(self, other):
        return 'child reflected add'

subclass_first = BinaryBase() + BinaryChild()

class InPlaceBinary:
    def __iadd__(self, other):
        return ('in-place add', other)

in_place = InPlaceBinary()
in_place += 3

class InPlaceFallback:
    def __iadd__(self, other):
        return NotImplemented
    def __add__(self, other):
        return ('ordinary fallback', other)

in_place_fallback = InPlaceFallback()
in_place_fallback += 4

assert reflected == 'reflected add'
assert reflected_order == 12
assert subclass_first == 'child reflected add'
assert in_place == ('in-place add', 3)
assert in_place_fallback == ('ordinary fallback', 4)
# ---
# case: binary numeric exceptions are catchable
class BrokenBinary:
    def __add__(self, other):
        raise ValueError('binary failed')

try:
    result = BrokenBinary() + 1
except ValueError as error:
    binary_error = f'{error!r}'

assert binary_error == 'ValueError("binary failed")'
# ---
# case: user data descriptors
class DataDescriptor:
    def __init__(self):
        self.gets = 0
        self.sets = 0
        self.deletes = 0
    def __get__(self, instance, owner):
        self.gets = self.gets + 1
        if instance is None:
            return owner
        return instance.saved
    def __set__(self, instance, value):
        self.sets = self.sets + 1
        instance.saved = value
        return 'ignored set result'
    def __delete__(self, instance):
        self.deletes = self.deletes + 1
        del instance.saved
        return 'ignored delete result'

descriptor = DataDescriptor()
class Described:
    field = descriptor

class DescribedChild(Described):
    pass

instance = Described()
instance.field = 42
loaded = instance.field
class_loaded = Described.field
subclass_loaded = DescribedChild.field
del instance.field
try:
    instance.saved
except AttributeError:
    deleted = True

assert loaded == 42
assert class_loaded is Described
assert subclass_loaded is DescribedChild
assert deleted is True
assert descriptor.gets == 3
assert descriptor.sets == 1
assert descriptor.deletes == 1
# ---
# case: non-data descriptor shadowing
class NonDataDescriptor:
    def __init__(self):
        self.gets = 0
    def __get__(self, instance, owner):
        self.gets = self.gets + 1
        return 'descriptor value'

nondata = NonDataDescriptor()
class Shadowable:
    field = nondata

shadowed = Shadowable()
shadowed.field = 'instance value'
instance_value = shadowed.field
class_value = Shadowable.field

class PlainDescriptor:
    pass

plain_descriptor = PlainDescriptor()
def instance_get(instance, owner):
    return 'must not run'
plain_descriptor.__get__ = instance_get
class PlainOwner:
    field = plain_descriptor
instance_special_ignored = PlainOwner().field is plain_descriptor

assert instance_value == 'instance value'
assert class_value == 'descriptor value'
assert nondata.gets == 1
assert instance_special_ignored is True
# ---
# case: descriptor exceptions are catchable
class BrokenDescriptor:
    def __get__(self, instance, owner):
        raise ValueError('descriptor get failed')
    def __set__(self, instance, value):
        raise ValueError('descriptor set failed')
    def __delete__(self, instance):
        raise ValueError('descriptor delete failed')

class BrokenOwner:
    field = BrokenDescriptor()

broken = BrokenOwner()
descriptor_errors = []
try:
    broken.field
except ValueError as error:
    descriptor_errors = [*descriptor_errors, f'{error!r}']
try:
    broken.field = 1
except ValueError as error:
    descriptor_errors = [*descriptor_errors, f'{error!r}']
try:
    del broken.field
except ValueError as error:
    descriptor_errors = [*descriptor_errors, f'{error!r}']

assert descriptor_errors[0] == 'ValueError("descriptor get failed")'
assert descriptor_errors[1] == 'ValueError("descriptor set failed")'
assert descriptor_errors[2] == 'ValueError("descriptor delete failed")'
# ---
# case: property decorators
class ManagedValue:
    def __init__(self, value):
        self._value = value
        self.gets = 0
        self.sets = 0
        self.deletes = 0
    @property
    def value(self):
        self.gets = self.gets + 1
        return self._value
    @value.setter
    def value(self, value):
        self.sets = self.sets + 1
        self._value = value
        return 'ignored setter result'
    @value.deleter
    def value(self):
        self.deletes = self.deletes + 1
        del self._value
        return 'ignored deleter result'

class ManagedChild(ManagedValue):
    pass

descriptor = ManagedValue.value
managed = ManagedChild(10)
initial = managed.value
managed.value = 20
updated = managed.value
del managed.value
try:
    managed.value
except AttributeError:
    missing_after_delete = True

assert ManagedValue.value is descriptor
assert ManagedChild.value is descriptor
assert descriptor.__name__ == 'value'
assert descriptor.fget is not None
assert descriptor.fset is not None
assert descriptor.fdel is not None
assert initial == 10
assert updated == 20
assert missing_after_delete is True
assert managed.gets == 3
assert managed.sets == 1
assert managed.deletes == 1
# ---
# case: property accessor copies
class DirectPropertyOwner:
    def __init__(self):
        self.saved = 3

def direct_getter(instance):
    return instance.saved

def direct_setter(instance, value):
    instance.saved = value

def direct_deleter(instance):
    del instance.saved

read_only = property(direct_getter, doc='direct property')
writable = read_only.setter(direct_setter)
complete = writable.deleter(direct_deleter)
DirectPropertyOwner.value = complete

owner = DirectPropertyOwner()
owner.value = 8
loaded_direct = owner.value
del owner.value

assert read_only is not writable
assert writable is not complete
assert read_only.fget is direct_getter
assert read_only.fset is None
assert writable.fget is direct_getter
assert writable.fset is direct_setter
assert writable.fdel is None
assert complete.fget is direct_getter
assert complete.fset is direct_setter
assert complete.fdel is direct_deleter
assert complete.__doc__ == 'direct property'
assert loaded_direct == 8
