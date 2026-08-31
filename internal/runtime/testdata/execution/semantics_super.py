# Super lookup behavior.
# case: zero and explicit argument super
class SuperBase:
    def __init__(self, value):
        self.value = value
    def total(self, amount):
        return self.value + amount

class SuperChild(SuperBase):
    def total(self, amount):
        return super().total(amount) + 1

class SuperGrandchild(SuperChild):
    pass

child = SuperChild(10)
grandchild = SuperGrandchild(20)
zero_child = child.total(2)
zero_grandchild = grandchild.total(3)
explicit_instance = super(SuperChild, grandchild).total(4)
parent_function = super(SuperChild, SuperChild).total
explicit_class = parent_function(grandchild, 5)
proxy = super(SuperChild, grandchild)

assert zero_child == 13
assert zero_grandchild == 24
assert explicit_instance == 24
assert explicit_class == 25
assert proxy.__thisclass__ is SuperChild
assert proxy.__self__ is grandchild
assert proxy.__self_class__ is SuperGrandchild
# ---
# case: super applies descriptors
class SuperDescriptor:
    def __get__(self, instance, owner):
        if instance is None:
            return owner
        return (instance.marker, owner)

class DescriptorBase:
    field = SuperDescriptor()
    @property
    def doubled(self):
        return self.marker * 2

class DescriptorChild(DescriptorBase):
    def inherited_field(self):
        return super().field
    def inherited_property(self):
        return super().doubled

class DescriptorGrandchild(DescriptorChild):
    pass

value = DescriptorGrandchild()
value.marker = 7
field = value.inherited_field()
property_value = value.inherited_property()
class_field = super(DescriptorChild, DescriptorChild).field

assert field[0] == 7
assert field[1] is DescriptorGrandchild
assert property_value == 14
assert class_field is DescriptorChild
