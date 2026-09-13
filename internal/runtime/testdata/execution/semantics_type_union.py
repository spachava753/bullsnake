# case: class unions normalize flatten and preserve first occurrence order
union = int | str
assert type(union).__module__ == 'typing'
assert type(union).__name__ == 'Union'
assert union.__args__ == (int, str)
assert union.__args__ is union.__args__
assert union.__parameters__ == ()
assert union.__parameters__ is union.__parameters__
assert int | int is int
assert (union | bytes | int).__args__ == (int, str, bytes)
assert (None | int).__args__ == (type(None), int)
assert (int | None).__args__ == (int, type(None))
assert repr(int | str | None) == 'int | str | None'
assert int | str == str | int
assert int | str != int | bytes
assert hash(int | str) == hash(str | int)
assert type.__or__(int, str) == union
assert type.__or__(int, 1) is NotImplemented
try:
    union.__args__ = ()
    assert False
except AttributeError:
    pass

# ---
# case: union class checks short circuit through native and Python classes
class Base:
    pass
class Child(Base):
    pass
assert isinstance(1, int | str)
assert isinstance(None, int | None)
assert not isinstance([], int | str)
assert isinstance(Child(), str | Base)
assert issubclass(Child, str | Base)
assert not issubclass(list, int | str)
class Meta(type):
    def __instancecheck__(cls, obj):
        return obj == 42
class Custom(metaclass=Meta):
    pass
assert isinstance(42, str | Custom)
assert not isinstance(0, str | Custom)

# ---
# case: union operators respect metaclass and reflected Python slots
class Meta(type):
    def __or__(cls, other):
        return ('left', cls, other)
    def __ror__(cls, other):
        return ('right', other, cls)
class Custom(metaclass=Meta):
    pass
assert Custom | int == ('left', Custom, int)
assert int | Custom == ('right', int, Custom)
class Declining(type):
    def __or__(cls, other):
        return NotImplemented
class Plain(metaclass=Declining):
    pass
assert (Plain | int).__args__ == (Plain, int)
class Right:
    def __ror__(self, left):
        return 'reflected'
assert int | Right() == 'reflected'

# ---
# case: reflected metaclass priority requires an override
class LeftMeta(type):
    def __or__(cls, other):
        return 'left'
    def __ror__(cls, other):
        return 'inherited reflected'
class RightMeta(LeftMeta):
    pass
class Left(metaclass=LeftMeta):
    pass
class Right(metaclass=RightMeta):
    pass
assert Left | Right == 'left'
class OverrideMeta(LeftMeta):
    def __ror__(cls, other):
        return 'overridden reflected'
class Override(metaclass=OverrideMeta):
    pass
assert Left | Override == 'overridden reflected'

# ---
# case: union slots preserve argument order and reject invalid receivers
union = int | str
assert union.__origin__ is type(union)
assert union.__name__ == union.__qualname__ == 'Union'
assert union.__module__ == 'typing'
assert type.__ror__(int, str).__args__ == (str, int)
assert union.__or__(None).__args__ == (int, str, type(None))
assert union.__ror__(bytes).__args__ == (bytes, int, str)
assert union.__hash__() == hash(union)
assert getattr(union, '__args__') is union.__args__
assert (int | ValueError).__args__ == (int, ValueError)
assert isinstance(ValueError(), int | Exception)
try:
    type.__or__(1, str)
    assert False
except TypeError as error:
    assert str(error) == "descriptor '__or__' requires a 'type' object"
try:
    int | 1
    assert False
except TypeError as error:
    assert str(error) == "unsupported operand type(s) for |: 'type' and 'int'"
try:
    int | list[str]
    assert False
except TypeError as error:
    assert str(error) == "unsupported operand type(s) for |: 'type' and 'GenericAlias'"

# ---
# case: unsupported metaclass key callbacks are not silently ignored
class Meta(type):
    def __hash__(cls):
        return 1
class Custom(metaclass=Meta):
    pass
try:
    int | Custom
    assert False
except NotImplementedError as error:
    assert str(error) == 'unions with metaclass equality or hashing overrides are not supported'
