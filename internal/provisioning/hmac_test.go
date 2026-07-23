package provisioning

import (
	"encoding/hex"
	"testing"
	"time"
)

const testInternalKey = "test-only-internal-key-material-that-is-long-enough"

func TestVerifySignatureAcceptsOneFreshRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	body := []byte(`{"company_id":"org2sva14i4udjmi2t4s"}`)
	timestamp := "1700000000"
	nonce := "0123456789abcdef0123456789abcdef"
	signature := hex.EncodeToString(sign(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, nonce, body))

	if err := verifySignature(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, nonce, signature, body, newReplayGuard(), now); err != nil {
		t.Fatalf("verifySignature: %v", err)
	}
}

func TestVerifySignatureRejectsReplayTamperAndExpiredRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	body := []byte(`{"company_id":"org2sva14i4udjmi2t4s"}`)
	timestamp := "1700000000"
	nonce := "0123456789abcdef0123456789abcdef"
	signature := hex.EncodeToString(sign(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, nonce, body))
	guard := newReplayGuard()
	if err := verifySignature(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, nonce, signature, body, guard, now); err != nil {
		t.Fatalf("initial request: %v", err)
	}
	if err := verifySignature(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, nonce, signature, body, guard, now); err == nil {
		t.Fatal("replayed request accepted")
	}
	if err := verifySignature(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", timestamp, "fedcba9876543210fedcba9876543210", signature, []byte(`{"company_id":"changed"}`), newReplayGuard(), now); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := verifySignature(testInternalKey, "agentcici", "POST", "/internal/v1/company-provisionings", "1699999699", "fedcba9876543210fedcba9876543210", signature, body, newReplayGuard(), now); err == nil {
		t.Fatal("expired request accepted")
	}
}
