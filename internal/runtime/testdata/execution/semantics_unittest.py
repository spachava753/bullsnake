# Go-backed unittest behavior needed by the first CPython conformance tests.
# case: future directive metadata is available
from __future__ import nested_scopes, division
assert nested_scopes is not None
assert division is not None

# ---
# case: discovers and executes test cases
import unittest

events = 0

class PassingCase(unittest.TestCase):
    def setUp(self):
        global events
        self.value = 0
        events = events * 10 + 1

    def test_assertions(self):
        global events
        self.assertEqual(7 // 2, 3)
        self.assertAlmostEqual(7 / 2, 3.5)
        self.assertIsInstance('literal', str)
        self.assertEqual(self.value, 0)
        self.value = 1
        events = events * 10 + 2

    def test_fresh_instance(self):
        global events
        self.assertEqual(self.value, 0)
        events = events * 10 + 4

    def tearDown(self):
        global events
        events = events * 10 + 3

unittest.main()
assert events == 123143

# ---
# case: failed assertion leaves through Python exception path
# error: AssertionError
# message: "1 != 2"
import unittest

class FailingCase(unittest.TestCase):
    def test_failure(self):
        self.assertEqual(1, 2)

unittest.main()
