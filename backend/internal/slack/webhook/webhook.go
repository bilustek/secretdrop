package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/slack"
)

const defaultTimeout = 5 * time.Second

// compile-time interface check.
var _ slack.Notifier = (*Notifier)(nil)

// ErrEmptyURL is returned when the webhook URL is empty.
var ErrEmptyURL = errors.New("webhook URL must not be empty")

// Notifier sends Slack notifications via an incoming webhook with Block Kit formatting.
type Notifier struct {
	webhookURL string
	httpClient *http.Client
}

// Option configures a Notifier value.
type Option func(*Notifier)

// WithHTTPClient sets a custom HTTP client for the notifier.
func WithHTTPClient(c *http.Client) Option {
	return func(n *Notifier) {
		n.httpClient = c
	}
}

// New creates a new webhook slack notifier. Returns error if webhookURL is empty.
func New(webhookURL string, opts ...Option) (*Notifier, error) {
	if webhookURL == "" {
		return nil, ErrEmptyURL
	}

	n := &Notifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}

	for _, opt := range opts {
		opt(n)
	}

	return n, nil
}

// Notify builds a Block Kit payload and posts it to the configured webhook URL.
func (n *Notifier) Notify(ctx context.Context, event slack.Event) error {
	payload := buildPayload(event)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Block Kit payload types.

type payload struct {
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	Color  string  `json:"color"`
	Blocks []block `json:"blocks"`
}

type block struct {
	Type   string    `json:"type"`
	Text   *textObj  `json:"text,omitempty"`
	Fields []textObj `json:"fields,omitempty"`
}

type textObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type eventMeta struct {
	color string
	emoji string
	title string
}

func metaFor(eventType slack.EventType) eventMeta {
	switch eventType {
	case slack.EventSubscriptionCreated:
		return eventMeta{color: "#36a64f", emoji: "\U0001f389", title: "New Subscription"}
	case slack.EventSubscriptionCancelled:
		return eventMeta{color: "#d32f2f", emoji: "\U0001f6a8", title: "Subscription Cancelled"}
	case slack.EventUserDeleted:
		return eventMeta{color: "#d32f2f", emoji: "\U0001f6a8", title: "User Deleted"}
	case slack.EventError:
		return eventMeta{color: "#d32f2f", emoji: "\U0001f6a8", title: "Backend Error"}
	default:
		return eventMeta{color: "#439fe0", emoji: "\u2139\ufe0f", title: "Notification"}
	}
}

func buildPayload(event slack.Event) payload {
	meta := metaFor(event.Type)

	headerBlock := block{
		Type: "section",
		Text: &textObj{
			Type: "mrkdwn",
			Text: fmt.Sprintf("%s *%s*", meta.emoji, meta.title),
		},
	}

	var blocks []block

	blocks = append(blocks, headerBlock)

	if event.Type == slack.EventError {
		blocks = append(blocks, buildErrorBlock(event))
	} else {
		blocks = append(blocks, buildFieldsBlock(event))
	}

	return payload{
		Attachments: []attachment{
			{
				Color:  meta.color,
				Blocks: blocks,
			},
		},
	}
}

func buildErrorBlock(event slack.Event) block {
	// Build code block content with sorted keys for deterministic output.
	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var content string
	for _, k := range keys {
		content += fmt.Sprintf("%s: %s\n", k, event.Fields[k])
	}

	if event.Message != "" {
		content = fmt.Sprintf("message: %s\n%s", event.Message, content)
	}

	return block{
		Type: "section",
		Text: &textObj{
			Type: "mrkdwn",
			Text: fmt.Sprintf("```\n%s```", content),
		},
	}
}

func buildFieldsBlock(event slack.Event) block {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	fields := make([]textObj, 0, len(keys)+1)

	// Add event message as the first field.
	if event.Message != "" {
		fields = append(fields, textObj{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Event:*\n%s", event.Message),
		})
	}

	for _, k := range keys {
		fields = append(fields, textObj{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*%s:*\n%s", capitalizeFirst(k), event.Fields[k]),
		})
	}

	return block{
		Type:   "section",
		Fields: fields,
	}
}

// capitalizeFirst returns the string with its first byte uppercased.
// Works for ASCII field keys used in this project.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}

	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}

	return string(b)
}
