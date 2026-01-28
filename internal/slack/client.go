package slack

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) SendMessage(channel, message string) error {
	return nil
}
