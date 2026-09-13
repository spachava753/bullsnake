# case: string join exposes a real method descriptor with native binding
join = str.join
assert type(join).__name__ == 'method_descriptor'
assert join is str.__dict__['join']
assert join.__objclass__ is str
assert join.__name__ == 'join'
assert join.__qualname__ == 'str.join'
assert join.__get__(None, str) is join
assert join('-', ['a', 'b']) == 'a-b'
separator = ':'
bound = join.__get__(separator, str)
assert type(bound) is type(len)
assert bound.__self__ is separator
assert bound.__name__ == 'join'
assert bound.__qualname__ == 'str.join'
assert bound(['a', 'b']) == 'a:b'
assert separator.join.__self__ is separator
assert not hasattr(bound, '__objclass__')
assert separator.join(['a', 'b']) == join(separator, ['a', 'b'])

# ---
# case: direct join descriptor resumes iterators and validates receiver and arity
calls = []
def strings():
    calls.append('start')
    yield 'x'
    calls.append('next')
    yield 'y'
assert str.join('|', strings()) == 'x|y'
assert calls == ['start', 'next']
for args in [(), (1, []), ('-',), ('-', [], [])]:
    try:
        str.join(*args)
        assert False
    except TypeError:
        pass
try:
    str.join.__get__(1, int)
    assert False
except TypeError:
    pass
try:
    str.join('-', ['a', 1])
    assert False
except TypeError:
    pass
try:
    str.join('-', iterable=['a'])
    assert False
except TypeError:
    pass
