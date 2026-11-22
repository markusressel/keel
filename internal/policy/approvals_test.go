package policy

import "testing"

func TestApprovalRequired(t *testing.T) {
	tests := []struct {
		name         string
		approvalsFor string
		current      string
		new          string
		want         bool
		wantErr      bool
	}{
		{name: "unset, patch", approvalsFor: "", current: "1.2.3", new: "1.2.4", want: true},
		{name: "unset, unparsable", approvalsFor: "", current: "latest", new: "latest", want: true},
		{name: "all, patch", approvalsFor: "all", current: "1.2.3", new: "1.2.4", want: true},

		{name: "major, major", approvalsFor: "major", current: "1.2.3", new: "2.0.0", want: true},
		{name: "major, major down", approvalsFor: "major", current: "2.0.0", new: "1.9.9", want: true},
		{name: "major, minor", approvalsFor: "major", current: "1.2.3", new: "1.3.0", want: false},
		{name: "major, patch", approvalsFor: "major", current: "1.2.3", new: "1.2.4", want: false},
		{name: "major, v prefix", approvalsFor: "major", current: "v1.2.3", new: "v1.3.0", want: false},
		{name: "major, v prefix major", approvalsFor: "major", current: "v1.2.3", new: "v2.0.0", want: true},
		{name: "major, pre-release", approvalsFor: "major", current: "1.2.3-rc1", new: "1.2.3", want: false},
		{name: "major, same tag (digest change)", approvalsFor: "major", current: "1.2.3", new: "1.2.3", want: false},
		{name: "major, two parts", approvalsFor: "major", current: "1.2", new: "1.3", want: false},

		{name: "minor, major", approvalsFor: "minor", current: "1.2.3", new: "2.0.0", want: true},
		{name: "minor, minor", approvalsFor: "minor", current: "1.2.3", new: "1.3.0", want: true},
		{name: "minor, patch", approvalsFor: "minor", current: "1.2.3", new: "1.2.4", want: false},
		{name: "minor, upper case", approvalsFor: "Minor", current: "1.2.3", new: "1.2.4", want: false},

		{name: "major, four parts revision", approvalsFor: "major", current: "21.0.0.0", new: "21.0.0.1", want: false},
		{name: "major, four parts minor", approvalsFor: "major", current: "21.0.0.0", new: "21.1.0.0", want: false},
		{name: "major, four parts major", approvalsFor: "major", current: "21.0.0.0", new: "22.0.0.0", want: true},
		{name: "minor, four parts revision", approvalsFor: "minor", current: "1.2.5.5", new: "1.2.5.6", want: false},
		{name: "minor, four parts minor", approvalsFor: "minor", current: "1.2.5.5", new: "1.3.0.0", want: true},
		{name: "minor, three to four parts", approvalsFor: "minor", current: "1.2.5", new: "1.2.5.1", want: false},

		{name: "major, latest", approvalsFor: "major", current: "latest", new: "latest", want: true},
		{name: "major, current not a version", approvalsFor: "major", current: "latest", new: "1.2.3", want: true},
		{name: "major, new not a version", approvalsFor: "major", current: "1.2.3", new: "main-abc1234", want: true},
		{name: "major, bare numbers", approvalsFor: "major", current: "41", new: "42", want: true},
		{name: "minor, build tags", approvalsFor: "minor", current: "build-100", new: "build-101", want: true},

		{name: "unknown value", approvalsFor: "patch-only", current: "1.2.3", new: "1.2.4", want: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApprovalRequired(tt.approvalsFor, tt.current, tt.new)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ApprovalRequired(%q, %q, %q) error = %v, wantErr %v", tt.approvalsFor, tt.current, tt.new, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ApprovalRequired(%q, %q, %q) = %v, want %v", tt.approvalsFor, tt.current, tt.new, got, tt.want)
			}
		})
	}
}
