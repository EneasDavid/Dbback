package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const turnstileMaxResponseBytes = 1 << 20

var (
	turnstileSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	turnstileHTTPClient    = &http.Client{Timeout: 6 * time.Second}
)

type turnstileVerifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
	RemoteIP string `json:"remoteip,omitempty"`
}

type turnstileVerifyResponse struct {
	Success bool `json:"success"`
}

func ValidateTurnstile(ctx context.Context, secret string, token string, remoteIP string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return NewHTTPError(http.StatusBadRequest, "confirme que voce nao e um robo")
	}

	payload, err := json.Marshal(turnstileVerifyRequest{
		Secret:   secret,
		Response: token,
		RemoteIP: strings.TrimSpace(remoteIP),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteverifyURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := turnstileHTTPClient.Do(req)
	if err != nil {
		return NewHTTPError(http.StatusServiceUnavailable, "nao foi possivel validar a verificacao anti-robo")
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return NewHTTPError(http.StatusServiceUnavailable, "nao foi possivel validar a verificacao anti-robo")
	}

	var result turnstileVerifyResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, turnstileMaxResponseBytes))
	if err := decoder.Decode(&result); err != nil {
		return NewHTTPError(http.StatusServiceUnavailable, "nao foi possivel validar a verificacao anti-robo")
	}
	if !result.Success {
		return NewHTTPError(http.StatusForbidden, "falha na verificacao anti-robo")
	}
	return nil
}
