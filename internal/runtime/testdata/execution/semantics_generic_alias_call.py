# case: generic aliases call their origin and retain orig class when writable
Alias = type(list[int])
assert list[int]((1, 2)) == [1, 2]
assert dict[str, int](a=1)['a'] == 1
class Origin:
    def __init__(self, value=0):
        self.value = value
alias = Alias(Origin, int)
value = alias(value=7)
assert type(value) is Origin
assert value.value == 7
assert value.__orig_class__ is alias
assert callable(alias)

# ---
# case: orig class assignment suppresses attribute and type errors only
Alias = type(list[int])
class Reject:
    @property
    def __orig_class__(self):
        return None
    @__orig_class__.setter
    def __orig_class__(self, value):
        raise TypeError('read only')
assert type(Alias(Reject, int)()) is Reject
class Broken:
    @property
    def __orig_class__(self):
        return None
    @__orig_class__.setter
    def __orig_class__(self, value):
        raise ValueError('metadata failed')
try:
    Alias(Broken, int)()
    assert False
except ValueError as error:
    assert str(error) == 'metadata failed'
class Failed:
    def __init__(self):
        raise RuntimeError('origin failed')
try:
    Alias(Failed, int)()
    assert False
except RuntimeError as error:
    assert str(error) == 'origin failed'

# ---
# case: alias subclass initialization ignores an instance replacement
Alias = type(list[int])
class Subclass(Alias):
    def __new__(cls, origin, args):
        result = super().__new__(cls, origin, args)
        result.__init__ = lambda *args: 42
        return result
    def __init__(self, origin, args):
        self.initialized = True
assert Subclass(list, int).initialized
