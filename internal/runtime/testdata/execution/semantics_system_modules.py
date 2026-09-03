# case: sys module metadata and cache
import builtins
from _abc import get_cache_token, _abc_register, _abc_subclasscheck
import sys

assert sys.version_info == (3, 14, 7, 'final', 0)
assert sys.modules['sys'] is sys
assert sys.modules['builtins'] is builtins
assert sys.modules['fixture'].__name__ == 'fixture'
assert 'sys' in sys.builtin_module_names
sys.modules['runtime_alias'] = sys
import runtime_alias
assert runtime_alias is sys
class SampleABC:
    pass
before_registration = get_cache_token()
assert _abc_register(SampleABC, list) is list
assert get_cache_token() == before_registration + 1
assert _abc_subclasscheck(SampleABC, list) is True
assert sys.exc_info() == (None, None, None)

# ---
# case: sys exception state and exit
import sys

try:
    raise ValueError('boom')
except ValueError as caught:
    error_type, error, traceback = sys.exc_info()
    assert error_type is ValueError
    assert error is caught
    assert traceback is not None
    assert traceback.tb_frame is not None

try:
    sys.exit(7)
except SystemExit as exit_error:
    assert exit_error.code == 7

# ---
# case: clock module
import time

before = time.perf_counter()
time.sleep(0)
after = time.monotonic()
assert before >= 0.0
assert after >= before
assert time.time() > 0.0

# ---
# case: in-memory text streams
import io

stream = io.StringIO()
assert stream.write('hello') == 5
assert stream.tell() == 5
assert stream.getvalue() == 'hello'
stream.seek(0)
assert stream.write('H') == 1
assert stream.getvalue() == 'Hello'
assert not stream.closed
stream.close()
assert stream.closed
try:
    stream.getvalue()
except ValueError:
    pass
else:
    raise AssertionError('closed StringIO remained readable')

# ---
# case: os and path modules
import os.path
from os.path import commonprefix

joined = os.path.join('alpha', 'beta.py')
assert joined == os.path.normpath('alpha/beta.py')
assert os.path.basename(joined) == 'beta.py'
assert os.path.dirname(joined) == 'alpha'
assert os.path.splitext(joined) == (os.path.join('alpha', 'beta'), '.py')
assert os.path.normpath('alpha/../beta') == 'beta'
assert commonprefix(['/usr/lib', '/usr/local']) == '/usr/l'
assert os.fspath('alpha') == 'alpha'
assert os.path.isabs(os.getcwd())
# ---
# case: module fallback and partial callables
import fixture
from _functools import partial

def __getattr__(name):
    if name == 'dynamic':
        return 42
    raise AttributeError(name)

def add(left, right):
    return left + right

class BuiltinHolder:
    length = len

assert fixture.dynamic == 42
assert partial(add, 2)(3) == 5
assert BuiltinHolder().length([1, 2, 3]) == 3
