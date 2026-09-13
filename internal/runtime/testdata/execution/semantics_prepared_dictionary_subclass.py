# case: prepared dict subclasses observe reads writes deletes and preserve identity
import sys
operations = []
class Namespace(dict):
    def __getitem__(self, key):
        operations.append(('get', key))
        if key == 'provided':
            return 42
        return super().__getitem__(key)
    def __setitem__(self, key, value):
        operations.append(('set', key))
        super().__setitem__(key, value)
        return 'ignored'
    def __delitem__(self, key):
        operations.append(('del', key))
        super().__delitem__(key)
        return 'ignored'
prepared = Namespace()
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return prepared
    def __new__(meta, name, bases, namespace):
        assert namespace is prepared
        return super().__new__(meta, name, bases, namespace)
class Sample(metaclass=Meta):
    retained_frame = sys._getframe()
    first = provided
    second = first
    del first
    def method(self):
        return __class__
assert Sample.second == 42
assert not hasattr(Sample, 'first')
assert Sample().method() is Sample
assert Sample.retained_frame.f_locals is prepared
assert ('get', 'provided') in operations
assert ('get', 'first') in operations
assert ('set', 'second') in operations
assert ('del', 'first') in operations
assert ('set', '__classcell__') in operations
assert Sample.__dict__ is not prepared
prepared['second'] = 99
assert Sample.second == 42

# ---
# case: namespace lookup falls back only on KeyError and assignment errors escape
class Namespace(dict):
    def __getitem__(self, key):
        if key == 'broken':
            raise ValueError('lookup failed')
        return super().__getitem__(key)
    def __setitem__(self, key, value):
        if key == 'blocked':
            raise RuntimeError('store failed')
        return super().__setitem__(key, value)
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return Namespace()
fallback = 7
class Good(metaclass=Meta):
    value = fallback
assert Good.value == 7
try:
    class Bad(metaclass=Meta):
        value = broken
    assert False
except ValueError as error:
    assert str(error) == 'lookup failed'
try:
    class Blocked(metaclass=Meta):
        blocked = 3
    assert False
except RuntimeError as error:
    assert str(error) == 'store failed'

# ---
# case: prepared namespace hooks participate before enclosing closure fallback
class Namespace(dict):
    def __getitem__(self, key):
        if key == 'injected':
            return 'namespace'
        return super().__getitem__(key)
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return Namespace()
def outer():
    injected = 'closure'
    other = 3
    class Sample(metaclass=Meta):
        first = injected
        second = other
    return Sample
result = outer()
assert result.first == 'namespace'
assert result.second == 3

# ---
# case: namespace deletion does not pre-read and translates missing keys
operations = []
class Namespace(dict):
    def __getitem__(self, key):
        if key == 'value':
            raise ValueError('must not read before deleting')
        return super().__getitem__(key)
    def __delitem__(self, key):
        operations.append(key)
        return super().__delitem__(key)
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return Namespace()
class Sample(metaclass=Meta):
    value = 1
    del value
assert operations == ['value']
try:
    class Missing(metaclass=Meta):
        del missing
    assert False
except NameError as error:
    assert str(error) == "name 'missing' is not defined"

# ---
# case: original bases publish through the prepared namespace callback
operations = []
class Namespace(dict):
    def __setitem__(self, key, value):
        operations.append(key)
        return super().__setitem__(key, value)
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return Namespace()
class Base:
    pass
class Alias:
    def __mro_entries__(self, bases):
        return (Base,)
alias = Alias()
class Child(alias, metaclass=Meta):
    pass
assert Child.__orig_bases__ == (alias,)
assert '__orig_bases__' in operations
