# case: aliases compare origins and nested arguments and preserve hash contract
Alias = type(list[int])
assert list[int] == list[int]
assert list[int] != list[str]
assert list[int] != tuple[int]
assert list[int].__eq__(int) is NotImplemented
assert list[int].__ne__(int) is NotImplemented
assert list[dict[str, tuple[int]]] == list[dict[str, tuple[int]]]
assert hash(list[int]) == hash(list) ^ hash((int,))
assert hash(list[dict[str, int]]) == hash(list[dict[str, int]])
class Child(Alias):
    pass
assert Child(list, int) == list[int]
assert hash(Child(list, int)) == hash(list[int])

# ---
# case: alias comparison and hash resume callbacks and propagate errors
calls = []
class Truth:
    def __bool__(self):
        calls.append('truth')
        return True
class Part:
    def __eq__(self, other):
        calls.append('eq')
        return Truth()
    def __hash__(self):
        calls.append('hash')
        return 42
Alias = type(list[int])
a, b = Part(), Part()
assert Alias(a, a) == Alias(b, b)
assert calls == ['eq', 'truth', 'eq', 'truth']
calls = []
value = Alias(a, (b,))
assert hash(value) == 42 ^ hash((42,))
assert calls == ['hash', 'hash']
calls = []
hash(value)
assert calls == ['hash']
class Broken:
    def __eq__(self, other):
        raise ValueError('comparison failed')
    def __hash__(self):
        raise ValueError('hash failed')
for value in [Alias(Broken(), int), Alias(list, Broken())]:
    try:
        value == Alias(list, int)
        assert False
    except ValueError as error:
        assert str(error) == 'comparison failed'
    try:
        hash(value)
        assert False
    except ValueError as error:
        assert str(error) == 'hash failed'
