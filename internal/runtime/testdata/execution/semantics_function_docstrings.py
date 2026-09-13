# case: function docstrings retain literal text without running the body
calls = []
def documented():
    ('first\n' r'raw\n')
    calls.append('called')
    return 3
assert calls == []
assert documented.__doc__ == 'first\nraw\\n'
assert documented.__doc__ is documented.__doc__
def empty():
    ''
def missing():
    pass
    'not docs'
def byte_string():
    b'not docs'
def formatted():
    f'not docs'
def template():
    t'not docs'
assert empty.__doc__ == ''
assert missing.__doc__ is None
assert byte_string.__doc__ is None
assert formatted.__doc__ is None
assert template.__doc__ is None
assert (lambda: 'not docs').__doc__ is None
assert documented() == 3
assert calls == ['called']

# ---
# case: docs are writable per function and deletion sets None
def factory():
    def function():
        'original'
    return function
first = factory()
second = factory()
assert first.__doc__ is second.__doc__
first.__doc__ = ['changed']
assert first.__doc__ == ['changed']
assert second.__doc__ == 'original'
del first.__doc__
assert first.__doc__ is None
del first.__doc__
assert first.__doc__ is None
slot = type(first).__doc__
assert type(slot).__name__ == 'member_descriptor'
assert slot.__get__(second) is second.__doc__
assert slot.__set__(first, 42) is None
assert first.__doc__ == 42
assert slot.__delete__(first) is None
assert first.__doc__ is None

# ---
# case: asynchronous generic and bound methods expose original docs
async def asynchronous():
    'async docs'
    return 1
def generic[T]():
    'generic docs'
    yield T
class Sample:
    def method(self):
        'method docs'
        return 1
assert asynchronous.__doc__ == 'async docs'
assert generic.__doc__ == 'generic docs'
assert Sample.method.__doc__ == 'method docs'
assert Sample().method.__doc__ == 'method docs'
