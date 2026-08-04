package principal

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

func TestServicePrincipalSyncRequiresSignedOwnerAndClientClaims(t *testing.T) {
	service := &Service{}
	actor := capability.Actor{ID: "service-developer", Scopes: []string{"identity.principal.sync"}}
	request := capability.Request{
		TenantID: "11111111-1111-4111-8111-111111111111",
		Actor:    actor,
		Principal: &capability.TrustedPrincipal{
			TenantID: "11111111-1111-4111-8111-111111111111", CompanyID: "orgaaaaaaaaaaaaaaaaa",
			PrincipalID: actor.ID, PrincipalType: "SERVICE", Actor: actor, Source: "official_oact",
		},
	}
	_, stableErr := service.Sync(context.Background(), request, SyncInput{DisplayName: "开发者"})
	if stableErr == nil || stableErr.Code != capability.CodeUnauthenticated {
		t.Fatalf("missing owner/client claims error=%#v", stableErr)
	}
}

func TestMachinePrincipalCannotUseHumanManagementCapabilities(t *testing.T) {
	service := &Service{}
	actor := capability.Actor{ID: "service-developer", Scopes: []string{"authorization.manage"}}
	request := capability.Request{Actor: actor, Principal: &capability.TrustedPrincipal{
		PrincipalID: actor.ID, PrincipalType: "SERVICE", Actor: actor,
	}}
	_, stableErr := service.List(context.Background(), request, ListInput{})
	if stableErr == nil || stableErr.Code != capability.CodeUnauthorized {
		t.Fatalf("machine management error=%#v", stableErr)
	}
	_, stableErr = service.SetOrganizationMembership(context.Background(), request, SetOrganizationMembershipInput{
		PrincipalID: "service-target", OrganizationID: "11111111-1111-4111-8111-111111111111",
		Active: true, Primary: true, ApprovalID: "approval-1",
	})
	if stableErr == nil || stableErr.Code != capability.CodeUnauthorized {
		t.Fatalf("machine organization membership error=%#v", stableErr)
	}
}

func TestSyncInputCannotSupplyIdentityClaims(t *testing.T) {
	service := &Service{}
	definition := CapabilityDefinitions(service)[0]
	request := capability.Request{Input: json.RawMessage(`{"display_name":"开发者","principal_id":"forged"}`)}
	_, stableErr := definition.Handler(context.Background(), request, nil)
	if stableErr == nil || stableErr.Code != capability.CodeValidationFailed {
		t.Fatalf("forged identity input error=%#v", stableErr)
	}
}

func TestOrganizationMembershipRequiresVerifiedApprovalAndRejectsUnknownInput(t *testing.T) {
	service := &Service{}
	actor := capability.Actor{ID: "human-manager", Scopes: []string{"authorization.manage"}}
	request := capability.Request{Actor: actor, Principal: &capability.TrustedPrincipal{
		PrincipalID: actor.ID, PrincipalType: "HUMAN", Actor: actor,
	}}
	_, stableErr := service.SetOrganizationMembership(context.Background(), request, SetOrganizationMembershipInput{
		PrincipalID: "service-target", OrganizationID: "11111111-1111-4111-8111-111111111111",
		Active: true, Primary: true, ApprovalID: "approval-1",
	})
	if stableErr == nil || stableErr.Code != capability.CodeUnauthorized {
		t.Fatalf("missing approval error=%#v", stableErr)
	}

	var membershipDefinition capability.Definition
	for _, definition := range CapabilityDefinitions(service) {
		if definition.Descriptor.ID == "identity.principal.set-organization-membership" {
			membershipDefinition = definition
			break
		}
	}
	if membershipDefinition.Descriptor.ID == "" {
		t.Fatal("organization membership capability is not registered")
	}
	request.Input = json.RawMessage(`{"principal_id":"service-target","organization_id":"11111111-1111-4111-8111-111111111111","active":true,"primary":true,"approval_id":"approval-1","tenant_id":"forged"}`)
	_, stableErr = membershipDefinition.Handler(context.Background(), request, nil)
	if stableErr == nil || stableErr.Code != capability.CodeValidationFailed {
		t.Fatalf("forged tenant input error=%#v", stableErr)
	}
}
