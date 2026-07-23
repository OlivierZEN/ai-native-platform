package provisioning

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
)

type agentCiCiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

type reservation struct {
	ReservationID string `json:"reservationId"`
	CompanyID     string `json:"companyId"`
	State         string `json:"state"`
}

func newAgentCiCiClient(cfg config.Provisioning) *agentCiCiClient {
	return &agentCiCiClient{baseURL: cfg.AgentCiCiBaseURL, key: cfg.AgentCiCiHMACKey, http: &http.Client{Timeout: 8 * time.Second}}
}

func (c *agentCiCiClient) reserve(ctx context.Context, companyID, idempotencyKey string) (reservation, error) {
	var result reservation
	err := c.call(ctx, http.MethodPost, "/internal/semattice/provisioning/reservations", map[string]string{"companyId": companyID, "idempotencyKey": idempotencyKey}, &result)
	return result, err
}

func (c *agentCiCiClient) complete(ctx context.Context, reservationID string, payload map[string]any) error {
	return c.call(ctx, http.MethodPost, "/internal/semattice/provisioning/reservations/"+reservationID+"/complete", payload, nil)
}

func (c *agentCiCiClient) call(ctx context.Context, method, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode AgentCiCi request")
	}
	now := time.Now().UTC()
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("generate internal request nonce")
	}
	nonce := hex.EncodeToString(nonceBytes)
	timestamp := fmt.Sprintf("%d", now.Unix())
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create AgentCiCi request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "semattice")
	req.Header.Set("X-Internal-Timestamp", timestamp)
	req.Header.Set("X-Internal-Nonce", nonce)
	req.Header.Set("X-Internal-Signature", hex.EncodeToString(sign(c.key, "semattice", method, path, timestamp, nonce, body)))
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("AgentCiCi unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("AgentCiCi rejected provisioning")
	}
	if target == nil {
		return nil
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || !envelope.Success || json.Unmarshal(envelope.Data, target) != nil {
		return fmt.Errorf("invalid AgentCiCi provisioning response")
	}
	return nil
}
