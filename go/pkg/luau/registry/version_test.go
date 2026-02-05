package registry

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    Version
		wantErr bool
	}{
		{"1.0.0", Version{Major: 1, Minor: 0, Patch: 0}, false},
		{"v1.2.3", Version{Major: 1, Minor: 2, Patch: 3}, false},
		{"0.1.0", Version{Major: 0, Minor: 1, Patch: 0}, false},
		{"1.0.0-alpha", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"}, false},
		{"1.0.0-beta.1", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta.1"}, false},
		{"1.0.0+build", Version{Major: 1, Minor: 0, Patch: 0, Build: "build"}, false},
		{"1.0.0-rc.1+build.123", Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "rc.1", Build: "build.123"}, false},
		{"1.2", Version{Major: 1, Minor: 2, Patch: 0}, false},
		{"1", Version{Major: 1, Minor: 0, Patch: 0}, false},
		{"invalid", Version{}, true},
		{"", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha", "1.0.0-alpha", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			v1 := MustParseVersion(tt.v1)
			v2 := MustParseVersion(tt.v2)
			got := v1.Compare(v2)
			if got != tt.want {
				t.Errorf("%v.Compare(%v) = %v, want %v", v1, v2, got, tt.want)
			}
		})
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		input   string
		version string
		match   bool
	}{
		// Exact match
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"=1.0.0", "1.0.0", true},

		// Greater than
		{">1.0.0", "1.0.1", true},
		{">1.0.0", "1.0.0", false},
		{">1.0.0", "0.9.9", false},

		// Greater than or equal
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "1.0.1", true},
		{">=1.0.0", "0.9.9", false},

		// Less than
		{"<2.0.0", "1.9.9", true},
		{"<2.0.0", "2.0.0", false},
		{"<2.0.0", "2.0.1", false},

		// Less than or equal
		{"<=2.0.0", "2.0.0", true},
		{"<=2.0.0", "1.9.9", true},
		{"<=2.0.0", "2.0.1", false},

		// Caret (compatible with)
		{"^1.2.3", "1.2.3", true},
		{"^1.2.3", "1.9.9", true},
		{"^1.2.3", "2.0.0", false},
		{"^1.2.3", "1.2.2", false},
		{"^0.2.3", "0.2.5", true},
		{"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.3", true},
		{"^0.0.3", "0.0.4", false},

		// Tilde (approximately)
		{"~1.2.3", "1.2.3", true},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"~1.2.3", "1.2.2", false},

		// Range
		{">=1.0.0 <2.0.0", "1.5.0", true},
		{">=1.0.0 <2.0.0", "1.0.0", true},
		{">=1.0.0 <2.0.0", "2.0.0", false},
		{">=1.0.0 <2.0.0", "0.9.9", false},

		// Wildcard/latest
		{"", "1.0.0", true},
		{"latest", "1.0.0", true},
		{"*", "9.9.9", true},
	}

	for _, tt := range tests {
		t.Run(tt.input+" matches "+tt.version, func(t *testing.T) {
			c, err := ParseConstraint(tt.input)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.input, err)
			}
			v := MustParseVersion(tt.version)
			got := c.Match(v)
			if got != tt.match {
				t.Errorf("Constraint(%q).Match(%v) = %v, want %v", tt.input, v, got, tt.match)
			}
		})
	}
}

func TestFindBestMatch(t *testing.T) {
	versions := []Version{
		MustParseVersion("1.0.0"),
		MustParseVersion("1.1.0"),
		MustParseVersion("1.2.0"),
		MustParseVersion("2.0.0"),
		MustParseVersion("2.1.0"),
	}

	tests := []struct {
		constraint string
		want       string
	}{
		{"^1.0.0", "1.2.0"},
		{"~1.1.0", "1.1.0"},
		{">=2.0.0", "2.1.0"},
		{"<2.0.0", "1.2.0"},
		{"", "2.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			c, _ := ParseConstraint(tt.constraint)
			got := FindBestMatch(versions, c)
			if got == nil {
				t.Fatalf("FindBestMatch returned nil")
			}
			if got.String() != tt.want {
				t.Errorf("FindBestMatch(%q) = %v, want %v", tt.constraint, got, tt.want)
			}
		})
	}
}
