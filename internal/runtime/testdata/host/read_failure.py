import sys
assert sys.stdin.read() == 'hé🙂'
try:
    sys.stdin.read()
    assert False
except PermissionError as e:
    assert e.errno == 13
    assert e.filename is None
