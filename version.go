package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a semantic version without build metadata.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

var semverRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ParseVersion parses "1.2.3" or "1.2.3-rc.1". No prefix allowed.
func ParseVersion(s string) (Version, bool) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat, _ := strconv.Atoi(m[3])
	return Version{maj, min, pat, m[4]}, true
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

func (v Version) core() Version { return Version{v.Major, v.Minor, v.Patch, ""} }

// Compare returns -1, 0, 1 following semver precedence.
func (v Version) Compare(o Version) int {
	for _, p := range [][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if p[0] != p[1] {
			if p[0] < p[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.Pre == o.Pre:
		return 0
	case v.Pre == "":
		return 1
	case o.Pre == "":
		return -1
	}
	a, b := strings.Split(v.Pre, "."), strings.Split(o.Pre, ".")
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func compareIdent(a, b string) int {
	an, aerr := strconv.Atoi(a)
	bn, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		if an < bn {
			return -1
		} else if an > bn {
			return 1
		}
		return 0
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// Tag is a semver-shaped git tag.
type Tag struct {
	Name   string
	Prefix string // "" or "v"
	V      Version
}

// ParseTag recognises "1.2.3" and "v1.2.3".
func ParseTag(name string) (Tag, bool) {
	prefix := ""
	rest := name
	if strings.HasPrefix(name, "v") {
		prefix, rest = "v", name[1:]
	}
	v, ok := ParseVersion(rest)
	if !ok {
		return Tag{}, false
	}
	return Tag{name, prefix, v}, true
}

// LatestTag returns the tag with the highest precedence.
func LatestTag(tags []Tag) (Tag, bool) {
	if len(tags) == 0 {
		return Tag{}, false
	}
	best := tags[0]
	for _, t := range tags[1:] {
		if t.V.Compare(best.V) > 0 {
			best = t
		}
	}
	return best, true
}

type prefixState int

const (
	prefixNone  prefixState = iota // no semver tags
	prefixClear                    // all tags share one style
	prefixMixed
)

// DetectPrefix inspects existing tags for their prefix style.
func DetectPrefix(tags []Tag) (string, prefixState) {
	if len(tags) == 0 {
		return "", prefixNone
	}
	for _, t := range tags[1:] {
		if t.Prefix != tags[0].Prefix {
			return "", prefixMixed
		}
	}
	return tags[0].Prefix, prefixClear
}

// NextVersion computes the version for kind (major|minor|patch), optionally as a pre-release.
func NextVersion(latest *Version, kind, pre string, tags []Tag) (Version, error) {
	var next Version
	if latest == nil {
		switch kind {
		case "major":
			next = Version{Major: 1}
		case "minor":
			next = Version{Minor: 1}
		case "patch":
			next = Version{Patch: 1}
		default:
			return Version{}, fmt.Errorf("unknown bump %q", kind)
		}
	} else {
		l := *latest
		switch kind {
		case "major":
			next = Version{Major: l.Major + 1}
		case "minor":
			next = Version{Major: l.Major, Minor: l.Minor + 1}
		case "patch":
			if l.Pre != "" {
				next = l.core()
			} else {
				next = Version{Major: l.Major, Minor: l.Minor, Patch: l.Patch + 1}
			}
		default:
			return Version{}, fmt.Errorf("unknown bump %q", kind)
		}
	}
	if pre != "" {
		n := 1
		re := regexp.MustCompile(`^` + regexp.QuoteMeta(pre) + `\.(\d+)$`)
		for _, t := range tags {
			if t.V.core() != next {
				continue
			}
			if m := re.FindStringSubmatch(t.V.Pre); m != nil {
				if k, _ := strconv.Atoi(m[1]); k >= n {
					n = k + 1
				}
			}
		}
		next.Pre = fmt.Sprintf("%s.%d", pre, n)
	}
	return next, nil
}
