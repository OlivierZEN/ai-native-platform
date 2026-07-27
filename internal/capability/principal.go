package capability

func BindTrustedPrincipal(request Request, principal TrustedPrincipal) (Request, *StableError) {
	// Internal trusted call sites predate the external Principal claim. They are
	// still authenticated server-side and retain their actor as a HUMAN compatibility
	// principal; externally supplied tokens are normalized by identity.Verifier.
	if principal.PrincipalID == "" {
		principal.PrincipalID = principal.Actor.ID
	}
	if principal.PrincipalType == "" {
		principal.PrincipalType = "HUMAN"
	}
	if principal.TenantID == "" || principal.CompanyID == "" || principal.PrincipalID == "" || principal.PrincipalType == "" || principal.Actor.ID == "" || principal.Source == "" {
		return Request{}, &StableError{Code: CodeUnauthenticated, Message: "verified identity is incomplete"}
	}
	if principal.PrincipalID != principal.Actor.ID {
		return Request{}, &StableError{Code: CodeUnauthenticated, Message: "verified principal does not match actor"}
	}
	if request.TenantID != "" && request.TenantID != principal.TenantID {
		return Request{}, &StableError{Code: CodeValidationFailed, Message: "tenant_id does not match the verified identity"}
	}
	if request.Actor.ID != "" && request.Actor.ID != principal.Actor.ID {
		return Request{}, &StableError{Code: CodeValidationFailed, Message: "actor.id does not match the verified identity"}
	}
	request.TenantID = principal.TenantID
	request.Actor = principal.Actor
	request.Principal = &principal
	return request, nil
}
