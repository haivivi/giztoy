-- Luau Type System Demo
-- Showcases Luau's gradual typing features

-- =============================================================================
-- Basic Type Annotations
-- =============================================================================

local name: string = "Luau"
local version: number = 0.706
local isTyped: boolean = true

-- =============================================================================
-- Type Aliases
-- =============================================================================

type Point = {x: number, y: number}
type Size = {width: number, height: number}
type Rectangle = {origin: Point, size: Size}

local function createPoint(x: number, y: number): Point
    return {x = x, y = y}
end

local function createRect(x: number, y: number, w: number, h: number): Rectangle
    return {
        origin = {x = x, y = y},
        size = {width = w, height = h}
    }
end

-- =============================================================================
-- Union Types
-- =============================================================================

type StringOrNumber = string | number
type OptionalString = string?  -- shorthand for string | nil

local function stringify(value: StringOrNumber): string
    if type(value) == "number" then
        return tostring(value)
    end
    return value
end

-- =============================================================================
-- Function Types
-- =============================================================================

type Callback = (number, number) -> number
type Predicate<T> = (T) -> boolean

local add: Callback = function(a, b)
    return a + b
end

local isPositive: Predicate<number> = function(n)
    return n > 0
end

-- =============================================================================
-- Generic Types
-- =============================================================================

type Array<T> = {T}
type Map<K, V> = {[K]: V}
type Optional<T> = T?

local numbers: Array<number> = {1, 2, 3, 4, 5}
local scores: Map<string, number> = {
    alice = 100,
    bob = 85,
    charlie = 92
}

-- =============================================================================
-- Table Types with Methods
-- =============================================================================

type Vector2 = {
    x: number,
    y: number,
    magnitude: (self: Vector2) -> number,
    normalize: (self: Vector2) -> Vector2,
}

local function createVector(x: number, y: number): Vector2
    local v: Vector2 = {
        x = x,
        y = y,
        magnitude = function(self: Vector2): number
            return math.sqrt(self.x * self.x + self.y * self.y)
        end,
        normalize = function(self: Vector2): Vector2
            local m = math.sqrt(self.x * self.x + self.y * self.y)
            return createVector(self.x / m, self.y / m)
        end
    }
    return v
end

-- =============================================================================
-- Typeof Operator
-- =============================================================================

local sample = {name = "test", value = 42}
type SampleType = typeof(sample)

local another: SampleType = {name = "copy", value = 100}

-- =============================================================================
-- Intersection Types (for mixins)
-- =============================================================================

type Named = {name: string}
type Aged = {age: number}
type Person = Named & Aged

local person: Person = {name = "Alice", age = 30}

-- =============================================================================
-- Test Results
-- =============================================================================

local p = createPoint(10, 20)
local rect = createRect(0, 0, 100, 50)
local vec = createVector(3, 4)

result_point = p.x + p.y                    -- 30
result_rect_area = rect.size.width * rect.size.height  -- 5000
result_magnitude = vec:magnitude()          -- 5.0
result_stringify = stringify(42)            -- "42"
result_add = add(10, 20)                    -- 30
result_positive = isPositive(5)             -- true
result_person = person.name .. " is " .. tostring(person.age)  -- "Alice is 30"
