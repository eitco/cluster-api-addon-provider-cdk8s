package resourcer

import (
	"testing"
)

func TestGetPluralFromKind(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{"Pod", "pods"},
		{"Service", "services"},
		{"Ingress", "ingresses"},
		{"Policy", "policies"},
		{"Class", "classes"},
		{"Node", "nodes"},
		{"Secret", "secrets"},
		{"ConfigMap", "configmaps"},
		{"Deployment", "deployments"},
		{"Repository", "repositories"},
		{"", "s"},
		{"Y", "ies"},
		{"S", "ses"},
		{"s", "ses"},
		{"policy", "policies"},
		{"class", "classes"},
	}

	for _, tt := range tests {
		got := getPluralFromKind(tt.kind)
		if got != tt.expected {
			t.Errorf("getPluralFromKind(%q) = %q; want %q", tt.kind, got, tt.expected)
		}
	}
}
