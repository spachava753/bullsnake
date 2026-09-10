# case: class namespace proxies retain live ordered values and raw descriptors
class Descriptor:
    def __get__(self, instance, owner):
        raise AssertionError('must not bind')
raw = Descriptor()
class Base:
    inherited = 1
class C(Base):
    first = 1
    second = 2
    descriptor = raw
    @classmethod
    def method(cls):
        return cls
proxy = C.__dict__
assert type(proxy).__name__ == 'mappingproxy'
assert 'inherited' not in proxy
assert proxy['descriptor'] is raw
assert proxy['method'].__func__ is C.method.__func__
assert proxy.get('missing') is None
assert proxy.get('missing', raw) is raw
keys = proxy.keys()
values = proxy.values()
items = proxy.items()
snapshot = proxy.copy()
size = len(proxy)
C.first = 10
assert proxy['first'] == 10
assert snapshot['first'] == 1
assert ('first', 10) in list(items)
assert 10 in list(values)
assert len(proxy) == size
C.third = 3
assert len(proxy) == size + 1
assert list(keys)[-1] == 'third'
del C.first
C.first = 20
assert list(keys)[-1] == 'first'
assert len([key for key in keys if key == 'first']) == 1
assert C.__dict__['first'] == 20
snapshot['first'] = 99
assert C.first == 20
assert bool(proxy)

# ---
# case: proxies reject mutations and report missing keys
class C:
    item = 1
proxy = C.__dict__
for operation in [lambda: setattr(C, '__dict__', {}), lambda: delattr(C, '__dict__')]:
    try:
        operation()
        assert False
    except AttributeError:
        pass
try:
    proxy['item'] = 2
    assert False
except TypeError:
    pass
try:
    del proxy['item']
    assert False
except TypeError:
    pass
for name in ['clear', 'pop', 'update']:
    assert not hasattr(proxy, name)
try:
    proxy['missing']
    assert False
except KeyError:
    pass
assert C.item == 1

# ---
# case: proxy iterators observe replacement and reject key changes
class C:
    item = 1
proxy = C.__dict__
iterator = iter(proxy.items())
C.item = 2
assert ('item', 2) in list(iterator)
iterator = iter(proxy)
C.extra = 3
try:
    next(iterator)
    assert False
except RuntimeError:
    pass

# ---
# case: class body deletion and reinsertion occur only once in namespace order
class C:
    first = 1
    second = 2
    del first
    first = 3
names = [name for name in C.__dict__ if name in ['first', 'second']]
assert names == ['second', 'first']

# ---
# case: evaluating class annotations publishes the cached dictionary in live views
class C:
    value: int
proxy = C.__dict__
assert '__annotations__' not in proxy
annotations = C.__annotations__
assert proxy['__annotations__'] is annotations
assert annotations['value'] is int
assert '__annotations_cache__' not in proxy
