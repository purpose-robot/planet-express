package email

import (
	"bytes"
	"context"
	"io"
)

type TemplateElement string

const (
	ElementSubject TemplateElement = "subject"
	ElementContent TemplateElement = "content"
)

type Sender interface {
	Send(ctx context.Context, from, recipient, subject, content string) error
}

type Renderer interface {
	Render(w io.Writer, name string, element TemplateElement, data any) error
}

type Service struct {
	config   ServiceConfig
	sender   Sender
	renderer Renderer
}

type ServiceConfig struct {
	From string
}

func NewService(config ServiceConfig, sender Sender, renderer Renderer) *Service {
	return &Service{
		config:   config,
		sender:   sender,
		renderer: renderer,
	}
}

func (s *Service) Send(ctx context.Context, name, recipient string, data any) error {
	var (
		subjectBuf bytes.Buffer
		contentBuf bytes.Buffer
	)

	viewData := struct {
		Views  any
		Global any
	}{
		Views:  data,
		Global: s.config,
	}

	if err := s.renderer.Render(&subjectBuf, name, ElementSubject, viewData); err != nil {
		return err
	}

	if err := s.renderer.Render(&contentBuf, name, ElementContent, viewData); err != nil {
		return err
	}

	return s.sender.Send(ctx, s.config.From, recipient, subjectBuf.String(), contentBuf.String())
}
