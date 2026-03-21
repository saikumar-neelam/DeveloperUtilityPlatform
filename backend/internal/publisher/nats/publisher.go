package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/nats-io/nats.go"
)

const subjectWebhookReceived = "webhooks.received"

// Publisher implements domain.EventPublisher using NATS.
type Publisher struct {
	nc *nats.Conn
}

func New(url string) (*Publisher, error) {
	nc, err := nats.Connect(url,
		nats.Name("webhook-platform"),
		nats.MaxReconnects(5),
		nats.ReconnectWait(nats.DefaultReconnectWait),
	)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}
	return &Publisher{nc: nc}, nil
}

func (p *Publisher) PublishWebhookReceived(_ context.Context, req *domain.WebhookRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("nats: marshal webhook event: %w", err)
	}
	if err := p.nc.Publish(subjectWebhookReceived, payload); err != nil {
		return fmt.Errorf("nats: publish webhook.received: %w", err)
	}
	return nil
}

func (p *Publisher) Close() {
	p.nc.Drain()
}

// Compile-time interface check.
var _ domain.EventPublisher = (*Publisher)(nil)
