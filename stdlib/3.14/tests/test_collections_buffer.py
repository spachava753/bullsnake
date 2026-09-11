# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Buffer

for source in [b'ab', bytearray(b'ab'), memoryview(b'ab')]:
    assert isinstance(source, Buffer)
    assert Buffer.__subclasshook__(type(source)) is True
    view = source.__buffer__(0)
    assert isinstance(view, memoryview)
    assert bytes(view) == b'ab'
    assert view.obj is source
    view.release()
assert not isinstance([], Buffer)
assert not isinstance('text', Buffer)

class Exporter(Buffer):
    def __buffer__(self, flags):
        return memoryview(b'exported')
assert bytes(Exporter().__buffer__(0)) == b'exported'
class Disabled:
    __buffer__ = None
assert not isinstance(Disabled(), Buffer)
try:
    Buffer()
    assert False
except TypeError:
    pass
try:
    Buffer.__buffer__(None, 0)
    assert False
except NotImplementedError:
    pass
