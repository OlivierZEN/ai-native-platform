package provisioning

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxClockSkew = 5 * time.Minute

type replayGuard struct {
	mu     sync.Mutex
	values map[string]time.Time
}

func newReplayGuard() *replayGuard { return &replayGuard{values: map[string]time.Time{}} }

func (g *replayGuard) consume(serviceID, nonce string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, expiry := range g.values {
		if !expiry.After(now) {
			delete(g.values, key)
		}
	}
	key := serviceID + ":" + nonce
	if _, exists := g.values[key]; exists {
		return false
	}
	g.values[key] = now.Add(maxClockSkew)
	return true
}

func verifySignature(key, serviceID, method, path, timestamp, nonce, signature string, body []byte, guard *replayGuard, now time.Time) error {
	if len(key) < 32 || serviceID == "" || len(nonce) < 16 || signature == "" {
		return fmt.Errorf("invalid internal authentication")
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || now.Sub(time.Unix(seconds, 0)).Abs() > maxClockSkew {
		return fmt.Errorf("invalid internal authentication")
	}
	expected := sign(key, serviceID, method, path, timestamp, nonce, body)
	provided, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(expected, provided) {
		return fmt.Errorf("invalid internal authentication")
	}
	if !guard.consume(serviceID, nonce, now) {
		return fmt.Errorf("invalid internal authentication")
	}
	return nil
}

func sign(key, serviceID, method, path, timestamp, nonce string, body []byte) []byte {
	bodyHash := sha256.Sum256(body)
	payload := strings.Join([]string{serviceID, method, path, timestamp, nonce, hex.EncodeToString(bodyHash[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
