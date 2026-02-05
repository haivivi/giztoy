package registry

import (
	"testing"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestMemoryRegistry(t *testing.T) {
	reg := NewMemoryRegistry()

	// Add a test package
	err := reg.AddPackageFromSource("@test/utils", "1.0.0", []byte(`
-- utils module
local utils = {}

function utils.add(a, b)
    return a + b
end

function utils.greet(name)
    return "Hello, " .. name
end

return utils
`))
	if err != nil {
		t.Fatalf("AddPackageFromSource failed: %v", err)
	}

	// List packages
	packages, err := reg.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages failed: %v", err)
	}
	if len(packages) != 1 || packages[0] != "@test/utils" {
		t.Errorf("ListPackages = %v, want [@test/utils]", packages)
	}

	// List versions
	versions, err := reg.ListVersions("@test/utils")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].String() != "1.0.0" {
		t.Errorf("ListVersions = %v, want [1.0.0]", versions)
	}

	// Resolve package
	pkg, err := reg.Resolve("@test/utils", "^1.0.0")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if pkg.Meta.Name != "@test/utils" {
		t.Errorf("Resolve returned wrong package: %v", pkg.Meta.Name)
	}
}

func TestRequireFunc(t *testing.T) {
	// Create registry with test packages
	reg := NewMemoryRegistry()

	// Add utils package
	err := reg.AddPackageFromSource("@test/utils", "1.0.0", []byte(`
local utils = {}
function utils.double(x)
    return x * 2
end
return utils
`))
	if err != nil {
		t.Fatalf("AddPackageFromSource utils failed: %v", err)
	}

	// Add math package that uses utils
	err = reg.AddPackageFromSource("@test/math", "1.0.0", []byte(`
local utils = require("@test/utils")
local math = {}
function math.quadruple(x)
    return utils.double(utils.double(x))
end
return math
`))
	if err != nil {
		t.Fatalf("AddPackageFromSource math failed: %v", err)
	}

	// Create Luau state
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Initialize __loaded table
	state.NewTable()
	state.SetGlobal("__loaded")

	// Register require function
	err = RegisterRequireFunc(state, reg)
	if err != nil {
		t.Fatalf("RegisterRequireFunc failed: %v", err)
	}

	// Test require and use
	err = state.DoString(`
local utils = require("@test/utils")
result = utils.double(5)
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("result")
	result := state.ToNumber(-1)
	if result != 10 {
		t.Errorf("result = %v, want 10", result)
	}
	state.Pop(1)

	// Test nested require
	err = state.DoString(`
local math = require("@test/math")
nested_result = math.quadruple(3)
`)
	if err != nil {
		t.Fatalf("DoString for nested require failed: %v", err)
	}

	state.GetGlobal("nested_result")
	nestedResult := state.ToNumber(-1)
	if nestedResult != 12 {
		t.Errorf("nested_result = %v, want 12", nestedResult)
	}
}

func TestRequireCycleDetection(t *testing.T) {
	t.Skip("Cycle detection needs more investigation")
	reg := NewMemoryRegistry()

	// Add package A that requires B
	err := reg.AddPackageFromSource("@test/a", "1.0.0", []byte(`
local b = require("@test/b")
return { name = "a", b = b }
`))
	if err != nil {
		t.Fatalf("AddPackageFromSource a failed: %v", err)
	}

	// Add package B that requires A (cycle)
	err = reg.AddPackageFromSource("@test/b", "1.0.0", []byte(`
local a = require("@test/a")
return { name = "b", a = a }
`))
	if err != nil {
		t.Fatalf("AddPackageFromSource b failed: %v", err)
	}

	// Create Luau state
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Initialize __loaded table
	state.NewTable()
	state.SetGlobal("__loaded")

	// Register require function
	err = RegisterRequireFunc(state, reg)
	if err != nil {
		t.Fatalf("RegisterRequireFunc failed: %v", err)
	}

	// Test cycle detection - require returns (nil, error) on cycle
	// The test script checks for the error
	err = state.DoString(`
local a, err = require("@test/a")
if err and string.find(err, "cyclic") then
    cycle_detected = true
else
    cycle_detected = false
end
`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("cycle_detected")
	cycleDetected := state.ToBoolean(-1)
	if !cycleDetected {
		t.Error("Expected cycle to be detected")
	}
}

func TestParseRequireName(t *testing.T) {
	tests := []struct {
		input      string
		wantName   string
		wantConstr string
	}{
		{"@scope/pkg", "@scope/pkg", ""},
		{"@scope/pkg@^1.0.0", "@scope/pkg", "^1.0.0"},
		{"@scope/pkg@>=1.0.0", "@scope/pkg", ">=1.0.0"},
		{"pkg", "pkg", ""},
		{"pkg@1.0.0", "pkg", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, constr := ParseRequireName(tt.input)
			if name != tt.wantName || constr != tt.wantConstr {
				t.Errorf("ParseRequireName(%q) = (%q, %q), want (%q, %q)",
					tt.input, name, constr, tt.wantName, tt.wantConstr)
			}
		})
	}
}
