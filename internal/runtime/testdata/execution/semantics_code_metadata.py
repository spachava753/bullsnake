# case: function and frame expose one stable immutable code identity
import sys
def outer(captured):
    def sample(a, /, b=1, *args, key=2, **kwargs):
        local = captured
        return sys._getframe(), local
    return sample
first, second = outer(10), outer(20)
assert first is not second
code = first.__code__
assert code is first.__code__
assert code is second.__code__
assert type(code).__name__ == 'code'
assert code.co_name == 'sample'
assert code.co_qualname == 'outer.<locals>.sample'
assert code.co_filename.endswith('semantics_code_metadata.py')
assert code.co_firstlineno == 4
assert code.co_argcount == 2
assert code.co_posonlyargcount == 1
assert code.co_kwonlyargcount == 1
assert code.co_freevars == ('captured',)
assert outer.__code__.co_cellvars == ('captured',)
assert code.co_varnames[:5] == ('a', 'b', 'key', 'args', 'kwargs'), code.co_varnames
assert 'local' in code.co_varnames
assert code.co_nlocals == len(code.co_varnames)
assert code.co_flags & 31 == 31
frame, result = first(3)
assert result == 10
assert frame.f_code is code
assert getattr(frame, 'f_code') is code
assert getattr(first, '__code__') is code
assert getattr(code, 'co_name') == 'sample'
try:
    code.co_name = 'changed'
    assert False
except AttributeError:
    pass
try:
    del code.co_name
    assert False
except AttributeError:
    pass
assert first(4)[1] == 10

# ---
# case: Python code flags distinguish suspended function kinds
def generator():
    yield 1
async def coroutine():
    return 1
async def async_generator():
    yield 1
assert generator.__code__.co_flags & 32
assert coroutine.__code__.co_flags & 128
assert async_generator.__code__.co_flags & 512
assert not generator.__code__.co_flags & (128 | 512)
assert not coroutine.__code__.co_flags & (32 | 512)
assert not async_generator.__code__.co_flags & (32 | 128)

# ---
# case: code replacement remains explicitly unsupported
# error: AttributeError
# message: "readonly attribute"
def f():
    pass
f.__code__ = f.__code__

# ---
# case: code local names distinguish captured locals from parameter cells
def outer(a, *, key):
    captured = 12
    def inner():
        return a, key, captured
    ordinary = 13
    return inner, ordinary
code = outer.__code__
assert code.co_varnames == ('a', 'key', 'inner', 'ordinary')
assert code.co_nlocals == 4
assert set(code.co_cellvars) == {'a', 'key', 'captured'}
inner, ordinary = outer(10, key=11)
assert inner() == (10, 11, 12)
assert ordinary == 13
