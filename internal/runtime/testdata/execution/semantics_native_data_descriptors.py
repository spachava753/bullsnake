# case: native function attributes expose real getset and member descriptors
def sample():
    return 1
Function = type(sample)
code = Function.__code__
globals_descriptor = Function.__globals__
assert type(code).__name__ == 'getset_descriptor'
assert type(globals_descriptor).__name__ == 'member_descriptor'
assert code is Function.__dict__['__code__']
assert code.__name__ == '__code__'
assert code.__objclass__ is Function
assert code.__get__(None, Function) is code
assert code.__get__(sample) is sample.__code__
assert globals_descriptor.__get__(sample) is sample.__globals__
assert not callable(code)
for descriptor in [code, globals_descriptor]:
    try:
        descriptor.__get__(1)
        assert False
    except TypeError:
        pass
    try:
        descriptor.__set__(sample, None)
        assert False
    except AttributeError:
        pass

# ---
# case: cell data descriptors read write and delete real closure bindings
def factory():
    value = 1
    def get():
        return value
    return get
get = factory()
cell = get.__closure__[0]
descriptor = type(cell).cell_contents
assert type(descriptor).__name__ == 'getset_descriptor'
assert descriptor.__get__(cell) == 1
descriptor.__set__(cell, 2)
assert get() == 2
descriptor.__delete__(cell)
try:
    get()
    assert False
except NameError:
    pass
try:
    descriptor.__get__(cell)
    assert False
except ValueError:
    pass

# ---
# case: type annotation descriptor executes the real lazy annotations
calls = []
def annotation():
    calls.append(1)
    return int
class Annotated:
    value: annotation()
descriptor = type.__dict__['__annotations__']
assert calls == []
assert descriptor.__get__(Annotated) == {'value': int}
assert descriptor.__get__(Annotated) is Annotated.__annotations__
assert calls == [1]
