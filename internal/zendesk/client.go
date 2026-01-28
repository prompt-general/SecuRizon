package zendesk

import "context"

type Client struct{}

type Ticket struct {
	ID string
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) CreateTicket(ctx context.Context, ticket interface{}) (*Ticket, error) {
	return &Ticket{ID: "zd_123"}, nil
}

func (c *Client) AddComment(ctx context.Context, ticketID string, comment interface{}) error {
	return nil
}
