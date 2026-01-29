package email

import (
	"fmt"
	"github.com/securizon/internal/tenant"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SendSubscriptionCancelledEmail(tenant *tenant.Tenant) error {
	fmt.Printf("Sending subscription cancelled email to tenant %s\n", tenant.ID)
	return nil
}

func (s *Service) Send(to, subject, body string) error {
	fmt.Printf("Sending email to %s: %s\n", to, subject)
	return nil
}
