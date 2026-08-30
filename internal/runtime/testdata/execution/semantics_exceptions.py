# Structured exception handling execution cases.
# case: bare handler catches local exception
caught = False
try:
    assert False, 'handled'
except:
    caught = True
assert caught is True
# ---
# case: normal try path skips handler
handled = False
try:
    result = 40 + 2
except:
    handled = True
assert result == 42
assert handled is False
# ---
# case: bare handler catches across function frame
def fail():
    return missing

caught = False
try:
    fail()
except:
    caught = True
assert caught is True
# ---
# case: nested bare handlers use innermost protection
inner = False
outer = False
try:
    try:
        missing
    except:
        inner = True
except:
    outer = True
assert inner is True
assert outer is False

escaped = False
try:
    try:
        missing_again
    except:
        handler_missing
except:
    escaped = True
assert escaped is True
# ---
# case: typed handlers match classes and tuples
assertion = False
try:
    assert False, 'typed'
except AssertionError:
    assertion = True
assert assertion is True

selected = ''
try:
    missing_typed
except TypeError:
    selected = 'type'
except NameError:
    selected = 'name'
assert selected == 'name'

tuple_selected = False
try:
    {}['missing']
except (TypeError, KeyError):
    tuple_selected = True
assert tuple_selected is True
# ---
# case: typed handlers honor exception inheritance
def read_empty_local():
    return local
    local = 1

name_parent = False
try:
    read_empty_local()
except NameError:
    name_parent = True
assert name_parent is True

exception_parent = False
try:
    1 // 0
except Exception:
    exception_parent = True
assert exception_parent is True
# ---
# case: try else runs only after normal body completion
normal = False
handled = False
try:
    value = 42
except:
    handled = True
else:
    normal = True
assert value == 42
assert normal is True
assert handled is False

caught = False
skipped = True
try:
    missing_in_body
except:
    caught = True
else:
    skipped = False
assert caught is True
assert skipped is True

outer = False
inner = False
try:
    try:
        pass
    except:
        inner = True
    else:
        missing_in_else
except NameError:
    outer = True
assert inner is False
assert outer is True

def returns_from_body():
    try:
        return 1
    except:
        return 2
    else:
        return 3

assert returns_from_body() == 1
# ---
# case: bare raise uses the active handled exception
reraised = False
try:
    try:
        missing_for_reraise
    except NameError:
        raise
except NameError:
    reraised = True
assert reraised is True

def reraiser():
    raise

called = False
try:
    missing_for_called_reraise
except NameError:
    try:
        reraiser()
    except NameError:
        called = True
assert called is True

restored = False
try:
    missing_outer_exception
except NameError:
    try:
        try:
            1 // 0
        except ZeroDivisionError:
            pass
        raise
    except NameError:
        restored = True
assert restored is True
