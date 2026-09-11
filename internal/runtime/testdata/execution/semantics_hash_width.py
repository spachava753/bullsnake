# case: user hash results preserve signed hash-width integers
class Hashed:
    def __init__(self, value):
        self.value = value
    def __hash__(self):
        return self.value
for value in [(1 << 62) + 1, -(1 << 62) - 1, (1 << 63) - 1, -(1 << 63), 0]:
    assert hash(Hashed(value)) == value
assert hash(Hashed(-1)) == -2
assert hash(Hashed(1 << 80)) == hash(1 << 80)

# ---
# case: sys maxsize reflects supported native index width
import sys
assert sys.maxsize == (1 << 63) - 1
assert len(range(sys.maxsize)) == sys.maxsize
try:
    len(range(sys.maxsize + 1))
    assert False
except OverflowError:
    pass
