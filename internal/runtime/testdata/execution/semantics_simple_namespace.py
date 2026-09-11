# case: implementation namespace uses truthful runtime metadata and writable state
import sys
assert sys.implementation.name == 'bullsnake'
assert sys.implementation.cache_tag is None
Namespace = type(sys.implementation)
assert Namespace.__name__ == 'SimpleNamespace'
assert Namespace.__module__ == 'types'
namespace = Namespace({'first': 1}, second=2, first=3)
assert namespace.first == 3
assert namespace.second == 2
assert list(namespace.__dict__) == ['first', 'second']
assert namespace.__dict__ is namespace.__dict__
namespace.third = 4
assert namespace.__dict__['third'] == 4
namespace.__dict__['fourth'] = 5
assert namespace.fourth == 5
del namespace.first
assert 'first' not in namespace.__dict__
namespace.__dict__.pop('second')
assert not hasattr(namespace, 'second')
try:
    namespace.__dict__ = {}
    assert False
except AttributeError:
    pass
assert Namespace([(name, len(name)) for name in ['a', 'bb']]).__dict__ == {'a': 1, 'bb': 2}

# ---
# case: namespace initialization validates keys and retains state on reinitialization
import sys
Namespace = type(sys.implementation)
namespace = Namespace(first=1)
namespace.__init__({'second': 2}, third=3)
assert namespace.__dict__ == {'first': 1, 'second': 2, 'third': 3}
try:
    namespace.__init__({'valid': 4, 1: 5}, ignored=True)
    assert False
except TypeError as error:
    assert str(error) == 'keywords must be strings'
assert namespace.__dict__ == {'first': 1, 'second': 2, 'third': 3}
for args in [(1, 2), (1,)]:
    try:
        Namespace(*args)
        assert False
    except TypeError:
        pass

# ---
# case: namespace subclasses use native construction and initializer slots
import sys
Namespace = type(sys.implementation)
class Child(Namespace):
    def __init__(self, value):
        super().__init__(value=value)
child = Child(42)
assert type(child) is Child
assert child.value == 42
assert child.__dict__ == {'value': 42}
class Factory(Namespace):
    def __new__(cls):
        return 42
    def __init__(self):
        raise AssertionError('foreign result bypasses initialization')
assert Factory() == 42
