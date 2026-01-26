-- Point class using metatables
-- Tests: OOP patterns, metatables, methods

local Point = {}
Point.__index = Point

function Point.new(x, y)
    local self = setmetatable({}, Point)
    self.x = x
    self.y = y
    return self
end

function Point:distance(other)
    local dx = self.x - other.x
    local dy = self.y - other.y
    return math.sqrt(dx * dx + dy * dy)
end

local p1 = Point.new(0, 0)
local p2 = Point.new(3, 4)
result = p1:distance(p2)
-- Expected: result = 5 (3-4-5 triangle)
