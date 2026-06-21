// Package discord は Discord Webhook へのメッセージ送信を行う。
// Webhook URL はログに漏洩しないよう、エラーメッセージから除外する。
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Message は Discord Webhook のペイロード。
type Message struct {
	Content string `json:"content"`
}

// Send は content を Discord Webhook に送信する。
func Send(ctx context.Context, webhookURL, content string) error {
	if webhookURL == "" {
		return errors.New("discord webhook URL is empty")
	}
	body, err := json.Marshal(Message{Content: content})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return maskURL(webhookURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return maskURL(webhookURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// maskURL はエラー文言に webhook URL が含まれないよう取り除く。
func maskURL(url string, err error) error {
	msg := err.Error()
	msg = strings.ReplaceAll(msg, url, "***")
	return errors.New("discord send failed: " + msg)
}
