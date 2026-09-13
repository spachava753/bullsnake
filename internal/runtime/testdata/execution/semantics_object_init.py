# case: object initialization exposes executable wrapper descriptors
init = object.__init__
assert type(init).__name__ == 'wrapper_descriptor'
assert init is object.__dict__['__init__']
assert init.__name__ == '__init__'
assert init.__qualname__ == 'object.__init__'
assert init.__objclass__ is object
value = object()
bound = value.__init__
assert type(bound).__name__ == 'method-wrapper'
assert bound.__self__ is value
assert bound.__objclass__ is object
assert bound.__name__ == '__init__'
assert callable(init) and callable(bound)
assert init(value) is None
assert bound() is None
assert init.__get__(None, object) is init
assert init.__get__(value, object).__self__ is value
assert init.__get__(value)() is None

# ---
# case: object initializer works through inherited lookup and super
calls = []
class Base:
    pass
class Child(Base):
    def __init__(self, value):
        calls.append(value)
        super().__init__()
assert Base.__init__ is object.__init__
assert Base().__init__() is None
assert type(Child(12)) is Child
assert calls == [12]
class Explicit:
    __init__ = object.__init__
assert type(Explicit()) is Explicit
try:
    Explicit(1)
    assert False
except TypeError:
    pass

# ---
# case: object initialization preserves excess argument and receiver errors
value = object()
try:
    object.__init__()
    assert False
except TypeError:
    pass
for args, kw in [((1,), {}), ((), {'bad': 1})]:
    try:
        object.__init__(value, *args, **kw)
        assert False
    except TypeError as error:
        assert str(error) == 'object.__init__() takes exactly one argument (the instance to initialize)'
class Plain:
    pass
try:
    object.__init__(Plain(), 1)
    assert False
except TypeError as error:
    assert str(error) == 'Plain.__init__() takes exactly one argument (the instance to initialize)'
class Custom:
    def __init__(self):
        pass
try:
    object.__init__(Custom(), 1)
    assert False
except TypeError as error:
    assert str(error) == 'object.__init__() takes exactly one argument (the instance to initialize)'
# Immutable constructors consume arguments in __new__, not object.__init__.
assert object.__init__(1, 2, ignored=True) is None
assert object.__init__('one', 'two') is None
try:
    object.__init__([], 1)
    assert False
except TypeError:
    pass
