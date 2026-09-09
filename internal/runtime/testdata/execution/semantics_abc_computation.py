# Class setup follows the pinned abc.ABCMeta.__new__ path; registry helpers and
# the full abc import remain outside this fixture while weak references are deferred.
# case: automatic abstract method computation through a metaclass
from _abc import _abc_init
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        cls = super().__new__(mcls, name, bases, namespace)
        _abc_init(cls)
        return cls
def abstractmethod(func):
    func.__isabstractmethod__ = True
    return func
class Abstract(metaclass=Meta):
    @abstractmethod
    def run(self):
        return 7
assert Abstract.__abstractmethods__ == frozenset(('run',))
try:
    Abstract()
except TypeError as error:
    assert str(error) == "Can't instantiate abstract class Abstract without an implementation for abstract method 'run'"
else:
    assert False
class Inherited(Abstract):
    pass
assert Inherited.__abstractmethods__ == frozenset(('run',))
class Concrete(Abstract):
    def run(self):
        return super().run() + 1
assert Concrete.__abstractmethods__ == frozenset()
assert Concrete().run() == 8
# ---
# case: class static and property abstract markers are live
from _abc import _abc_init
def abstractmethod(func):
    func.__isabstractmethod__ = True
    return func
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        cls = super().__new__(mcls, name, bases, namespace)
        _abc_init(cls)
        return cls
class Abstract(metaclass=Meta):
    @classmethod
    @abstractmethod
    def class_method(cls):
        pass
    @staticmethod
    @abstractmethod
    def static_method():
        pass
    @property
    @abstractmethod
    def value(self):
        pass
assert Abstract.__abstractmethods__ == frozenset(('class_method', 'static_method', 'value'))
class Inherited(Abstract):
    pass
assert Inherited.__abstractmethods__ == Abstract.__abstractmethods__
class Concrete(Abstract):
    @classmethod
    def class_method(cls):
        return cls
    @staticmethod
    def static_method():
        return 10
    @property
    def value(self):
        return 20
assert Concrete.__abstractmethods__ == frozenset()
assert Concrete.class_method() is Concrete
assert Concrete.static_method() == 10
assert Concrete().value == 20
# ---
# case: accessor order short circuit and marker exceptions
seen = []
class Marker:
    def __bool__(self):
        seen.append('bool')
        return True
class Getter:
    @property
    def __isabstractmethod__(self):
        seen.append('get')
        return Marker()
class Broken:
    @property
    def __isabstractmethod__(self):
        raise ValueError('marker failed')
p = property(Getter(), Broken())
assert p.__isabstractmethod__ is True
assert seen == ['get', 'bool']
for wrapper in (staticmethod, classmethod, property):
    try:
        wrapper(Broken()).__isabstractmethod__
    except ValueError as error:
        assert str(error) == 'marker failed'
    else:
        assert False
class Missing:
    @property
    def __isabstractmethod__(self):
        raise AttributeError('absent')
assert property(Missing()).__isabstractmethod__ is False
def function():
    pass
wrapper = classmethod(function)
assert wrapper.__isabstractmethod__ is False
function.__isabstractmethod__ = True
assert wrapper.__isabstractmethod__ is True
function.__isabstractmethod__ = False
assert wrapper.__isabstractmethod__ is False
# ---
# case: computation failure preserves previous abstract methods
from _abc import _abc_init
class C:
    pass
C.__abstractmethods__ = frozenset(('previous',))
class Broken:
    @property
    def __isabstractmethod__(self):
        raise ValueError('marker failed')
C.value = Broken()
try:
    _abc_init(C)
except ValueError as error:
    assert str(error) == 'marker failed'
else:
    assert False
assert C.__abstractmethods__ == frozenset(('previous',))
del C.value
assert _abc_init(C) is None
assert C.__abstractmethods__ == frozenset()
assert isinstance(C(), C)
# ---
# case: inherited names interleave iteration and descriptor lookup
from _abc import _abc_init
events = []
class Marker:
    @property
    def __isabstractmethod__(self):
        events.append('marker')
        return True
class Descriptor:
    def __get__(self, instance, owner):
        events.append('lookup')
        return Marker()
class Names:
    def __iter__(self):
        events.append('first')
        yield 'method'
        events.append('second')
        yield 'absent'
class Base:
    method = Descriptor()
Base.__abstractmethods__ = Names()
class Child(Base):
    pass
_abc_init(Child)
assert events == ['first', 'lookup', 'marker', 'second']
assert Child.__abstractmethods__ == frozenset(('method',))
# ---
# case: direct namespace snapshot survives marker mutation
from _abc import _abc_init
events = []
class First:
    @property
    def __isabstractmethod__(self):
        events.append('first')
        del Target.second
        return True
class Second:
    @property
    def __isabstractmethod__(self):
        events.append('second')
        return True
class Target:
    first = First()
    second = Second()
_abc_init(Target)
assert events == ['first', 'second']
assert Target.__abstractmethods__ == frozenset(('first', 'second'))
# ---
# case: getattr and hasattr surround the complete descriptor computation
class FalseMarker:
    def __bool__(self):
        return False
class Function:
    @property
    def __isabstractmethod__(self):
        return FalseMarker()
wrapper = classmethod(Function())
assert getattr(wrapper, '__isabstractmethod__', 7) is False
assert hasattr(wrapper, '__isabstractmethod__') is True
class MissingTruth:
    def __bool__(self):
        raise AttributeError('truth missing')
class Function:
    @property
    def __isabstractmethod__(self):
        return MissingTruth()
wrapper = classmethod(Function())
assert getattr(wrapper, '__isabstractmethod__', 7) == 7
assert hasattr(wrapper, '__isabstractmethod__') is False
