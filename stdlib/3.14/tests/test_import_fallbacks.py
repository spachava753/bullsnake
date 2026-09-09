# Bullsnake regression tests for selected unchanged CPython 3.14.7 fallbacks.
# These assertions are project tests, not adapted upstream unittest methods.
import operator
import keyword
import heapq

assert operator.add(4, 5) == 9
assert operator.itemgetter(1)(['first', 'second']) == 'second'
assert keyword.iskeyword('class')
assert not keyword.iskeyword('classify')
assert keyword.issoftkeyword('match')
assert not keyword.iskeyword('match')
heap = [4, 1, 3, 2]
heapq.heapify(heap)
heapq.heappush(heap, 0)
assert [heapq.heappop(heap) for _ in range(5)] == [0, 1, 2, 3, 4]
