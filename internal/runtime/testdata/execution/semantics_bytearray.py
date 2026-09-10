# case: mutable byte buffers copy input and immutable snapshots
b = bytearray(b'abc')
assert type(b) is bytearray
assert len(b) == 3 and bool(b)
assert bytearray(3) == b'\0\0\0'
assert bytes() == b''
snapshot = bytes(b)
b[1] = 255
assert b == b'a\xffc'
assert b'abc' == snapshot
assert list(b) == [97, 255, 99]
copy = bytearray(b)
copy[0] = 0
assert b[0] == 97 and copy[0] == 0
assert not bytearray()
# ---
# case: bytearray slice assignment supports binary readinto implementations
b = bytearray(6)
b[:3] = b'abc'
assert bytes(b) == b'abc\0\0\0'
assert b[1:3] == bytearray(b'bc')
b[1:3] = b'longer'
assert b == b'alonger\0\0\0'
b[:] = b'abcdef'
b[::2] = b'XYZ'
assert b == b'XbYdZf'
b[::-2] = b'123'
assert b == b'X3Y2Z1'
b[:] = b[::-1]
assert b == b'1Z2Y3X'
del b[1::2]
assert b == b'123'
del b[-1]
assert b == b'12'
# ---
# case: invalid buffer changes preserve the previous bytes
b = bytearray(b'kept')
for call, kind in ((lambda: bytearray(-1), ValueError),
                   (lambda: bytearray(1 << 100), OverflowError),
                   (lambda: bytearray('text'), TypeError)):
    try:
        call()
        assert False
    except kind:
        pass
try:
    b[0] = 256
    assert False
except ValueError:
    pass
try:
    b[::2] = b'x'
    assert False
except ValueError:
    pass
try:
    hash(b)
    assert False
except TypeError:
    pass
assert b == b'kept'
