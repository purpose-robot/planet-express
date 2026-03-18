package smtp

import (
	"context"
	"time"

	"github.com/wneessen/go-mail"
)

type Sender struct {
	client *mail.Client
}

func NewSender(host string, port int, username, password string) (*Sender, error) {
	client, err := mail.NewClient(
		host,
		mail.WithPort(port),
		mail.WithTimeout(5*time.Second),
		mail.WithUsername(username),
		mail.WithPassword(password),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return nil, err
	}

	return &Sender{
		client: client,
	}, nil
}

func (s *Sender) Send(ctx context.Context, from, recipient, subject, content string) error {
	message := mail.NewMsg()

	if err := message.From(from); err != nil {
		return err
	}

	if err := message.To(recipient); err != nil {
		return err
	}

	message.Subject(subject)
	message.SetBodyString(mail.TypeTextPlain, content)

	return s.client.DialAndSendWithContext(ctx, message)
}
