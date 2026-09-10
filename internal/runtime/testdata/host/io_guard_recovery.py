import builtins
builtins.guarded_reader.raw.fail = False
assert builtins.guarded_reader.read(1) == b'x'
