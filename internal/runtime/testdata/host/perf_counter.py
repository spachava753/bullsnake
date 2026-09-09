import time
try:
    time.perf_counter(1)
    assert False
except TypeError:
    pass
try:
    time.perf_counter(unexpected=True)
    assert False
except TypeError:
    pass
start = time.perf_counter()
assert start == 1.5
assert time.perf_counter() - start == 0.25
