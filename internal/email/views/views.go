package views

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"text/template"

	"github.com/purpose-robot/planet-express/internal/email"
)

type Renderer struct {
	views map[string]*View
}

func NewRenderer(viewFS fs.FS) (*Renderer, error) {
	templates, err := fs.Glob(viewFS, "*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("no email templates found in directory")
	}

	views := make(map[string]*View, len(templates))

	for _, tmpl := range templates {
		name := strings.TrimSuffix(tmpl, ".tmpl")

		view, err := Parse(viewFS, name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse view %q: %w", name, err)
		}

		views[name] = view
	}

	return &Renderer{
		views: views,
	}, nil
}

func (r *Renderer) Render(w io.Writer, name string, element email.TemplateElement, data any) error {
	v, ok := r.views[name]
	if ok {
		return v.Render(w, element, data)
	}

	return fmt.Errorf("view %q not found", name)
}

type View struct {
	tmpl *template.Template
}

func Parse(fs fs.FS, name string) (*View, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s.tmpl", name)
	tmpl, err := template.New(name).ParseFS(fs, filename)
	if err != nil {
		return nil, err
	}

	subjectTmpl := tmpl.Lookup(string(email.ElementSubject))
	if subjectTmpl == nil {
		return nil, fmt.Errorf("missing %s template", email.ElementSubject)
	}

	contentTmpl := tmpl.Lookup(string(email.ElementContent))
	if contentTmpl == nil {
		return nil, fmt.Errorf("missing %s template", email.ElementContent)
	}

	return &View{
		tmpl: tmpl,
	}, nil
}

func (v *View) Render(w io.Writer, element email.TemplateElement, data any) error {
	return v.tmpl.ExecuteTemplate(w, string(element), data)
}

func validateName(name string) error {
	for _, c := range name {
		if !validViewRune(c) {
			return fmt.Errorf("invalid character %v in view name: %s", c, name)
		}
	}

	return nil
}

func validViewRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
}
