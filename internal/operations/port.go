package operations

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

const ContractVersion = "native-tenant-operations.v2"

// Company identifiers retain the established 20-character value format during
// the terminology migration. The field name, rather than existing identity
// values, changes from org_id to company_id.
var companyIDPattern = regexp.MustCompile("^org[a-z0-9]{17}$")

type GlobalTenant struct {
	TenantID        uuid.UUID
	CompanyID       string
	TenantRevision  int64
	ProductRevision int64
}

type Port interface {
	ContractVersion() string
	VerifyGlobalTenant(context.Context, GlobalTenant) error
}

// ClaimBoundPort is the local v2 adapter used when a verified identity has
// already bound tenant_id and company_id to the invocation. A future remote
// operations adapter implements the same port without changing domain logic.
type ClaimBoundPort struct{}

func (ClaimBoundPort) ContractVersion() string {
	return ContractVersion
}

func (ClaimBoundPort) VerifyGlobalTenant(_ context.Context, tenant GlobalTenant) error {
	if tenant.TenantID == uuid.Nil || !companyIDPattern.MatchString(tenant.CompanyID) {
		return fmt.Errorf("global tenant identity is invalid")
	}
	if tenant.TenantRevision < 1 || tenant.ProductRevision < 1 {
		return fmt.Errorf("global tenant revisions must be positive")
	}
	return nil
}
