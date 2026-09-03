package bullsnake_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spachava753/bullsnake"
	"github.com/spachava753/bullsnake/host"
)

const cpythonUnittestHarness = `
import unittest

from test import test_contains, test_unary
from test.test_unittest import (
    test_assertions,
    test_async_case,
    test_break,
    test_case,
    test_discovery,
    test_functiontestcase,
    test_loader,
    test_program,
    test_result,
    test_runner,
    test_setups,
    test_skipping,
    test_suite,
    test_util,
)
from test.test_unittest.testmock import (
    testasync as testmock_async,
    testcallable,
    testhelpers,
    testmagicmethods,
    testmock,
    testpatch,
    testsealable,
    testsentinel,
    testthreadingmock,
    testwith,
)

modules = (
    test_assertions,
    test_async_case,
    test_break,
    test_case,
    test_discovery,
    test_functiontestcase,
    test_loader,
    test_suite,
    test_result,
    test_program,
    test_runner,
    test_setups,
    test_skipping,
    test_util,
    test_unary,
    test_contains,
    testmock_async,
    testcallable,
    testhelpers,
    testmagicmethods,
    testmock,
    testpatch,
    testsealable,
    testsentinel,
    testthreadingmock,
    testwith,
)

suite = unittest.TestSuite()
for test_module in modules:
    suite.addTests(unittest.defaultTestLoader.loadTestsFromModule(test_module))

result = unittest.TestResult()
suite.run(result)
tests_run = result.testsRun
successful = result.wasSuccessful()
failures = result.failures
errors = result.errors
failure_ids = [test.id() for test, _ in failures]
error_ids = [test.id() for test, _ in errors]
`

// TestCPythonUnittestCompatibility executes the pinned, unchanged CPython
// unittest and unittest.mock self-tests plus two language test modules when a
// checkout is supplied.
func TestCPythonUnittestCompatibility(t *testing.T) {
	cpython := testingCPythonCheckout(t)
	interpreter := bullsnake.New(bullsnake.Config{
		Host:             host.Default(),
		ModuleSearchPath: []string{filepath.Join(cpython, "Lib")},
	})
	module, err := interpreter.ExecuteModule(
		"bullsnake_cpython_tests",
		"bullsnake_cpython_tests.py",
		cpythonUnittestHarness,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "tests_run", "1104")
	assertModuleRepr(t, module, "failure_ids", "[]")
	assertModuleRepr(t, module, "error_ids", "[]")
	assertModuleRepr(t, module, "successful", "True")
}

func testingCPythonCheckout(t *testing.T) string {
	t.Helper()
	checkout := os.Getenv("BULLSNAKE_CPYTHON")
	if checkout == "" {
		t.Skip("set BULLSNAKE_CPYTHON to the pinned CPython checkout")
	}
	return checkout
}

func assertModuleRepr(t *testing.T, module *bullsnake.Module, name, expected string) {
	t.Helper()
	value, found := module.Get(name)
	if !found {
		t.Fatalf("module result %q is missing", name)
	}
	if actual := value.Repr(); actual != expected {
		t.Fatalf("%s = %s, want %s", name, actual, expected)
	}
}
