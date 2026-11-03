package usecases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// clock es un reloj fijo que se puede adelantar.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

// classifier responde siempre lo mismo y cuenta llamadas.
type classifier struct {
	calls int
	out   domain.Classification
	err   error
}

func (c *classifier) Classify(context.Context, string) (domain.Classification, error) {
	c.calls++
	return c.out, c.err
}

// selfOut registra lo que se escribe en el chat propio y devuelve ids m1, m2…
type selfOut struct {
	texts []string
	docs  []ports.OutDocument
	fail  error
}

func (s *selfOut) SendToSelfID(_ context.Context, text string) (string, error) {
	s.texts = append(s.texts, text)
	return fmt.Sprintf("m%d", len(s.texts)), nil
}

func (s *selfOut) SendDocumentToSelf(_ context.Context, doc ports.OutDocument) error {
	if s.fail != nil {
		return s.fail
	}
	s.docs = append(s.docs, doc)
	return nil
}

// vault entrega bytes fijos, o falla para las rutas marcadas.
type vault struct{ missing map[string]bool }

func (v vault) Fetch(_ context.Context, d domain.Delivery) (ports.OutDocument, error) {
	if v.missing[d.Path] {
		return ports.OutDocument{}, errors.New("no existe")
	}
	return ports.OutDocument{Filename: d.Filename, Mime: d.Mime, Data: []byte("x")}, nil
}

// thirdParty simula WhatsApp: qué números existen y qué se envió.
type thirdParty struct {
	on   map[string]bool
	sent []string
}

func (f *thirdParty) Lookup(_ context.Context, e164 string) (string, bool, error) {
	return e164[1:] + "@s.whatsapp.net", f.on[e164], nil
}

func (f *thirdParty) SendDocument(_ context.Context, to domain.Recipient, doc ports.OutDocument) error {
	f.sent = append(f.sent, to.JID+" "+doc.Filename)
	return nil
}
