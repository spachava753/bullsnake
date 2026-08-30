# Runtime exception cases for names.
# case: delete missing module name
# error: NameError
# message: "name 'missing' is not defined"
del missing
# ---
# case: delete missing local
# error: UnboundLocalError
# message: "cannot access local variable 'value' where it is not associated with a value"
def clear():
    del value
clear()
# ---
# case: delete missing explicit global
# error: NameError
# message: "name 'absent' is not defined"
def clear():
    global absent
    del absent
clear()
# ---
# case: unbound local cell
# error: UnboundLocalError
# message: "cannot access local variable 'value' where it is not associated with a value"
def outer():
    def read():
        return value
    current = value
    value = 1
    return current
answer = outer()
# ---
# case: unbound free cell
# error: NameError
# message: "cannot access free variable 'value' where it is not associated with a value in enclosing scope"
def outer():
    def read():
        return value
    result = read()
    value = 1
    return result
answer = outer()
# ---
# case: delete empty free cell
# error: NameError
# message: "cannot access free variable 'value' where it is not associated with a value in enclosing scope"
def outer():
    value = 1
    def clear():
        nonlocal value
        del value
        del value
    clear()
answer = outer()
# ---
# case: unbound fast local
# error: UnboundLocalError
# message: "cannot access local variable 'value' where it is not associated with a value"
def read():
    observed = value
    value = 1
    return observed
answer = read()
# ---
# case: missing name
# error: NameError
# message: "name 'missing' is not defined"
answer = missing
# ---
# case: absent imported module
# error: ModuleNotFoundError
# message: "No module named 'absent'"
import absent
# ---
# case: empty tuple loop leaves target unbound
# error: NameError
# message: "name 'absent' is not defined"
for absent in ():
    pass
absent
# ---
# case: empty set loop leaves target unbound
# error: NameError
# message: "name 'absent' is not defined"
for absent in {*()}:
    pass
absent
# ---
# case: empty string loop leaves target unbound
# error: NameError
# message: "name 'absent_text' is not defined"
for absent_text in '':
    pass
absent_text
# ---
# case: empty bytes loop leaves target unbound
# error: NameError
# message: "name 'absent_byte' is not defined"
for absent_byte in b'':
    pass
absent_byte
