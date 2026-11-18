package version

import (
	"sort"
	"testing"
)

func TestNewSemverFourPart(t *testing.T) {
	tests := []struct {
		in         string
		wantErr    bool
		major      int64
		minor      int64
		patch      int64
		revision   int64
		prerelease string
	}{
		{in: "1.2.5.6", major: 1, minor: 2, patch: 5, revision: 6},
		{in: "v1.3.1.1", major: 1, minor: 3, patch: 1, revision: 1},
		{in: "1.3.1.1-rc1", major: 1, minor: 3, patch: 1, revision: 1, prerelease: "rc1"},
		{in: "1.3.1", major: 1, minor: 3, patch: 1},
		{in: "1.3.1-rc1", major: 1, minor: 3, patch: 1, prerelease: "rc1"},
		{in: "1.2.5.6.7", wantErr: true},
		{in: "1.2.5.x", wantErr: true},
		{in: "latest", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			v, err := NewSemver(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Major() != tt.major || v.Minor() != tt.minor || v.Patch() != tt.patch ||
				v.Revision() != tt.revision || v.Prerelease() != tt.prerelease {
				t.Errorf("got %d.%d.%d.%d-%s", v.Major(), v.Minor(), v.Patch(), v.Revision(), v.Prerelease())
			}
			if v.Original() != tt.in && tt.in != "v1.3.1.1" {
				t.Errorf("original: got %q, want %q", v.Original(), tt.in)
			}
		})
	}
}

func TestSemverOrder(t *testing.T) {
	// ascending
	order := []string{
		"1.2.5-rc1", "1.2.5", "1.2.5.1-rc1", "1.2.5.1", "1.2.5.6", "1.2.5.7", "1.2.6", "1.3.0", "1.3.1", "1.3.1.1", "1.3.2",
	}
	var vs Collection
	for i := len(order) - 1; i >= 0; i-- { // start reversed
		v, err := NewSemver(order[i])
		if err != nil {
			t.Fatal(err)
		}
		vs = append(vs, v)
	}
	sort.Sort(vs)
	for i, v := range vs {
		if v.Original() != order[i] {
			t.Errorf("position %d: got %s, want %s", i, v.Original(), order[i])
		}
	}
}

func TestNewAvailableFourPart(t *testing.T) {
	tags := []string{"1.2.5.6", "1.2.5.7", "1.3.0", "1.3.1", "1.3.1.1", "latest", "v1.3.2b1-daily.20261118"}

	got, available, err := NewAvailable("1.3.1", tags, false)
	if err != nil || !available || got != "1.3.1.1" {
		t.Errorf("1.3.1: got %q %v %v, want 1.3.1.1 true", got, available, err)
	}
	got, available, err = NewAvailable("1.2.5.6", tags, false)
	if err != nil || !available || got != "1.3.1.1" {
		t.Errorf("1.2.5.6: got %q %v %v, want 1.3.1.1 true", got, available, err)
	}
	_, available, err = NewAvailable("1.3.1.1", tags, false)
	if err != nil || available {
		t.Errorf("1.3.1.1: got available=%v err=%v, want false", available, err)
	}
}

func TestGetVersionFourPart(t *testing.T) {
	v, err := GetVersion("1.2.5.6")
	if err != nil {
		t.Fatal(err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 5 || v.Revision != 6 || v.Original != "1.2.5.6" {
		t.Errorf("got %+v", v)
	}
}
