# Runtime prerequisites for ABCMeta; these cases do not import abc.
# case: abstract allocation and metadata isolation
calls = []
class Base:
    def __init__(self):
        calls.append('init')
Base.__abstractmethods__ = frozenset(('zeta', 'alpha'))
try:
    Base()
except TypeError as error:
    assert str(error) == "Can't instantiate abstract class Base without an implementation for abstract methods 'alpha', 'zeta'"
else:
    assert False
assert calls == []
class Child(Base):
    pass
assert not hasattr(Child, '__abstractmethods__')
assert isinstance(Child(), Child)
assert calls == ['init']
Base.__abstractmethods__ = frozenset()
assert isinstance(Base(), Base)
Base.__abstractmethods__ = ('only',)
del Base.__abstractmethods__
assert not hasattr(Base, '__abstractmethods__')
assert isinstance(Base(), Base)
# ---
# case: class body metadata does not activate abstract allocation
class Body:
    __abstractmethods__ = frozenset(('method',))
assert isinstance(Body(), Body)
Dynamic = type('Dynamic', (), {'__abstractmethods__': frozenset(('method',))})
assert isinstance(Dynamic(), Dynamic)
# ---
# case: abstract flag snapshots truth but retains metadata
class Mutable:
    pass
names = ['method']
Mutable.__abstractmethods__ = names
assert Mutable.__abstractmethods__ is names
names.pop()
try:
    Mutable()
except TypeError as error:
    assert str(error) == "Can't instantiate abstract class Mutable without an implementation for abstract method ''"
else:
    assert False
Mutable.__abstractmethods__ = names
names.append('method')
assert isinstance(Mutable(), Mutable)
# ---
# case: abstract metadata truth callbacks and atomic failure
class Target:
    pass
events = []
class Names:
    def __bool__(self):
        events.append('truth')
        return True
    def __iter__(self):
        events.append('iterate')
        yield 'z'
        yield 'a'
names = Names()
assert setattr(Target, '__abstractmethods__', names) is None
assert events == ['truth']
assert Target.__abstractmethods__ is names
try:
    Target()
except TypeError as error:
    assert str(error) == "Can't instantiate abstract class Target without an implementation for abstract methods 'a', 'z'"
else:
    assert False
assert events == ['truth', 'iterate']
class Broken:
    def __bool__(self):
        raise ValueError('truth failed')
try:
    Target.__abstractmethods__ = Broken()
except ValueError as error:
    assert str(error) == 'truth failed'
else:
    assert False
assert Target.__abstractmethods__ is names
class Concrete:
    def __len__(self):
        return 0
Target.__abstractmethods__ = Concrete()
assert isinstance(Target(), Target)
assert delattr(Target, '__abstractmethods__') is None
try:
    del Target.__abstractmethods__
except AttributeError as error:
    assert str(error) == '__abstractmethods__'
else:
    assert False
# ---
# case: abstract metadata iterator failure propagates
class Target:
    pass
class Names:
    def __iter__(self):
        yield 'first'
        raise LookupError('names failed')
Target.__abstractmethods__ = Names()
try:
    Target()
except LookupError as error:
    assert str(error) == 'names failed'
else:
    assert False
# ---
# case: abstract metadata rejects non-string names after sorting
class Target:
    pass
Target.__abstractmethods__ = (1,)
try:
    Target()
except TypeError as error:
    assert str(error) == 'sequence item 0: expected str instance, int found'
else:
    assert False
# ---
# case: ordinary constructor arguments checked before abstract methods
class Target:
    pass
Target.__abstractmethods__ = ('method',)
try:
    Target(1)
except TypeError as error:
    assert str(error) == 'Target() takes no arguments'
else:
    assert False
# ---
# case: abstract names use Python comparisons before string validation
class Target:
    pass
comparisons = []
class Name:
    def __lt__(self, other):
        comparisons.append('compare')
        return False
Target.__abstractmethods__ = [Name(), Name()]
try:
    Target()
except TypeError as error:
    assert str(error) == 'sequence item 0: expected str instance, Name found'
else:
    assert False
assert comparisons == ['compare']
# ---
# case: invalid truth result preserves concrete state
class Target:
    pass
class Broken:
    def __bool__(self):
        return 1
try:
    setattr(Target, '__abstractmethods__', Broken())
except TypeError as error:
    assert str(error) == '__bool__ should return bool, returned int'
else:
    assert False
assert not hasattr(Target, '__abstractmethods__')
assert isinstance(Target(), Target)
# ---
# case: builtin exception allocation does not use object abstract restriction
class Error(Exception):
    pass
Error.__abstractmethods__ = ('method',)
assert isinstance(Error('message'), Error)
