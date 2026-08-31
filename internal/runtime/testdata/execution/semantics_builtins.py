# Runtime behavior for pure builtins.
# case: lengths of built-in values
assert len('') == 0
assert len('a\u2603') == 2
assert len('\U0001f600') == 1
assert len('\ud800') == 1
assert len(b'\x00abc') == 4
assert len(()) == 0
assert len((1, 2, 3)) == 3
assert len([]) == 0
assert len([1, 2]) == 2
assert len({}) == 0
assert len({'first': 1, 'second': 2}) == 2
assert len({1, 2, 3}) == 3
# ---
# case: user length protocol
class Sized:
    def __init__(self, size):
        self.size = size

    def __len__(self):
        return self.size

value = Sized(4)
value.__len__ = None
assert len(value) == 4

class BooleanLength:
    def __len__(self):
        return True

assert len(BooleanLength()) == 1
# ---
# case: user length exception propagation
class FailingLength:
    def __len__(self):
        raise ValueError('length failed')

caught = None
try:
    len(FailingLength())
except ValueError as exception:
    caught = exception

assert caught is not None
# ---
# case: iter builtin over native values
items = [10, 20]
iterator = iter(items)
assert iter(iterator) is iterator
assert next(iterator) == 10
assert next(iterator) == 20
assert next(iterator, None) is None

text = iter('a\u2603')
assert next(text) == 'a'
assert next(text) == '\u2603'

keys = iter({'first': 1, 'second': 2})
assert next(keys) == 'first'
assert next(keys) == 'second'
# ---
# case: iter builtin over user protocol
class CountingIterator:
    def __init__(self):
        self.current = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == 2:
            raise StopIteration
        self.current += 1
        return self.current

class CountingValues:
    def __iter__(self):
        return CountingIterator()

iterator = iter(CountingValues())
assert next(iterator) == 1
assert next(iterator) == 2
assert next(iterator, None) is None

self_iterator = CountingIterator()
assert iter(self_iterator) is self_iterator
