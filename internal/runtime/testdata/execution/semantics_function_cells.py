# case: function closures expose the actual retained cells
import sys
def factory(value):
    def read():
        return value
    def write(new):
        nonlocal value
        value = new
    return read, write, sys._getframe()
read, write, frame = factory(1)
closure = read.__closure__
assert type(closure) is tuple
assert closure is read.__closure__
cell = closure[0]
assert cell is write.__closure__[0]
assert type(cell).__name__ == 'cell'
assert cell.cell_contents == 1
write(2)
assert cell.cell_contents == 2
cell.cell_contents = 3
assert read() == 3
assert frame.f_locals['value'] == 3
frame.f_locals['value'] = 4
assert cell.cell_contents == 4
assert getattr(read, '__closure__') is closure
assert getattr(cell, 'cell_contents') == 4
setattr(cell, 'cell_contents', 5)
assert read() == 5
assert factory.__closure__ is None

# ---
# case: empty closure cells can be deleted and rebound after frame return
def factory():
    def read():
        return value
    return read
    value = 1
read = factory()
cell = read.__closure__[0]
try:
    cell.cell_contents
    assert False
except ValueError as error:
    assert str(error) == 'Cell is empty'
cell.cell_contents = None
assert read() is None
del cell.cell_contents
delattr(cell, 'cell_contents')
try:
    read()
    assert False
except NameError:
    pass
cell.cell_contents = 12
assert read() == 12
try:
    read.__closure__ = ()
    assert False
except AttributeError:
    pass

# ---
# case: function globals use the actual module dictionary
import sys
module_globals = sys._getframe().f_globals
value = 1
def read():
    return value
assert read.__globals__ is module_globals
assert getattr(read, '__globals__') is module_globals
read.__globals__['value'] = 2
assert read() == 2
value = 3
assert read.__globals__['value'] == 3
del read.__globals__['value']
try:
    read()
    assert False
except NameError:
    pass
try:
    del read.__globals__
    assert False
except AttributeError:
    pass
