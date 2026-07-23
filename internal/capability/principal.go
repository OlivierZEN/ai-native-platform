package capability

func BindTrustedPrincipal(request Request, principal TrustedPrincipal) (Request, *StableError) {
	if principal.TenantID == "" || principal.CompanyID == "" || principal.Actor.ID == "" || principal.Source == "" {
		return Request{}, &StableError{Code: CodeUnauthenticated, Message: "verified identity is incomplete"}
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
