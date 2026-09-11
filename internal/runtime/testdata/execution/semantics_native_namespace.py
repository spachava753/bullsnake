# case: native namespace proxies expose callable implemented descriptors
proxy = type.__dict__
assert type(proxy).__name__ == 'mappingproxy'
assert proxy['__new__'] is type.__new__
C = proxy['__new__'](type, 'C', (), {'value': 7})
assert C.value == 7
assert proxy['__instancecheck__'](C, C())
assert proxy['__subclasscheck__'](C, C)
for cls, instance in [(set, {1}), (frozenset, frozenset({1}))]:
    descriptor = cls.__dict__['__contains__']
    assert descriptor is cls.__contains__
    assert descriptor(instance, 1)
    assert not descriptor(instance, 2)
    snapshot = cls.__dict__.copy()
    snapshot['__contains__'] = None
    assert cls.__dict__['__contains__'] is descriptor
assert '__contains__' not in object.__dict__
assert object.__dict__['__subclasshook__'](object, C) is NotImplemented

# ---
# case: raw native class subscription descriptors require an explicit receiver
for cls in [list, tuple, dict, set, frozenset]:
    raw = cls.__dict__['__class_getitem__']
    alias = raw(cls, int)
    assert alias.__origin__ is cls
    assert alias.__args__ == (int,)
    assert cls.__class_getitem__(str).__origin__ is cls
    try:
        raw(1, int)
        assert False
    except TypeError:
        pass
