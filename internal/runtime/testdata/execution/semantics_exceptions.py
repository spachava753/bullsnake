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
