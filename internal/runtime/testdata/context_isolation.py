import _contextvars
if not hasattr(_contextvars, 'variable'):
    _contextvars.variable = _contextvars.ContextVar('shared', default=None)
variable = _contextvars.variable
before = variable.get()
value = []
token = variable.set(value)
assert variable.get() is value
assert token.var is variable
