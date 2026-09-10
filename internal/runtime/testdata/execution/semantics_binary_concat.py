# case: bytes concatenate buffers without changing prior values
a = b'ab'
original = a
a += b'cd'
assert a == b'abcd' and original == b'ab'
assert b'x' + bytearray(b'y') == b'xy'
assert b'x' + memoryview(b'y') == b'xy'
class Reflected:
    def __radd__(self, other):
        return other
assert b'x' + Reflected() == b'x'

# ---
# case: bytearray concatenation preserves snapshots and export restrictions
a = bytearray(b'ab')
b = a + b'cd'
assert isinstance(b, bytearray) and b == b'abcd' and a == b'ab'
alias = a
try:
    a += a
    assert False
except BufferError:
    pass
a += b'ab'
assert a is alias and a == b'abab'
view = memoryview(a)
try:
    a += b'x'
    assert False
except BufferError:
    pass
assert a == b'abab'
a += b''
view.release()
a += b'x'
assert a == b'ababx'
