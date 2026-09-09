import sys
for operation in [lambda: sys.stdout.write(3), lambda: sys.stdout.write(), lambda: sys.stdout.write(text='x'), lambda: sys.stdout.flush(1), lambda: sys.stdin.read('bad'), lambda: sys.stdin.readline(None), lambda: sys.stdin.readline(size=2)]:
    try:
        operation()
        assert False
    except TypeError:
        pass
