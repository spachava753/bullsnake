# case: cmp_to_key missing comparator
# error: TypeError
# message: "cmp_to_key() missing required argument 'mycmp' (pos 1)"
from _functools import cmp_to_key
cmp_to_key()
# ---
# case: cmp_to_key extra comparator
# error: TypeError
# message: "cmp_to_key() takes at most 1 argument (2 given)"
from _functools import cmp_to_key
cmp_to_key(1, 2)
# ---
# case: key wrapper missing object
# error: TypeError
# message: "K() missing required argument 'obj' (pos 1)"
from _functools import cmp_to_key
cmp_to_key(lambda left, right: 0)()
# ---
# case: key wrapper extra object
# error: TypeError
# message: "K() takes at most 1 argument (2 given)"
from _functools import cmp_to_key
cmp_to_key(lambda left, right: 0)(1, 2)
# ---
# case: key wrapper wrong comparison type
# error: TypeError
# message: "other argument must be K instance"
from _functools import cmp_to_key
cmp_to_key(lambda left, right: 0)(1) < 2
# ---
# case: key wrapper non-callable comparator
# error: TypeError
# message: "'int' object is not callable"
from _functools import cmp_to_key
key = cmp_to_key(1)
sorted([2, 1], key=key)
# ---
# case: key wrapper comparator failure
# error: RuntimeError
# message: "comparison failed"
from _functools import cmp_to_key

def failing_compare(left, right):
    raise RuntimeError('comparison failed')

sorted([2, 1], key=cmp_to_key(failing_compare))
