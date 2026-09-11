# case: tuple hashing calls user hashes and caches successful results
calls = []
class Hashed:
    def __init__(self, value):
        self.value = value
    def __hash__(self):
        calls.append(self.value)
        return self.value
first, second = Hashed(7), Hashed(8)
value = (first, (second,))
hashed = hash(value)
assert calls == [7, 8]
assert hash(value) == hashed
assert value.__hash__() == hashed
assert calls == [7, 8]
assert hashed == hash((7, (8,)))
assert hash(()) == 5740354900026072187
assert hash((1, 2, 3)) == 529344067295497451

# ---
# case: failed tuple hashes retry and preserve Python exceptions
class Retry:
    def __init__(self):
        self.ready = False
    def __hash__(self):
        if not self.ready:
            raise ValueError('not ready')
        return 4
item = Retry()
value = (item,)
try:
    hash(value)
    assert False
except ValueError as error:
    assert str(error) == 'not ready'
item.ready = True
assert hash(value) == hash((4,))
try:
    hash((1, []))
    assert False
except TypeError:
    pass

# ---
# case: native nested tuple hashing does not recurse through the Go stack
value = 1
for index in range(10000):
    value = (value,)
hashed = hash(value)
assert hash(value) == hashed
assert hash(frozenset([(1, 2), (3, 4)])) == hash(frozenset([(3, 4), (1, 2)]))
