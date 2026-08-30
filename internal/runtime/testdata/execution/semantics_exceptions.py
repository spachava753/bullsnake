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
# ---
# case: exception handler bindings clear on every exit
normal_cleared = False
try:
    try:
        missing_for_binding
    except NameError as error:
        representation = f'{error!r}'
    error
except NameError:
    normal_cleared = True
assert representation == 'NameError("name \'missing_for_binding\' is not defined")'
assert normal_cleared is True

try:
    missing_before_delete
except NameError as deleted_error:
    del deleted_error
try:
    deleted_error
except NameError:
    deleted_cleared = True
assert deleted_cleared is True

def make_reader():
    try:
        missing_before_return
    except NameError as returned_error:
        def read_error():
            return returned_error
        return read_error

reader = make_reader()
try:
    reader()
except NameError:
    return_cleared = True
assert return_cleared is True

for item in (1,):
    try:
        missing_before_break
    except NameError as break_error:
        break
try:
    break_error
except NameError:
    break_cleared = True
assert break_cleared is True

try:
    try:
        missing_before_secondary
    except NameError as secondary_error:
        another_missing
except NameError:
    secondary_raised = True
try:
    secondary_error
except NameError:
    secondary_cleared = True
assert secondary_raised is True
assert secondary_cleared is True
# ---
# case: finally runs on normal and exceptional paths
normal_finally = False
try:
    value = 42
finally:
    normal_finally = True
assert value == 42
assert normal_finally is True

exception_finally = False
caught_after_finally = False
try:
    try:
        missing_before_finally
    finally:
        exception_finally = True
except NameError:
    caught_after_finally = True
assert exception_finally is True
assert caught_after_finally is True

replacement_caught = False
try:
    try:
        1 // 0
    finally:
        missing_from_finally
except NameError:
    replacement_caught = True
assert replacement_caught is True
# ---
# case: return unwinds final suites from inner to outer
state = 1

def preserve_return_value():
    global state
    try:
        return state
    finally:
        state = 2

assert preserve_return_value() == 1
assert state == 2

def replace_return_value():
    try:
        return 3
    finally:
        return 4

assert replace_return_value() == 4

order = 0

def nested_return():
    global order
    try:
        try:
            return 5
        finally:
            order = order * 10 + 1
    finally:
        order = order * 10 + 2

assert nested_return() == 5
assert order == 12

def suppress_exception():
    try:
        missing_before_return_from_finally
    finally:
        return 6

assert suppress_exception() == 6

binding_seen = False

def final_inside_handler():
    global binding_seen
    try:
        missing_before_bound_return
    except NameError as error:
        try:
            return 7
        finally:
            binding_seen = error is error

assert final_inside_handler() == 7
assert binding_seen is True

binding_cleared = False

def handler_inside_final():
    global binding_cleared
    try:
        try:
            missing_before_outer_final
        except NameError as error:
            return 8
    finally:
        try:
            error
        except UnboundLocalError:
            binding_cleared = True

assert handler_inside_final() == 8
assert binding_cleared is True
