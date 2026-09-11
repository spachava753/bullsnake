# case: class statements rewrite aliases and retain original bases
Alias = type(list[int])
class Base:
    def value(self):
        return 12
alias = Alias(Base, int)
class Child(alias):
    pass
assert Child.__bases__ == (Base,)
assert Child.__orig_bases__ == (alias,)
assert Child().value() == 12
assert isinstance(Child(), Base)
try:
    type('Dynamic', (alias,), {})
    assert False
except TypeError:
    pass

# ---
# case: base rewriting precedes prepare and publishes originals after body
calls = []
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        calls.append(('prepare', bases))
        return {}
    def __new__(meta, name, bases, namespace):
        calls.append(('new', bases, namespace['__orig_bases__']))
        return super().__new__(meta, name, bases, namespace)
class Left:
    pass
class Right:
    pass
class Entry:
    def __mro_entries__(self, bases):
        calls.append(('entries', bases))
        return (Left, Right)
entry = Entry()
class Result(entry, metaclass=Meta):
    calls.append('body')
    __orig_bases__ = 'overwritten'
assert Result.__bases__ == (Left, Right)
assert calls == [('entries', (entry,)), ('prepare', (Left, Right)), 'body', ('new', (Left, Right), (entry,))]
assert calls[0][1] is Result.__orig_bases__

# ---
# case: base rewriting removes bases skips classes and propagates hook errors
class Remove:
    def __mro_entries__(self, bases):
        return ()
class Empty(Remove()):
    pass
assert Empty.__bases__ == (object,)
class Base:
    @classmethod
    def __mro_entries__(cls, bases):
        raise AssertionError('class hooks must not run')
class Normal(Base):
    pass
assert '__orig_bases__' not in Normal.__dict__
class Bad:
    def __mro_entries__(self, bases):
        return []
try:
    class Rejected(Bad()):
        assert False
except TypeError as error:
    assert str(error) == '__mro_entries__ must return a tuple'
class Broken:
    def __mro_entries__(self, bases):
        raise ValueError('base failed')
try:
    class Failed(Broken()):
        assert False
except ValueError as error:
    assert str(error) == 'base failed'
