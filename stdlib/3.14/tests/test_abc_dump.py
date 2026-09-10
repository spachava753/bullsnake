# Project-owned regression against unchanged CPython 3.14.7 abc.py,
# commit 823f0323ee6ec1402088b73bce1a38473cac36dc.
import sys
from abc import ABC, get_cache_token
class Dump(ABC):
    pass
class Output:
    def __init__(self):
        self.parts = []
    def write(self, text):
        self.parts.append(text)

Dump.register(int)
assert issubclass(bool, Dump)
assert not issubclass(str, Dump)
token = get_cache_token()
expected = (
    'Class: test_abc_dump.Dump\n'
    f'Inv. counter: {token}\n'
    "_abc_registry: {<weakref; to <class 'int'>>}\n"
    "_abc_cache: {<weakref; to <class 'bool'>>}\n"
    "_abc_negative_cache: {<weakref; to <class 'str'>>}\n"
    f'_abc_negative_cache_version: {token}\n'
)
explicit = Output()
assert Dump._dump_registry(file=explicit) is None
assert ''.join(explicit.parts) == expected
redirected = Output()
sys.stdout = redirected
assert Dump._dump_registry() is None
assert ''.join(redirected.parts) == expected
assert ''.join(explicit.parts) == expected
