package version

type Version struct {
	Major int
	Minor int
	Patch int
}

// Compare compares two Version structs and returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func Compare(v1, v2 Version) int {
	switch {
	case v1.Major != v2.Major:
		return compareInt(v1.Major, v2.Major)
	case v1.Minor != v2.Minor:
		return compareInt(v1.Minor, v2.Minor)
	case v1.Patch != v2.Patch:
		return compareInt(v1.Patch, v2.Patch)
	}
	return 0
}

// compareInt compares two integers and returns -1, 0, or 1.
func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
