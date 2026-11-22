package policy

import (
	"reflect"
	"testing"
)

func TestCompareTags(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.12", "1.0.9", 1},
		{"1.0.9", "1.0.12", -1},
		{"15.0.0", "9.1.0", 1},
		{"1.2.3", "1.2.3", 0},
		{"1.2.3.4", "1.2.3.10", -1},
		{"1.2.3", "latest", 1},
		{"latest", "1.2.3", -1},
		{"build-b", "build-a", 1},
		// a bare number is not a version, so it compares as a string
		{"9", "10", 1},
	}
	for _, tt := range tests {
		if got := compareTags(tt.a, tt.b); got != tt.want {
			t.Errorf("compareTags(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestGlobPolicy_VersionOrder(t *testing.T) {
	p, err := NewGlobPolicy("glob:1.0.*")
	if err != nil {
		t.Fatal(err)
	}
	got := p.Filter([]string{"1.0.9", "1.0.12", "1.0.10", "2.0.0", "latest"})
	want := []string{"1.0.12", "1.0.10", "1.0.9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter() = %v, want %v", got, want)
	}

	update, _ := p.ShouldUpdate("1.0.9", "1.0.12")
	if !update {
		t.Error("ShouldUpdate(1.0.9, 1.0.12) = false, want true")
	}
	update, _ = p.ShouldUpdate("1.0.12", "1.0.9")
	if update {
		t.Error("ShouldUpdate(1.0.12, 1.0.9) = true, want false")
	}
}

func TestIsNewerTag(t *testing.T) {
	tests := []struct {
		candidate, current string
		want               bool
	}{
		{"1.0.12", "1.0.9", true},
		{"1.0.9", "1.0.12", false},
		{"1.0.9", "1.0.9", false},
		// a version tag can move to a build tag and back, as before: plain string order
		{"build-1696801785", "1.0.0", true},
		{"build-2", "build-1", true},
	}
	for _, tt := range tests {
		if got := isNewerTag(tt.candidate, tt.current); got != tt.want {
			t.Errorf("isNewerTag(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
		}
	}
}

// Forgejo's long-term support releases are every fourth major version (7, 11, 15, 19, ...): the major version
// modulo 4 is 3.
const forgejoLTSPattern = `regexp:^([37]|[0-9]*([02468][37]|[13579][159]))\.[0-9]+\.[0-9]+$`

func TestRegexpPolicy_LongTermSupportReleases(t *testing.T) {
	p, err := NewRegexpPolicy(forgejoLTSPattern)
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{"13", "13.0.5", "14.0.5", "15", "15.0", "15.0.9", "15.0.12", "15.0.10", "16.0.7", "17.0.2", "11.0.3", "7.0.16", "latest"}

	got := p.Filter(tags)
	want := []string{"15.0.12", "15.0.10", "15.0.9", "11.0.3", "7.0.16"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter() = %v, want %v", got, want)
	}

	got = p.Filter(append(tags, "19.0.0", "19.0.1", "23.0.0", "27.1.0", "31.0.0", "18.0.0", "21.0.0", "22.0.0", "25.0.0"))
	want = []string{"31.0.0", "27.1.0", "23.0.0", "19.0.1", "19.0.0", "15.0.12", "15.0.10", "15.0.9", "11.0.3", "7.0.16"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter() with later releases = %v, want %v", got, want)
	}
}

func TestRegexpPolicy_CompareGroupVersionOrder(t *testing.T) {
	p, err := NewRegexpPolicy(`regexp:^app-(?P<compare>[0-9.]+)$`)
	if err != nil {
		t.Fatal(err)
	}
	got := p.Filter([]string{"app-1.0.9", "app-1.0.12", "app-1.0.10"})
	want := []string{"app-1.0.12", "app-1.0.10", "app-1.0.9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter() = %v, want %v", got, want)
	}
}
