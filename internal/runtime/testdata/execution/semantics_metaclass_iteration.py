# case: classes iterate through metaclass slots across consumers
class Meta(type):
    def __iter__(cls):
        return iter((1, 2, 3))
class Values(metaclass=Meta):
    def __iter__(self):
        assert False
assert list(Values) == [1, 2, 3]
assert tuple(Values) == (1, 2, 3)
assert set(Values) == {1, 2, 3}
assert [value for value in Values] == [1, 2, 3]
assert list(enumerate(Values)) == [(0, 1), (1, 2), (2, 3)]
assert list(map(lambda value: value + 1, Values)) == [2, 3, 4]
assert list(filter(lambda value: value > 1, Values)) == [2, 3]
assert list(zip(Values, Values)) == [(1, 1), (2, 2), (3, 3)]
assert all(Values) is True and any(Values) is True
assert sum(Values) == 6

# ---
# case: a class can itself be an iterator through its metaclass
class Meta(type):
    def __iter__(cls):
        return cls
    def __next__(cls):
        if cls.position == 3:
            raise StopIteration('finished')
        result = cls.position
        cls.position += 1
        return result
class Cursor(metaclass=Meta):
    position = 0
    __next__ = None
assert iter(Cursor) is Cursor
assert next(Cursor) == 0
assert list(Cursor) == [1, 2]
assert next(Cursor, 'done') == 'done'
Cursor.position = 0
assert [value for value in Cursor] == [0, 1, 2]
Cursor.position = 0
assert list(map(lambda value: value, Cursor)) == [0, 1, 2]
Cursor.position = 0
assert list(filter(None, Cursor)) == [1, 2]
Cursor.position = 0
assert list(enumerate(Cursor)) == [(0, 0), (1, 1), (2, 2)]
Cursor.position = 0
assert list(zip(Cursor)) == [(0,), (1,), (2,)]
Cursor.position = 0
assert all(Cursor) is False
Cursor.position = 0
assert any(Cursor) is True

# ---
# case: metaclass iteration binds descriptors and propagates lookup failures
calls = []
class Iteration:
    def __get__(self, receiver, owner):
        calls.append((receiver, owner))
        return lambda: iter(('item',))
class Meta(type):
    __iter__ = Iteration()
class Values(metaclass=Meta):
    pass
assert list(Values) == ['item']
assert calls == [(Values, Meta)]
class BrokenMeta(type):
    def __iter__(cls):
        raise ValueError('iteration failed')
class Broken(metaclass=BrokenMeta):
    pass
try:
    list(Broken)
except ValueError as error:
    assert str(error) == 'iteration failed'
else:
    assert False
class InvalidMeta(type):
    def __iter__(cls):
        return []
class Invalid(metaclass=InvalidMeta):
    pass
try:
    iter(Invalid)
except TypeError as error:
    assert str(error) == "iter() returned non-iterator of type 'list'"
else:
    assert False
class Plain:
    def __iter__(self):
        return iter(())
try:
    iter(Plain)
except TypeError:
    pass
else:
    assert False
