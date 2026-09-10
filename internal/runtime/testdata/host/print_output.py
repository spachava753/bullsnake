# print initializes sys on demand and uses only its configured output.
assert print('hé🙂', 7, sep='|', end='!', flush=True) is None
import sys
assert not sys.stdout.closed
sys.stdout.close()
try:
    print('closed')
    assert False
except ValueError:
    pass
