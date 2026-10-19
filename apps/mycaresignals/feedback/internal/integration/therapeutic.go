package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"qr-feedback-api/internal/models"
)

type DeliveryStore interface {
	ListPendingDeliveries(context.Context, int) ([]models.PendingDelivery, error)
	MarkDeliveryComplete(context.Context, int64) error
	MarkDeliveryFailed(context.Context, int64, string) error
}

type Dispatcher struct {
	store      DeliveryStore
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewDispatcher(store DeliveryStore, baseURL, token string) *Dispatcher {
	return &Dispatcher{
		store: store, endpoint: strings.TrimRight(baseURL, "/") + "/internal/v1/feedback-intake/receipts",
		token: strings.TrimSpace(token), httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *Dispatcher) Configured() bool {
	return d != nil && d.store != nil && strings.HasPrefix(d.endpoint, "http") && d.token != ""
}

func (d *Dispatcher) Run(ctx context.Context) {
	if !d.Configured() {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		_ = d.DispatchOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	if !d.Configured() {
		return errors.New("therapeutic intake delivery is not configured")
	}
	items, err := d.store.ListPendingDeliveries(ctx, 20)
	if err != nil {
		return err
	}
	for _, item := range items {
		if visitorFeedbackDelivery(item.Payload) {
			if err := d.store.MarkDeliveryComplete(ctx, item.ID); err != nil {
				return err
			}
			continue
		}
		code := d.deliver(ctx, item.Payload)
		if code == "" {
			if err := d.store.MarkDeliveryComplete(ctx, item.ID); err != nil {
				return err
			}
			continue
		}
		if err := d.store.MarkDeliveryFailed(ctx, item.ID, code); err != nil {
			return err
		}
	}
	return nil
}

func visitorFeedbackDelivery(payload json.RawMessage) bool {
	var envelope struct {
		IntakeType string `json:"intake_type"`
	}
	return json.Unmarshal(payload, &envelope) == nil && envelope.IntakeType == models.IntakeTypeVisitorFeedback
}

func (d *Dispatcher) deliver(ctx context.Context, payload json.RawMessage) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "request_build_failed"
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Feedback-Intake-Token", d.token)
	response, err := d.httpClient.Do(request)
	if err != nil {
		return "network_error"
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return ""
	}
	return fmt.Sprintf("http_%d", response.StatusCode)
}
