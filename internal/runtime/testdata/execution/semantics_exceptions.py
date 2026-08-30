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
# ---
# case: loop control unwinds final suites from inner to outer
break_order = 0
for item in (1, 2):
    try:
        break_order = break_order * 10 + item
        break
    finally:
        break_order = break_order * 10 + 9
else:
    break_order = -1
assert break_order == 19

continue_order = 0
for item in (1, 2):
    try:
        continue_order = continue_order * 10 + item
        continue
    finally:
        continue_order = continue_order * 10 + 9
else:
    continue_order = continue_order * 10 + 8
assert continue_order == 19298

nested_order = 0
for item in (1,):
    try:
        try:
            break
        finally:
            nested_order = nested_order * 10 + 1
    finally:
        nested_order = nested_order * 10 + 2
assert nested_order == 12

visits = 0
for item in (1, 2):
    try:
        break
    finally:
        visits += 1
        if item == 1:
            continue
assert visits == 2

break_suppressed_exception = False
for item in (1,):
    try:
        missing_before_break_from_finally
    finally:
        break
else:
    missing_break_else
break_suppressed_exception = True
assert break_suppressed_exception is True

continued = 0
continue_else = False
for item in (1, 2):
    try:
        missing_before_continue_from_finally
    finally:
        continued += 1
        continue
else:
    continue_else = True
assert continued == 2
assert continue_else is True

binding_seen_during_break = False
for item in (1,):
    try:
        missing_before_bound_break
    except NameError as error:
        try:
            break
        finally:
            binding_seen_during_break = error is error
assert binding_seen_during_break is True

binding_cleared_before_final = False
for item in (1,):
    try:
        try:
            missing_before_outer_loop_final
        except NameError as error:
            break
    finally:
        try:
            error
        except NameError:
            binding_cleared_before_final = True
assert binding_cleared_before_final is True
# ---
# case: combined handlers and finally compose all exits
normal_order = 0
try:
    normal_order = 1
except:
    normal_order = -1
else:
    normal_order = normal_order * 10 + 2
finally:
    normal_order = normal_order * 10 + 3
assert normal_order == 123

handled_order = 0
try:
    missing_for_combined_handler
except NameError:
    handled_order = 4
finally:
    handled_order = handled_order * 10 + 5
assert handled_order == 45

unmatched_final = False
unmatched_caught = False
try:
    try:
        1 // 0
    except NameError:
        missing_unmatched_handler
    finally:
        unmatched_final = True
except ZeroDivisionError:
    unmatched_caught = True
assert unmatched_final is True
assert unmatched_caught is True

handler_failure_final = False
handler_failure_caught = False
try:
    try:
        missing_before_failing_handler
    except NameError:
        another_missing_from_handler
    finally:
        handler_failure_final = True
except NameError:
    handler_failure_caught = True
assert handler_failure_final is True
assert handler_failure_caught is True

else_failure_final = False
else_failure_caught = False
try:
    try:
        pass
    except:
        missing_unused_handler
    else:
        missing_from_combined_else
    finally:
        else_failure_final = True
except NameError:
    else_failure_caught = True
assert else_failure_final is True
assert else_failure_caught is True

return_final_count = 0

def return_from_combined_body():
    global return_final_count
    try:
        return 6
    except:
        return -1
    finally:
        return_final_count += 1

assert return_from_combined_body() == 6
assert return_final_count == 1

binding_cleared_in_combined_final = False

def return_from_combined_handler():
    global binding_cleared_in_combined_final
    try:
        missing_before_combined_return
    except NameError as error:
        return 7
    finally:
        try:
            error
        except UnboundLocalError:
            binding_cleared_in_combined_final = True

assert return_from_combined_handler() == 7
assert binding_cleared_in_combined_final is True
# ---
# case: exceptional finally exposes the pending exception to bare raise
same_exception = False
try:
    try:
        try:
            missing_before_finally_reraise
        except NameError as original:
            saved_exception = original
            raise
    finally:
        raise
except NameError as reraised:
    same_exception = reraised is saved_exception
assert same_exception is True

nested_finally_reraise = False
try:
    try:
        try:
            missing_before_nested_finally
        finally:
            raise
    finally:
        raise
except NameError:
    nested_finally_reraise = True
assert nested_finally_reraise is True

def suppress_with_return():
    try:
        missing_before_suppressed_finally
    finally:
        return 9

assert suppress_with_return() == 9
no_stale_exception = False
try:
    raise
except RuntimeError:
    no_stale_exception = True
assert no_stale_exception is True

continued_after_exception = 0
for item in (1,):
    try:
        missing_before_final_continue_cleanup
    finally:
        continued_after_exception += 1
        continue
assert continued_after_exception == 1
no_loop_exception = False
try:
    raise
except RuntimeError:
    no_loop_exception = True
assert no_loop_exception is True
# ---
# case: explicit exception causes retain Python state
cause = TypeError('inner')
instance_cause = False
try:
    raise ValueError('outer') from cause
except ValueError as error:
    instance_cause = error.__cause__ is cause
    assert error.__context__ is None
    assert error.__suppress_context__ is True
assert instance_cause is True

class_cause = False
try:
    raise ValueError from TypeError
except ValueError as error:
    class_cause = f'{error.__cause__!r}' == 'TypeError("")'
    assert error.__suppress_context__ is True
assert class_cause is True

none_cause = False
try:
    raise ValueError('hidden') from None
except ValueError as error:
    none_cause = error.__cause__ is None
    assert error.__context__ is None
    assert error.__suppress_context__ is True
assert none_cause is True

ordinary_raise = False
try:
    raise ValueError('plain')
except ValueError as error:
    ordinary_raise = error.__cause__ is None
    assert error.__context__ is None
    assert error.__suppress_context__ is False
assert ordinary_raise is True
# ---
# case: raised exceptions link the active handled context
handler_context = False
try:
    try:
        1 // 0
    except ZeroDivisionError as original:
        saved_handler_context = original
        missing_from_handler_context
except NameError as error:
    handler_context = error.__context__ is saved_handler_context
    assert error.__cause__ is None
    assert error.__suppress_context__ is False
assert handler_context is True

def fail_in_called_frame():
    missing_from_called_context

cross_frame_context = False
try:
    try:
        1 // 0
    except ZeroDivisionError as original:
        saved_cross_frame_context = original
        fail_in_called_frame()
except NameError as error:
    cross_frame_context = error.__context__ is saved_cross_frame_context
assert cross_frame_context is True

pending = NameError('pending')
finally_context = False
try:
    try:
        raise pending
    finally:
        1 // 0
except ZeroDivisionError as error:
    finally_context = error.__context__ is pending
assert finally_context is True

explicit_context = False
try:
    try:
        missing_before_explicit_context
    except NameError as original:
        saved_explicit_context = original
        raise ValueError('outer') from TypeError('cause')
except ValueError as error:
    explicit_context = error.__context__ is saved_explicit_context
    assert f'{error.__cause__!r}' == 'TypeError("cause")'
    assert error.__suppress_context__ is True
assert explicit_context is True

suppressed_context = False
try:
    try:
        missing_before_suppressed_context
    except NameError as original:
        saved_suppressed_context = original
        raise ValueError('outer') from None
except ValueError as error:
    suppressed_context = error.__context__ is saved_suppressed_context
    assert error.__cause__ is None
    assert error.__suppress_context__ is True
assert suppressed_context is True

first = ValueError('first')
second = TypeError('second')
cycle_broken = False
try:
    raise first
except ValueError:
    try:
        raise second
    except TypeError:
        try:
            raise first
        except ValueError as reraised:
            cycle_broken = reraised.__context__ is second
            assert second.__context__ is None
assert cycle_broken is True
# ---
# case: user exception classes raise and match through inheritance
class Problem(Exception):
    pass

class SpecificProblem(Problem):
    pass

problem = Problem('broken')
assert f'{problem!r}' == 'Problem("broken")'

caught_identity = False
try:
    raise problem
except Problem as caught:
    caught_identity = caught is problem
assert caught_identity is True

caught_parent = False
try:
    raise SpecificProblem('specific')
except Problem:
    caught_parent = True
assert caught_parent is True

caught_builtin_parent = False
try:
    raise SpecificProblem
except Exception as caught:
    caught_builtin_parent = f'{caught!r}' == 'SpecificProblem("")'
assert caught_builtin_parent is True

caught_tuple = False
try:
    raise SpecificProblem('tuple')
except (TypeError, Problem):
    caught_tuple = True
assert caught_tuple is True

custom_cause = SpecificProblem('cause')
caught_custom_cause = False
try:
    raise Problem('outer') from custom_cause
except Problem as caught:
    caught_custom_cause = caught.__cause__ is custom_cause
assert caught_custom_cause is True
# ---
# case: exception groups construct and raise as ordinary exceptions
first_group_error = ValueError('bad')
second_group_error = TypeError('wrong')
group = ExceptionGroup('batch', [first_group_error, second_group_error])
assert group.message == 'batch'
assert group.exceptions is group.exceptions
assert group.exceptions[0] is first_group_error
assert group.exceptions[1] is second_group_error
assert f'{group!r}' == 'ExceptionGroup("batch", [ValueError("bad"), TypeError("wrong")])'

nested_group = ExceptionGroup('nested', (group, ValueError('later')))
assert nested_group.exceptions[0] is group

caught_group = False
try:
    raise group
except ExceptionGroup as caught:
    caught_group = caught is group
assert caught_group is True

caught_as_exception = False
try:
    raise group
except Exception:
    caught_as_exception = True
assert caught_as_exception is True
# ---
# case: base exception groups select their runtime class from their children
ordinary_group = BaseExceptionGroup('ordinary', [ValueError('recoverable')])
narrowed_to_exception_group = False
try:
    raise ordinary_group
except ExceptionGroup as caught:
    narrowed_to_exception_group = caught is ordinary_group
assert narrowed_to_exception_group is True

fatal = BaseException('stop')
mixed_group = BaseExceptionGroup('mixed', (ValueError('recoverable'), fatal))
assert mixed_group.message == 'mixed'
assert mixed_group.exceptions[1] is fatal
caught_as_base_group = False
caught_as_exception = False
try:
    raise mixed_group
except Exception:
    caught_as_exception = True
except BaseExceptionGroup as caught:
    caught_as_base_group = caught is mixed_group
assert caught_as_exception is False
assert caught_as_base_group is True

class CustomGroup(ExceptionGroup):
    pass

custom_group = CustomGroup('custom', [ValueError('child')])
caught_custom_group = False
try:
    raise custom_group
except CustomGroup as caught:
    caught_custom_group = caught is custom_group
assert caught_custom_group is True
assert custom_group.message == 'custom'

class CustomBaseGroup(BaseExceptionGroup):
    pass

custom_base_group = CustomBaseGroup('custom base', [BaseException('child')])
caught_custom_base_group = False
try:
    raise custom_base_group
except CustomBaseGroup as caught:
    caught_custom_base_group = caught is custom_base_group
assert caught_custom_base_group is True
# ---
# case: except star splits nested groups and runs every matching clause
outer_value = ValueError('outer value')
inner_type = TypeError('inner type')
inner_value = ValueError('inner value')
inner_group = ExceptionGroup('inner', [inner_type, inner_value])
original_group = ExceptionGroup('outer', [outer_value, inner_group])
value_match = None
type_match = None
try:
    raise original_group
except* ValueError as caught:
    value_match = caught
except* TypeError as caught:
    type_match = caught
assert value_match.message == 'outer'
assert value_match.exceptions[0] is outer_value
assert value_match.exceptions[1].message == 'inner'
assert value_match.exceptions[1].exceptions[0] is inner_value
assert type_match.message == 'outer'
assert type_match.exceptions[0].message == 'inner'
assert type_match.exceptions[0].exceptions[0] is inner_type
# ---
# case: except star wraps a naked exception and runs else only without failure
naked = ValueError('naked')
wrapped = None
try:
    raise naked
except* ValueError as caught:
    wrapped = caught
assert wrapped.message == ''
assert wrapped.exceptions[0] is naked

else_ran = False
try:
    completed = True
except* Exception:
    completed = False
else:
    else_ran = True
assert completed is True
assert else_ran is True
# ---
# case: except star keeps dispatching after a handler raises
value_leaf = ValueError('value')
type_leaf = TypeError('type')
source_group = ExceptionGroup('source', [value_leaf, type_leaf])
replacement = KeyError('replacement')
type_handler_ran = False
replacement_identity = False
try:
    try:
        raise source_group
    except* ValueError:
        raise replacement
    except* TypeError:
        type_handler_ran = True
except KeyError as caught:
    replacement_identity = caught is replacement
assert type_handler_ran is True
assert replacement_identity is True
# ---
# case: except star propagates unmatched leaves and recombines bare reraises
unmatched_value = ValueError('handled')
unmatched_type = TypeError('unmatched')
unmatched_source = ExceptionGroup('unmatched source', [unmatched_value, unmatched_type])
handled_value = False
try:
    try:
        raise unmatched_source
    except* ValueError:
        handled_value = True
except ExceptionGroup as rest:
    assert rest.message == 'unmatched source'
    assert rest.exceptions[0] is unmatched_type
assert handled_value is True

reraised_value = ValueError('reraised')
handled_type = TypeError('handled')
reraise_source = ExceptionGroup('reraise source', [reraised_value, handled_type])
type_was_handled = False
try:
    try:
        raise reraise_source
    except* ValueError:
        raise
    except* TypeError:
        type_was_handled = True
except ExceptionGroup as reraised:
    assert reraised.message == 'reraise source'
    assert reraised.exceptions[0] is reraised_value
assert type_was_handled is True
# ---
# case: except star composes with finally and base exception groups
base_leaf = BaseException('base')
base_source = BaseExceptionGroup('base source', [base_leaf])
base_was_handled = False
finalized = False
try:
    try:
        raise base_source
    except* BaseException as caught:
        base_was_handled = caught.exceptions[0] is base_leaf
finally:
    finalized = True
assert base_was_handled is True
assert finalized is True

ordinary_leaf = ValueError('ordinary')
base_only_leaf = BaseException('base only')
mixed_source = BaseExceptionGroup('mixed source', [ordinary_leaf, base_only_leaf])
ordinary_part = None
base_part = None
try:
    raise mixed_source
except* Exception as caught:
    ordinary_part = caught
except* BaseException as caught:
    base_part = caught
assert ordinary_part.exceptions[0] is ordinary_leaf
assert base_part.exceptions[0] is base_only_leaf
ordinary_part_is_exception_group = False
try:
    raise ordinary_part
except ExceptionGroup:
    ordinary_part_is_exception_group = True
assert ordinary_part_is_exception_group is True
base_part_is_exception_group = False
try:
    raise base_part
except ExceptionGroup:
    base_part_is_exception_group = True
except BaseExceptionGroup:
    pass
assert base_part_is_exception_group is False
