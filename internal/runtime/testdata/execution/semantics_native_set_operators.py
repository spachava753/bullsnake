# case: native set operators preserve mathematical membership and left result kind
left = {1, 2}
right = {2, 3}
assert left | right == {1, 2, 3}
assert left & right == {2}
assert left - right == {1}
assert left ^ right == {1, 3}
assert left == {1, 2}
assert right == {2, 3}
assert left | left is not left
frozen = frozenset(right)
assert type(left | frozen) is set
assert type(frozen | left) is frozenset
assert type(frozen & left) is frozenset
assert frozen - left == frozenset({3})
assert frozen ^ left == frozenset({1, 3})
assert set.__or__(left, right) == {1, 2, 3}
assert left.__and__(right) == {2}
assert set.__rsub__(left, frozen) == frozenset({3})
assert type(set.__rsub__(left, frozen)) is frozenset
assert type(frozenset.__ror__(frozen, left)) is set
assert set.__and__(left, [1, 2]) is NotImplemented
assert type(next(iter({1} & {True}))) is bool
assert type(next(iter({True} & {1, 2}))) is bool

# ---
# case: mutable set in-place operators retain aliases and accept frozen operands
value = {1, 2}
retained = value
value |= frozenset({2, 3})
assert value is retained
assert value == {1, 2, 3}
value &= {2, 3, 4}
assert value is retained
assert value == {2, 3}
value -= {2}
assert value is retained
assert value == {3}
value ^= {3, 5}
assert value is retained
assert value == {5}
value ^= value
assert value is retained
assert value == set()
frozen = frozenset({1})
retained = frozen
frozen |= {2}
assert frozen == frozenset({1, 2})
assert retained == frozenset({1})
assert type(frozen) is frozenset

# ---
# case: unsupported native set operands decline to Python reflected methods
class Reflected:
    def __rand__(self, left):
        assert left == {1}
        return 'reflected'
assert {1} & Reflected() == 'reflected'
try:
    {1} & [1]
    assert False
except TypeError as error:
    assert str(error) == "unsupported operand type(s) for &: 'set' and 'list'"
