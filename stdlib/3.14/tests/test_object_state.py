"""Exercise native object state through unchanged copyreg slot discovery."""
# Do not import copyreg first: object state must load it through the normal importer.
class Sample:
    pass
value = Sample()
assert value.__getstate__() is None
assert Sample.__slotnames__ == []
value.number = 42
state = value.__getstate__()
assert state == {'number': 42}
assert state is value.__getstate__()
state['number'] = 99
assert value.number == 99
value.extra = 'value'
assert state['extra'] == 'value'
assert object().__getstate__() is None

class Custom:
    def __getstate__(self):
        return 'custom'
custom = Custom()
custom.field = 1
assert custom.__getstate__() == 'custom'
assert object.__getstate__(custom) == {'field': 1}

class Mapping(dict):
    pass
mapping = Mapping({'entry': 1})
mapping.label = 'state'
assert mapping.__getstate__() == {'label': 'state'}
assert 'entry' not in mapping.__getstate__()

import copyreg
assert copyreg._slotnames(Sample) is Sample.__slotnames__
assert '__getstate__' not in dict.__dict__
assert Mapping.__getstate__ is object.__getstate__
assert object.__getstate__.__objclass__ is object

# Slot-cache lookup is local to the class, not inherited from a parent.
class Child(Sample):
    pass
assert Child().__getstate__() is None
assert Child.__slotnames__ is not Sample.__slotnames__

original = copyreg._slotnames
calls = []
class DuringDiscovery:
    pass
empty = DuringDiscovery()
def discover(cls):
    calls.append(cls)
    empty.added = 1
    return []
copyreg._slotnames = discover
try:
    # Empty/nonempty state is captured before the helper runs.
    assert empty.__getstate__() is None
    state = empty.__getstate__()
    assert state == {'added': 1}
    assert calls == [DuringDiscovery, DuringDiscovery]
    def fail(cls):
        raise ValueError('slot lookup failed')
    copyreg._slotnames = fail
    try:
        empty.__getstate__()
    except ValueError as error:
        assert str(error) == 'slot lookup failed'
    else:
        assert False
finally:
    copyreg._slotnames = original
assert empty.__getstate__() is state

DuringDiscovery.__slotnames__ = 1
try:
    empty.__getstate__()
except TypeError as error:
    assert str(error) == 'DuringDiscovery.__slotnames__ should be a list or None, not int'
else:
    assert False
DuringDiscovery.__slotnames__ = ['field']
try:
    empty.__getstate__()
except TypeError as error:
    assert str(error) == 'object state for nonempty slots is not implemented'
else:
    assert False
DuringDiscovery.__slotnames__ = None
assert empty.__getstate__() is state

# Native classes also run the real helper when no slot cache exists.
calls = []
def native_discovery(cls):
    calls.append(cls)
    return None
copyreg._slotnames = native_discovery
try:
    assert object().__getstate__() is None
    assert object().__getstate__() is None
    assert calls == [object, object]
    copyreg._slotnames = lambda cls: 1
    try:
        object().__getstate__()
    except TypeError as error:
        assert str(error) == "copyreg._slotnames didn't return a list or None"
    else:
        assert False
finally:
    copyreg._slotnames = original

for call in (lambda: value.__getstate__(1), lambda: value.__getstate__(x=1)):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
