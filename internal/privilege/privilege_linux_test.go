//go:build linux

package privilege

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		root, caps bool
		want       string
	}{
		{name: "root", root: true, want: "administrative"},
		{name: "capacidade relevante", caps: true, want: "privileged"},
		{name: "somente leitura", want: "read_only"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classify(test.root, test.caps); got != test.want {
				t.Fatalf("classify() = %q; esperado %q", got, test.want)
			}
		})
	}
}
