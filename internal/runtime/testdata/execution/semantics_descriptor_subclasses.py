# ABC helper patterns adapted from CPython Lib/abc.py at
# 823f0323ee6ec1402088b73bce1a38473cac36dc; upstream source/license is in stdlib/3.14.
# case: ABC-style descriptor subclasses construct and bind
class abstractclassmethod(classmethod):
    __isabstractmethod__ = True
    def __init__(self, callable):
        callable.__isabstractmethod__ = True
        super().__init__(callable)
class abstractstaticmethod(staticmethod):
    __isabstractmethod__ = True
    def __init__(self, callable):
        callable.__isabstractmethod__ = True
        super().__init__(callable)
class abstractproperty(property):
    __isabstractmethod__ = True
class C:
    @abstractclassmethod
    def owner(cls):
        return cls
    @abstractstaticmethod
    def static():
        return 7
    @abstractproperty
    def value(self):
        return 8
assert C.owner() is C
assert C().owner() is C
assert C.static() == 7
assert C().static() == 7
assert C().value == 8
assert isinstance(C.value, abstractproperty)
assert isinstance(C.value, property)
assert type(C.value) is abstractproperty
assert C.value.__isabstractmethod__ is True
# ---
# case: descriptor subclass inherited initialization and attributes
class Static(staticmethod):
    marker = 3
class Child(Static):
    pass
def function():
    return 9
wrapper = Child(function)
assert type(wrapper) is Child
assert isinstance(wrapper, Static)
assert isinstance(wrapper, staticmethod)
assert wrapper.__func__ is function
assert wrapper.marker == 3
wrapper.extra = 7
assert wrapper.extra == 7
class C:
    method = wrapper
assert C().method() == 9
# ---
# case: descriptor initializer errors propagate
class Broken(classmethod):
    def __init__(self, callable):
        raise ValueError('descriptor failed')
try:
    Broken(None)
except ValueError as error:
    assert str(error) == 'descriptor failed'
else:
    assert False
class Bad(property):
    def __init__(self):
        return 3
try:
    Bad()
except TypeError as error:
    assert str(error) == "__init__() should return None, not 'int'"
else:
    assert False
# ---
# case: inherited abstractproperty participates in class computation
from _abc import _abc_init
class abstractproperty(property):
    __isabstractmethod__ = True
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        cls = type.__new__(mcls, name, bases, namespace)
        _abc_init(cls)
        return cls
class Base(metaclass=Meta):
    @abstractproperty
    def value(self):
        pass
assert Base.__abstractmethods__ == frozenset(('value',))
class Concrete(Base):
    @property
    def value(self):
        return 42
assert Concrete().value == 42
# ---
# case: property accessor copies reconstruct subclass state
calls = []
class Property(property):
    def __init__(self, fget=None, fset=None, fdel=None, doc=None):
        calls.append('init')
        super().__init__(fget, fset, fdel, doc)
def get(self):
    return self.stored
def put(self, value):
    self.stored = value
original = Property(get)
original.extra = 7
copy = original.setter(put)
assert type(copy) is Property
assert copy is not original
assert not hasattr(copy, 'extra')
assert calls == ['init', 'init']
class C:
    value = copy
c = C()
c.value = 42
assert c.value == 42
# ---
# case: wrapper subclasses reject writes to inherited readonly fields
class Static(staticmethod):
    pass
def function():
    pass
wrapper = Static(function)
try:
    wrapper.__func__ = None
except AttributeError:
    pass
else:
    assert False
assert wrapper.__func__ is function
