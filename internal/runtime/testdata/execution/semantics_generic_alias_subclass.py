# case: generic alias subclasses construct native state through super new
Alias = type(list[int])
events = []
class Subclass(Alias):
    def __new__(cls, origin, arguments):
        events.append(('new', cls))
        return super().__new__(cls, origin, arguments)
    def __init__(self, origin, arguments):
        events.append(('init', self.__origin__))
        self.tag = 'ready'
    def __repr__(self):
        return f'wrapped {super().__repr__()}'
value = Subclass(list, int)
assert type(value) is Subclass
assert isinstance(value, Alias)
assert value.__origin__ is list
assert value.__args__ == (int,)
assert value.tag == 'ready'
assert repr(value) == 'wrapped list[int]'
assert events == [('new', Subclass), ('init', list)]
class Inherited(Alias):
    pass
assert Inherited(dict, (str, int)).__args__ == (str, int)

# ---
# case: generic alias construction follows new return and init contracts
Alias = type(list[int])
class Different(Alias):
    def __new__(cls, *args):
        return 42
    def __init__(self, *args):
        assert False
assert Different(list, int) == 42
class InvalidInit(Alias):
    def __init__(self, *args):
        return 1
try:
    InvalidInit(list, int)
    assert False
except TypeError:
    pass
try:
    Alias.__new__(int, list, int)
    assert False
except TypeError:
    pass
assert Ellipsis is ...
