package registry

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// String returns the string representation of the version.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Prerelease versions have lower precedence
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	return 0
}

// LessThan returns true if v < other.
func (v Version) LessThan(other Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other.
func (v Version) GreaterThan(other Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other.
func (v Version) Equal(other Version) bool {
	return v.Compare(other) == 0
}

// versionRegex matches semantic versions.
var versionRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`)

// ParseVersion parses a version string.
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	matches := versionRegex.FindStringSubmatch(s)
	if matches == nil {
		return Version{}, fmt.Errorf("invalid version: %s", s)
	}

	var v Version
	v.Major, _ = strconv.Atoi(matches[1])
	if matches[2] != "" {
		v.Minor, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		v.Patch, _ = strconv.Atoi(matches[3])
	}
	v.Prerelease = matches[4]
	v.Build = matches[5]

	return v, nil
}

// MustParseVersion parses a version string, panicking on error.
func MustParseVersion(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Constraint represents a version constraint.
type Constraint struct {
	op      constraintOp
	version Version

	// For range constraints (>= x < y)
	minOp  constraintOp
	minVer Version
	maxOp  constraintOp
	maxVer Version
}

type constraintOp int

const (
	opNone constraintOp = iota
	opEQ                // =
	opNE                // !=
	opGT                // >
	opGE                // >=
	opLT                // <
	opLE                // <=
	opCaret             // ^
	opTilde             // ~
	opRange             // >= x < y
)

// Match checks if a version satisfies the constraint.
func (c *Constraint) Match(v Version) bool {
	switch c.op {
	case opNone, opEQ:
		return v.Major == c.version.Major &&
			v.Minor == c.version.Minor &&
			v.Patch == c.version.Patch &&
			(c.version.Prerelease == "" || v.Prerelease == c.version.Prerelease)

	case opNE:
		return !c.Match(v)

	case opGT:
		return v.GreaterThan(c.version)

	case opGE:
		return v.GreaterThan(c.version) || v.Equal(c.version)

	case opLT:
		return v.LessThan(c.version)

	case opLE:
		return v.LessThan(c.version) || v.Equal(c.version)

	case opCaret:
		// ^1.2.3 means >=1.2.3 <2.0.0 (for major > 0)
		// ^0.2.3 means >=0.2.3 <0.3.0 (for major == 0)
		// ^0.0.3 means >=0.0.3 <0.0.4 (for major == 0 && minor == 0)
		if v.LessThan(c.version) {
			return false
		}
		if c.version.Major > 0 {
			return v.Major == c.version.Major
		}
		if c.version.Minor > 0 {
			return v.Major == 0 && v.Minor == c.version.Minor
		}
		return v.Major == 0 && v.Minor == 0 && v.Patch == c.version.Patch

	case opTilde:
		// ~1.2.3 means >=1.2.3 <1.3.0
		if v.LessThan(c.version) {
			return false
		}
		return v.Major == c.version.Major && v.Minor == c.version.Minor

	case opRange:
		// Check min constraint
		minOK := false
		switch c.minOp {
		case opGE:
			minOK = v.GreaterThan(c.minVer) || v.Equal(c.minVer)
		case opGT:
			minOK = v.GreaterThan(c.minVer)
		default:
			minOK = true
		}

		if !minOK {
			return false
		}

		// Check max constraint
		switch c.maxOp {
		case opLE:
			return v.LessThan(c.maxVer) || v.Equal(c.maxVer)
		case opLT:
			return v.LessThan(c.maxVer)
		default:
			return true
		}

	default:
		return false
	}
}

// String returns the string representation of the constraint.
func (c *Constraint) String() string {
	switch c.op {
	case opNone, opEQ:
		return c.version.String()
	case opNE:
		return "!=" + c.version.String()
	case opGT:
		return ">" + c.version.String()
	case opGE:
		return ">=" + c.version.String()
	case opLT:
		return "<" + c.version.String()
	case opLE:
		return "<=" + c.version.String()
	case opCaret:
		return "^" + c.version.String()
	case opTilde:
		return "~" + c.version.String()
	case opRange:
		return fmt.Sprintf(">=%s <%s", c.minVer.String(), c.maxVer.String())
	default:
		return c.version.String()
	}
}

// constraintRegex matches version constraints.
var constraintRegex = regexp.MustCompile(`^([\^~]|>=?|<=?|!=|=)?(.+)$`)

// ParseConstraint parses a version constraint string.
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "latest" || s == "*" {
		return &Constraint{op: opGE, version: Version{}}, nil
	}

	// Check for range constraint: ">=1.0.0 <2.0.0"
	if strings.Contains(s, " ") {
		parts := strings.Fields(s)
		if len(parts) == 2 {
			c1, err := ParseConstraint(parts[0])
			if err != nil {
				return nil, err
			}
			c2, err := ParseConstraint(parts[1])
			if err != nil {
				return nil, err
			}

			if (c1.op == opGE || c1.op == opGT) && (c2.op == opLT || c2.op == opLE) {
				return &Constraint{
					op:     opRange,
					minOp:  c1.op,
					minVer: c1.version,
					maxOp:  c2.op,
					maxVer: c2.version,
				}, nil
			}
		}
		return nil, fmt.Errorf("invalid range constraint: %s", s)
	}

	matches := constraintRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid constraint: %s", s)
	}

	op := matches[1]
	verStr := matches[2]

	v, err := ParseVersion(verStr)
	if err != nil {
		return nil, err
	}

	c := &Constraint{version: v}
	switch op {
	case "", "=":
		c.op = opEQ
	case "!=":
		c.op = opNE
	case ">":
		c.op = opGT
	case ">=":
		c.op = opGE
	case "<":
		c.op = opLT
	case "<=":
		c.op = opLE
	case "^":
		c.op = opCaret
	case "~":
		c.op = opTilde
	default:
		return nil, fmt.Errorf("unknown operator: %s", op)
	}

	return c, nil
}

// FindBestMatch finds the best version matching a constraint from a list.
// Returns nil if no matching version is found.
func FindBestMatch(versions []Version, constraint *Constraint) *Version {
	if constraint == nil {
		return nil
	}

	var matches []Version
	for _, v := range versions {
		if constraint.Match(v) {
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort descending and return the highest
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].GreaterThan(matches[j])
	})

	return &matches[0]
}

// SortVersions sorts versions in descending order (newest first).
func SortVersions(versions []Version) {
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].GreaterThan(versions[j])
	})
}

// SortVersionsAsc sorts versions in ascending order (oldest first).
func SortVersionsAsc(versions []Version) {
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].LessThan(versions[j])
	})
}
