# case: source globals remain live across dictionary and bytecode mutation
import fixture
namespace = fixture.__dict__
first = 1
second = 2
assert namespace['first'] == 1
namespace['first'] = 3
assert first == 3
def change():
    global first, second
    first = 4
    del second
change()
assert namespace['first'] == 4
assert 'second' not in namespace
del first
assert 'first' not in namespace
namespace['first'] = 5
assert first == 5
assert list(namespace)[-1] == 'first'

# ---
# case: clearing a module namespace removes attribute and dir bindings
import sys
namespace = sys.__dict__
namespace['item'] = 1
namespace.clear()
assert len(namespace) == 0
assert not hasattr(sys, 'item')
assert not hasattr(sys, 'argv')
assert 'item' not in dir(sys)
sys.recovered = 2
assert namespace['recovered'] == 2

# ---
# case: module dictionary is shared with attribute and global name operations
import sys
namespace = sys.__dict__
assert type(namespace) is dict
assert namespace is sys.__dict__
namespace['namespace_test'] = 10
assert sys.namespace_test == 10
sys.namespace_test = 20
assert namespace['namespace_test'] == 20
del sys.namespace_test
assert 'namespace_test' not in namespace
namespace['namespace_test'] = 30
namespace.pop('namespace_test')
assert not hasattr(sys, 'namespace_test')
namespace['namespace_test'] = 40
assert 'namespace_test' in dir(sys)
del namespace['namespace_test']
assert 'namespace_test' not in dir(sys)

# ---
# case: builtins module dictionary shares ordinary name lookup
import builtins
namespace = builtins.__dict__
namespace['namespace_marker'] = 123
assert namespace_marker == 123
builtins.namespace_marker = 456
assert namespace_marker == 456
del namespace['namespace_marker']
try:
    namespace_marker
    assert False
except NameError:
    pass

# ---
# case: module dictionaries preserve insertion order after exposure
import sys
namespace = sys.__dict__
sys.namespace_first = 1
sys.namespace_second = 2
del sys.namespace_first
sys.namespace_first = 3
assert list(namespace)[-2:] == ['namespace_second', 'namespace_first']
copy = namespace.copy()
copy.clear()
assert sys.namespace_first == 3
try:
    sys.__dict__ = {}
    assert False
except AttributeError:
    pass
try:
    del sys.__dict__
    assert False
except AttributeError:
    pass
