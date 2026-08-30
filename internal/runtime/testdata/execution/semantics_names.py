# Runtime execution cases for names.
# case: deleted name bindings
module_value = 1
del module_value
module_value = 2
marker = 1
def reset_global():
    global marker
    del marker
    marker = 3
    return marker
global_result = reset_global()
def reset_local():
    item = 1
    del item
    item = 4
    return item
local_result = reset_local()
assert f'{module_value!r}' == "2", "module_value"
assert f'{marker!r}' == "3", "marker"
assert f'{global_result!r}' == "3", "global_result"
assert f'{local_result!r}' == "4", "local_result"
