# case: memory views share mutable bytes and retain independent release state
b = bytearray(b'abcd')
v = memoryview(b)
assert v.obj is b
assert v.format == 'B' and v.itemsize == 1 and v.ndim == 1
assert v.shape == (4,) and v.strides == (1,) and v.suboffsets == ()
assert v.nbytes == 4 and len(v) == 4
assert v.c_contiguous and v.f_contiguous and v.contiguous
assert not v.readonly
v[1] = 90
assert b == b'aZcd'
v[1:3] = b'XY'
assert b == b'aXYd'
child = v[1:3]
assert child.obj is b and bytes(child) == b'XY'
assert child.tolist() == [88, 89]
v.release()
assert bytes(child) == b'XY'
try:
    b[:] = b'longer'
    assert False
except BufferError:
    pass
child.release()
b[:] = b'longer'
assert b == b'longer'
# ---
# case: readonly and strided views preserve content and assignment rules
v = memoryview(b'abcdef')
assert v.readonly and bytes(v[::-2]) == b'fdb'
assert v[::2].tolist() == [97, 99, 101]
assert v == b'abcdef'
assert hash(v) == hash(b'abcdef')
try:
    v[0] = 1
    assert False
except TypeError:
    pass
b = bytearray(b'abcdef')
with memoryview(b) as writable:
    writable[::2] = b'XYZ'
    assert b == b'XbYdZf'
    readonly = writable.toreadonly()
    assert readonly.readonly
    b[1] = 33
    assert bytes(readonly) == b'X!YdZf'
readonly.release()
b[:] = b'resized'
# ---
# case: released view operations fail and release is idempotent
v = memoryview(bytearray(b'ab'))
v.release()
assert v.release() is None
for call in (lambda: bytes(v), lambda: len(v), lambda: v[0],
             lambda: v.obj, lambda: v.nbytes, v.tobytes, v.__enter__):
    try:
        call()
        assert False
    except ValueError:
        pass
# ---
# case: bytearray replacement cannot invalidate outstanding views
b = bytearray(b'abcd')
v = memoryview(b)
b[1:3] = b'XY'
assert bytes(v) == b'aXYd'
try:
    del b[0]
    assert False
except BufferError:
    pass
try:
    b[::2] = b'x'
    assert False
except ValueError:
    pass
assert bytes(v) == b'aXYd'
v.release()
del b[0]
assert b == b'XYd'

# ---
# case: view order conversion and cached hashes survive explicit release
v = memoryview(b'abc')
assert v.tobytes(order='F') == b'abc'
assert v.tobytes(None) == b'abc'
h = hash(v)
v.release()
assert hash(v) == h
try:
    memoryview(b'abc').tobytes('wrong')
    assert False
except ValueError:
    pass
