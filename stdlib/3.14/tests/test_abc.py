# Project-owned regression tests against unchanged CPython 3.14.7 abc.py,
# commit 823f0323ee6ec1402088b73bce1a38473cac36dc. This is not upstream test_abc.py.
from abc import ABC, ABCMeta, abstractmethod, abstractclassmethod, abstractstaticmethod, abstractproperty, get_cache_token, update_abstractmethods
import abc
import abc as repeated
assert abc is repeated

class Abstract(ABC):
    @abstractmethod
    def run(self):
        return 'base'

try:
    Abstract()
    assert False
except TypeError:
    pass

class Concrete(Abstract):
    def run(self):
        return (super().run(), 'concrete')

assert Concrete().run() == ('base', 'concrete')
assert isinstance(Concrete(), Abstract)
assert issubclass(Concrete, Abstract)
assert Abstract.__abstractmethods__ == frozenset({'run'})

class Virtual:
    pass

class Descendant(Virtual):
    pass

before = get_cache_token()
assert not issubclass(Virtual, Abstract)
assert not isinstance(Descendant(), Abstract)
assert Abstract.register(Virtual) is Virtual
assert get_cache_token() != before
assert issubclass(Descendant, Abstract)
assert isinstance(Descendant(), Abstract)
assert not hasattr(Virtual(), 'run')

class Root(ABC):
    pass
class Middle(Root):
    pass
assert not issubclass(Virtual, Root)
Middle.register(Virtual)
assert issubclass(Virtual, Root)
assert isinstance(Virtual(), Root)
try:
    Middle.register(Root)
    assert False
except RuntimeError:
    pass

class Descriptors(ABC):
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
assert Descriptors.__abstractmethods__ == frozenset({'class_method', 'static_method', 'value'})

class OldDescriptors(ABC):
    @abstractclassmethod
    def class_method(cls):
        pass
    @abstractstaticmethod
    def static_method():
        pass
    @abstractproperty
    def value(self):
        pass
assert OldDescriptors.__abstractmethods__ == Descriptors.__abstractmethods__

class Structural(ABC):
    @classmethod
    def __subclasshook__(cls, candidate):
        if hasattr(candidate, 'run'):
            return True
        return NotImplemented
assert issubclass(Concrete, Structural)
assert not issubclass(Virtual, Structural)
assert isinstance(Concrete(), Structural)

class Numbers(ABC):
    pass
Numbers.register(int)
assert isinstance(1, Numbers)
assert isinstance(True, Numbers)
assert not isinstance('text', Numbers)
assert issubclass(bool, Numbers)
Numbers._abc_registry_clear()
Numbers._abc_caches_clear()
assert not issubclass(int, Numbers)

# Abstract allocation never requires registry membership.
assert not isinstance(Virtual(), Concrete)
assert ABCMeta.__module__ == 'abc'

# Recompute abstract state after live class mutation without changing children.
class Updated(Abstract):
    pass
class UpdatedChild(Updated):
    pass
namespace = Updated.__dict__
Updated.run = lambda self: 'updated'
assert update_abstractmethods(Updated) is Updated
assert not Updated.__abstractmethods__
assert namespace['__abstractmethods__'] is Updated.__abstractmethods__
assert Updated().run() == 'updated'
assert UpdatedChild.__abstractmethods__ == frozenset({'run'})
update_abstractmethods(UpdatedChild)
assert UpdatedChild().run() == 'updated'
del Updated.run
update_abstractmethods(Updated)
assert Updated.__abstractmethods__ == frozenset({'run'})
try:
    Updated()
    assert False
except TypeError:
    pass
assert update_abstractmethods(Virtual) is Virtual
