# Runtime exception cases for user-defined object protocols.
# case: non-boolean bool result
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidBool:
    def __bool__(self):
        return 1

if InvalidBool():
    pass
# ---
# case: disabled bool method
# error: TypeError
# message: "'DisabledBool' cannot be interpreted as a boolean"
class DisabledBool:
    __bool__ = None

if DisabledBool():
    pass
# ---
# case: non-integer length result
# error: TypeError
# message: "'float' object cannot be interpreted as an integer"
class FloatLength:
    def __len__(self):
        return 1.5

if FloatLength():
    pass
# ---
# case: negative length result
# error: ValueError
# message: "__len__() should return >= 0"
class NegativeLength:
    def __len__(self):
        return -1

if NegativeLength():
    pass
# ---
# case: oversized length result
# error: OverflowError
# message: "cannot fit 'int' into an index-sized integer"
class HugeLength:
    def __len__(self):
        return 100000000000000000000000000000000000000000000000000000000

if HugeLength():
    pass
# ---
# case: instance-only iter method
# error: TypeError
# message: "'InstanceOnlyIterable' object is not iterable"
class InstanceOnlyIterable:
    pass

def make_iterator():
    return ()

value = InstanceOnlyIterable()
value.__iter__ = make_iterator
for item in value:
    pass
# ---
# case: disabled iter method
# error: TypeError
# message: "'DisabledIterable' object is not iterable"
class DisabledIterable:
    __iter__ = None

for item in DisabledIterable():
    pass
# ---
# case: invalid iter result
# error: TypeError
# message: "iter() returned non-iterator of type 'list'"
class InvalidIterable:
    def __iter__(self):
        return []

for item in InvalidIterable():
    pass
# ---
# case: disabled next result
# error: TypeError
# message: "iter() returned non-iterator of type 'DisabledNext'"
class DisabledNext:
    def __iter__(self):
        return self
    __next__ = None

for item in DisabledNext():
    pass
# ---
# case: noncallable next method
# error: TypeError
# message: "'int' object is not callable"
class NoncallableNext:
    def __iter__(self):
        return self
    __next__ = 1

for item in NoncallableNext():
    pass
# ---
# case: disabled contains method
# error: TypeError
# message: "'DisabledContainer' object is not a container"
class DisabledContainer:
    __contains__ = None

result = 1 in DisabledContainer()
# ---
# case: noncallable contains method
# error: TypeError
# message: "'int' object is not callable"
class NoncallableContainer:
    __contains__ = 1

result = 1 in NoncallableContainer()
# ---
# case: invalid contains truth result
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidTruth:
    def __bool__(self):
        return 1

class InvalidResultContainer:
    def __contains__(self, needle):
        return InvalidTruth()

result = 1 in InvalidResultContainer()
# ---
# case: instance-only getitem method
# error: TypeError
# message: "'InstanceOnlySubscript' object is not subscriptable"
class InstanceOnlySubscript:
    pass

def getitem(key):
    return key

value = InstanceOnlySubscript()
value.__getitem__ = getitem
result = value[1]
# ---
# case: disabled getitem method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledGetitem:
    __getitem__ = None

result = DisabledGetitem()[1]
# ---
# case: disabled setitem method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledSetitem:
    __setitem__ = None

value = DisabledSetitem()
value[1] = 2
# ---
# case: disabled delitem method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledDelitem:
    __delitem__ = None

value = DisabledDelitem()
del value[1]
# ---
# case: disabled equality method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledEquality:
    __eq__ = None

result = DisabledEquality() == DisabledEquality()
# ---
# case: invalid implicit inequality truth
# error: TypeError
# message: "__bool__ should return bool, returned int"
class InvalidInequalityTruth:
    def __bool__(self):
        return 1

class ImplicitInvalidInequality:
    def __eq__(self, other):
        return InvalidInequalityTruth()

result = ImplicitInvalidInequality() != ImplicitInvalidInequality()
# ---
# case: NotImplemented boolean context
# error: TypeError
# message: "NotImplemented should not be used in a boolean context"
if NotImplemented:
    pass
# ---
# case: declined user ordering
# error: TypeError
# message: "'<' not supported between instances of 'DeclinedOrdering' and 'DeclinedOrdering'"
class DeclinedOrdering:
    def __lt__(self, other):
        return NotImplemented
    def __gt__(self, other):
        return NotImplemented

result = DeclinedOrdering() < DeclinedOrdering()
# ---
# case: disabled ordering method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledOrdering:
    __lt__ = None

result = DisabledOrdering() < DisabledOrdering()
# ---
# case: disabled unary method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledUnary:
    __neg__ = None

result = -DisabledUnary()
# ---
# case: instance-only unary method
# error: TypeError
# message: "bad operand type for unary -: 'InstanceOnlyUnary'"
class InstanceOnlyUnary:
    pass

def negate():
    return 1

value = InstanceOnlyUnary()
value.__neg__ = negate
result = -value
# ---
# case: declined user binary operation
# error: TypeError
# message: "unsupported operand type(s) for +: 'DeclinedBinary' and 'DeclinedBinary'"
class DeclinedBinary:
    def __add__(self, other):
        return NotImplemented
    def __radd__(self, other):
        return NotImplemented

result = DeclinedBinary() + DeclinedBinary()
# ---
# case: declined user in-place operation
# error: TypeError
# message: "unsupported operand type(s) for +=: 'DeclinedInPlace' and 'DeclinedInPlace'"
class DeclinedInPlace:
    def __iadd__(self, other):
        return NotImplemented
    def __add__(self, other):
        return NotImplemented
    def __radd__(self, other):
        return NotImplemented

value = DeclinedInPlace()
value += DeclinedInPlace()
# ---
# case: disabled binary method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledBinary:
    __add__ = None

result = DisabledBinary() + 1
# ---
# case: disabled in-place binary method
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledInPlace:
    __iadd__ = None
    def __add__(self, other):
        return 'must not run'

value = DisabledInPlace()
value += 1
# ---
# case: disabled descriptor get
# error: TypeError
# message: "'NoneType' object is not callable"
class DisabledGetDescriptor:
    __get__ = None

class DisabledGetOwner:
    field = DisabledGetDescriptor()

result = DisabledGetOwner().field
# ---
# case: missing descriptor set half
# error: AttributeError
# message: "__set__"
class DeleteOnlyDescriptor:
    def __delete__(self, instance):
        pass

class DeleteOnlyOwner:
    field = DeleteOnlyDescriptor()

value = DeleteOnlyOwner()
value.field = 1
# ---
# case: missing descriptor delete half
# error: AttributeError
# message: "__delete__"
class SetOnlyDescriptor:
    def __set__(self, instance, value):
        pass

class SetOnlyOwner:
    field = SetOnlyDescriptor()

value = SetOnlyOwner()
del value.field
# ---
# case: property without getter
# error: AttributeError
# message: "property 'value' of 'UnreadableProperty' object has no getter"
class UnreadableProperty:
    value = property()

UnreadableProperty().value
# ---
# case: property without setter
# error: AttributeError
# message: "property 'value' of 'ReadOnlyProperty' object has no setter"
class ReadOnlyProperty:
    @property
    def value(self):
        return 1

ReadOnlyProperty().value = 2
# ---
# case: property without deleter
# error: AttributeError
# message: "property 'value' of 'UndeletableProperty' object has no deleter"
class UndeletableProperty:
    @property
    def value(self):
        return 1

value = UndeletableProperty()
del value.value
# ---
# case: noncallable property getter
# error: TypeError
# message: "'int' object is not callable"
class InvalidPropertyGetter:
    value = property(1)

InvalidPropertyGetter().value
