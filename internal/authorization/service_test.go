package authorization

import "testing"

func TestValidPermissionUsesResourceSpecificActions(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		action       string
		want         bool
	}{
		{name: "object read", resourceType: "object", action: ActionRead, want: true},
		{name: "object create", resourceType: "object", action: ActionCreate, want: true},
		{name: "object rejects field write", resourceType: "object", action: ActionWrite, want: false},
		{name: "field read", resourceType: "field", action: ActionRead, want: true},
		{name: "field write", resourceType: "field", action: ActionWrite, want: true},
		{name: "field rejects object update", resourceType: "field", action: ActionUpdate, want: false},
		{name: "platform keeps capability action vocabulary", resourceType: "platform", action: "manage", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validPermission(test.resourceType, "resource", test.action); got != test.want {
				t.Fatalf("validPermission(%q, %q) = %t, want %t", test.resourceType, test.action, got, test.want)
			}
		})
	}
}
