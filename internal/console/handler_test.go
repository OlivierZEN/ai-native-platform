package console

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
)

type verifierStub struct {
	principal capability.TrustedPrincipal
	expiry    time.Time
	err       error
}

func (stub verifierStub) VerifyWithExpiration(context.Context, string) (capability.TrustedPrincipal, time.Time, error) {
	return stub.principal, stub.expiry, stub.err
}

func TestConsoleSessionProtectsFixtures(t *testing.T) {
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(verifierStub{principal: capability.TrustedPrincipal{TenantID: "93ff0c87-a626-529e-b8cf-195825df2488", CompanyID: "org2sva14i4udjmi2t4s", Actor: capability.Actor{ID: "admin-demo", Scopes: []string{"authorization.manage"}}}, expiry: now.Add(time.Hour)}, "console-session-key-material-that-is-long-enough")
	handler.now = func() time.Time { return now }
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/console/api/objects", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", unauthorized.Code)
	}
	anonymousSession := httptest.NewRecorder()
	handler.ServeHTTP(anonymousSession, httptest.NewRequest(http.MethodGet, "/console/session", nil))
	if anonymousSession.Code != http.StatusOK || !contains(anonymousSession.Body.String(), `"authenticated":false`) {
		t.Fatalf("anonymous session status=%d body=%s", anonymousSession.Code, anonymousSession.Body.String())
	}
	exchange := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/console/session", nil)
	request.Header.Set("Authorization", "Bearer validated-oact")
	handler.ServeHTTP(exchange, request)
	if exchange.Code != http.StatusCreated {
		t.Fatalf("exchange status=%d body=%s", exchange.Code, exchange.Body.String())
	}
	cookie := exchange.Result().Cookies()[0]
	objects := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/console/api/objects", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(objects, request)
	if objects.Code != http.StatusOK || !contains(objects.Body.String(), "客户") {
		t.Fatalf("objects status=%d body=%s", objects.Code, objects.Body.String())
	}
}

func TestConsoleSessionRejectsMissingManagementScope(t *testing.T) {
	handler := NewHandler(verifierStub{principal: capability.TrustedPrincipal{TenantID: "93ff0c87-a626-529e-b8cf-195825df2488", CompanyID: "org2sva14i4udjmi2t4s", Actor: capability.Actor{ID: "reader", Scopes: []string{"record.read"}}}, expiry: time.Now().Add(time.Hour)}, "console-session-key-material-that-is-long-enough")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/console/session", nil)
	request.Header.Set("Authorization", "Bearer insufficient-oact")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("scope status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestFixtureMatchesConsoleSummary(t *testing.T) {
	session := session{CompanyID: "org2sva14i4udjmi2t4s"}
	overview := fixture("/console/api/overview", session).(map[string]any)
	summary := overview["summary"].(map[string]int)
	objects := fixture("/console/api/objects", session).(map[string]any)["objects"].([]map[string]any)
	fieldCount := 0
	for _, object := range objects {
		fieldCount += len(object["fields"].([]map[string]string))
	}
	members := fixture("/console/api/members", session).(map[string]any)["members"].([]map[string]any)
	events := fixture("/console/api/audit-events", session).(map[string]any)["events"].([]map[string]string)
	settings := fixture("/console/api/system-settings", session).(map[string]any)["settings"].([]map[string]string)
	if len(objects) != summary["objects"] || fieldCount != summary["fields"] || len(members) != summary["members"] || len(events) != summary["audit_events"] || len(settings) != 10 {
		t.Fatalf("fixture summary mismatch: objects=%d fields=%d members=%d events=%d settings=%d", len(objects), fieldCount, len(members), len(events), len(settings))
	}
}

func contains(value, fragment string) bool {
	return len(value) >= len(fragment) && (value == fragment || index(value, fragment) >= 0)
}
func index(value, fragment string) int {
	for start := 0; start+len(fragment) <= len(value); start++ {
		if value[start:start+len(fragment)] == fragment {
			return start
		}
	}
	return -1
}
