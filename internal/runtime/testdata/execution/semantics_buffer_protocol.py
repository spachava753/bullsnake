# case: buffer descriptors enforce writable and export lifetime rules
source = bytearray(b'ab')
view = source.__buffer__(1)
assert view.obj is source
view[0] = 99
assert bytes(source) == b'cb'
try:
    source += b'x'
    assert False
except BufferError:
    pass
source.__release_buffer__(view)
source.__release_buffer__(view)
source += b'x'
assert bytes(source) == b'cbx'
try:
    b'ab'.__buffer__(1)
    assert False
except BufferError:
    pass
try:
    bytearray.__buffer__(b'ab', 0)
    assert False
except TypeError:
    pass

# ---
# case: memoryview buffer exports pin their source until all child views release
source = memoryview(bytearray(b'ab'))
exported = source.__buffer__(0)
assert exported.obj is source
child = memoryview(exported)
exported.release()
try:
    source.release()
    assert False
except BufferError:
    pass
child.release()
source.release()
try:
    source.__buffer__(0)
    assert False
except ValueError:
    pass

# ---
# case: buffer flags use the index protocol and enforce contiguous requests
class Flags:
    def __index__(self):
        return 0
view = b'ab'.__buffer__(Flags())
assert bytes(view) == b'ab'
view.release()
source = memoryview(bytearray(b'abcd'))[::2]
try:
    source.__buffer__(0)
    assert False
except BufferError:
    pass
exported = source.__buffer__(0x18)
assert bytes(exported) == b'ac'
assert exported.strides == (2,)
source.__release_buffer__(exported)
source.release()
for flags in ['bad', 1 << 40]:
    try:
        b'ab'.__buffer__(flags)
        assert False
    except (TypeError, OverflowError):
        pass

# ---
# case: release buffer checks ownership and memoryview arguments
first, second = bytearray(b'ab'), bytearray(b'cd')
view = first.__buffer__(0)
try:
    second.__release_buffer__(view)
    assert False
except ValueError:
    pass
try:
    first.__release_buffer__(1)
    assert False
except TypeError:
    pass
first.__release_buffer__(view)
assert bytes(first) == b'ab'
