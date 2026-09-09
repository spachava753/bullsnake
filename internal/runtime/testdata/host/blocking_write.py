import sys
try:
    sys.stdout.write('hé🙂')
    assert False
except BlockingIOError as e:
    assert e.errno == 11
    assert e.characters_written == 2
