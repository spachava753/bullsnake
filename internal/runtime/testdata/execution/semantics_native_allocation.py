# case: singleton native allocators preserve real singleton identities
for value in (None, Ellipsis, NotImplemented):
    cls = type(value)
    assert cls() is value
    assert cls.__new__(cls) is value
    assert value.__new__(cls) is value
    assert value.__new__ is cls.__new__
assert type(None).__new__ is not object.__new__
try:
    type(None)(1)
    assert False
except TypeError as error:
    assert str(error) == 'NoneType takes no arguments'
try:
    None.__new__(object)
    assert False
except TypeError as error:
    assert str(error) == 'NoneType.__new__(object): object is not a subtype of NoneType'

# ---
# case: object new allocates without calling initialization
calls = []
class Plain:
    pass
class Initialized:
    def __init__(self, value):
        calls.append(value)
first = object.__new__(Plain)
second = Plain.__new__(Plain)
assert type(first) is Plain
assert type(second) is Plain
assert first is not second
assert first.__new__ is object.__new__
created = object.__new__(Initialized, 3)
assert type(created) is Initialized
assert calls == []
class Child(Plain):
    pass
assert type(Child.__new__(Child)) is Child
assert type(object.__new__(object)) is object
assert object().__new__ is object.__new__

# ---
# case: object allocation validates safety and excess arguments
class Plain:
    pass
try:
    object.__new__(Plain, 1)
    assert False
except TypeError as error:
    assert str(error) == 'Plain() takes no arguments'
class Custom:
    def __new__(cls, value):
        return object.__new__(cls)
try:
    object.__new__(Custom, 1)
    assert False
except TypeError as error:
    assert str(error) == 'object.__new__() takes exactly one argument (the type to instantiate)'
try:
    object.__new__(dict)
    assert False
except TypeError as error:
    assert str(error) == 'object.__new__(dict) is not safe, use dict.__new__()'
class Abstract:
    pass
Abstract.__abstractmethods__ = frozenset({'missing'})
try:
    object.__new__(Abstract)
    assert False
except TypeError:
    pass
