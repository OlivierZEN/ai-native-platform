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

func TestValidRevocationTargetAcceptsLegacyBoundedActions(t *testing.T) {
	if !validRevocationTarget("field", "legacy-field", ActionUpdate) {
		t.Fatal("legacy field/update permission must remain revocable")
	}
	if validRevocationTarget("unknown", "legacy-field", ActionUpdate) {
		t.Fatal("unknown resource type must not be revocable")
	}
	if validRevocationTarget("field", "", ActionUpdate) {
		t.Fatal("empty resource reference must not be revocable")
	}
}
