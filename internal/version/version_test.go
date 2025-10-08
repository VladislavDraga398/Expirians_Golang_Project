package version

import (
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	v, c, d := Info()
	switch {
	case v == "":
		t.Error("version should not be empty")
	case c == "":
		t.Error("commit should not be empty")
	case d == "":
		t.Error("date should not be empty")
	default:
		t.Log("version: ", v)
		t.Log("commit: ", c)
		t.Log("date: ", d)
	}
}

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Error("GetVersion should not return empty string")
	}
}

func TestGetCommit(t *testing.T) {
	c := GetCommit()
	if c == "" {
		t.Error("GetCommit should not return empty string")
	}
}

func TestGetDate(t *testing.T) {
	d := GetDate()
	if d == "" {
		t.Error("GetDate should not return empty string")
	}
}

func TestString(t *testing.T) {
	s := String()
	switch {
	case s == "":
		t.Error("String should not return empty string")
	default:
		t.Log("string: ", s)
	}

	// Should contain version, commit, and date
	switch {
	case !strings.Contains(s, "version="):
		t.Error("String should contain 'version='")
	case !strings.Contains(s, "commit="):
		t.Error("String should contain 'commit='")
	case !strings.Contains(s, "date="):
		t.Error("String should contain 'date='")
	default:
		t.Log("string: ", s)
	}
}

func TestVersionConsistency(t *testing.T) {
	// GetVersion should match Info
	v1 := GetVersion()
	v2, _, _ := Info()

	switch {
	case v1 != v2:
		t.Errorf("GetVersion (%s) should match Info version (%s)", v1, v2)
	default:
		t.Log("version: ", v1)
	}

	// GetCommit should match Info
	c1 := GetCommit()
	_, c2, _ := Info()

	switch {
	case c1 != c2:
		t.Errorf("GetCommit (%s) should match Info commit (%s)", c1, c2)
	default:
		t.Log("commit: ", c1)
	}

	// GetDate should match Info
	d1 := GetDate()
	_, _, d2 := Info()

	switch {
	case d1 != d2:
		t.Errorf("GetDate (%s) should match Info date (%s)", d1, d2)
	default:
		t.Log("date: ", d1)
	}
}
