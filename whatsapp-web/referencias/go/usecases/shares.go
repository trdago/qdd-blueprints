package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// ShareKind es el tipo de acción de un envío a un tercero en el motor de aprobación.
const ShareKind = "envio_tercero"

// Negativas de un envío a un tercero (no se propone nada).
var (
	ErrBadNumber     = errors.New("el número no es válido")
	ErrNotOnWhatsApp = errors.New("ese número no está en WhatsApp")
	ErrTooManyFiles  = errors.New("son más archivos que el tope por envío")
	ErrNothingToSend = errors.New("no hay archivos que enviar")
)

// SharesDeps arma el envío confirmado a terceros.
type SharesDeps struct {
	Approvals *Approvals
	Sender    ports.ThirdPartySender
	Vault     ports.DocumentFetcher
	Limits    domain.ShareLimits
	Pause     time.Duration // espera entre archivos, para que no salgan en ráfaga
	Log       *slog.Logger
}

// Shares es el ÚNICO código que llama a ThirdPartySender.SendDocument, y solo como ejecutor de una propuesta aprobada con
// 👍 (TestOnlyTheSharesUseCaseCallsThirdPartySend lo verifica). Solo envía documentos: nunca texto libre.
type Shares struct{ deps SharesDeps }

type sharePayload struct {
	To    domain.Recipient  `json:"to"`
	Items []domain.Delivery `json:"items"`
}

// NewShares crea el envío confirmado; se registra en el motor como ejecutor de ShareKind.
func NewShares(deps SharesDeps) *Shares {
	return &Shares{deps: deps}
}

// Prepare valida destinatario y topes, comprueba el número en WhatsApp y deja la propuesta en el chat propio.
func (s *Shares) Prepare(ctx context.Context, to domain.Recipient, items []domain.Delivery) (domain.Action, error) {
	if err := s.checkItems(items); err != nil {
		return domain.Action{}, err
	}
	to, err := s.resolve(ctx, to)
	if err != nil {
		return domain.Action{}, err
	}
	payload, err := json.Marshal(sharePayload{To: to, Items: items})
	if err != nil {
		return domain.Action{}, err
	}
	return s.deps.Approvals.Propose(ctx, ShareKind, shareSummary(to, items), payload, s.deps.Limits.TTL)
}

func (s *Shares) checkItems(items []domain.Delivery) error {
	if len(items) == 0 {
		return ErrNothingToSend
	}
	if len(items) > s.deps.Limits.MaxFiles {
		return fmt.Errorf("%w: %d de %d", ErrTooManyFiles, len(items), s.deps.Limits.MaxFiles)
	}
	return nil
}

// resolve normaliza el número a E.164 y toma el JID que devuelve WhatsApp: nunca uno escrito a mano.
func (s *Shares) resolve(ctx context.Context, to domain.Recipient) (domain.Recipient, error) {
	e164, ok := domain.NormalizePhoneE164(to.E164)
	if !ok {
		return to, ErrBadNumber
	}
	jid, on, err := s.deps.Sender.Lookup(ctx, e164)
	if err != nil {
		return to, err
	}
	if !on {
		return to, ErrNotOnWhatsApp
	}
	to.E164, to.JID = e164, jid
	return to, nil
}

// shareSummary es lo que la persona aprueba: nombre y número COMPLETO (es lo que debe verificar) y cada archivo.
func shareSummary(to domain.Recipient, items []domain.Delivery) string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, "• "+it.Filename)
	}
	who := to.Name + " (" + to.E164 + ")"
	if to.New {
		who += " · número nuevo: verifícalo"
	}
	return "enviar por WhatsApp a " + who + "\n" + strings.Join(names, "\n")
}

// Execute envía cada archivo de una propuesta aprobada y cuenta cuántos salieron.
func (s *Shares) Execute(ctx context.Context, a domain.Action) (string, error) {
	var p sharePayload
	if err := json.Unmarshal(a.Payload, &p); err != nil {
		return "", err
	}
	sent, failed := s.sendAll(ctx, p)
	result := fmt.Sprintf("%d de %d enviados a %s", sent, len(p.Items), domain.MaskPhone(p.To.E164))
	if sent == 0 {
		return "", errors.New(result + ": " + strings.Join(failed, "; "))
	}
	return result, nil
}

func (s *Shares) sendAll(ctx context.Context, p sharePayload) (int, []string) {
	sent := 0
	var failed []string
	for i, it := range p.Items {
		if i > 0 {
			wait(ctx, s.deps.Pause)
		}
		if err := s.sendOne(ctx, p.To, it); err != nil {
			failed = append(failed, it.Filename+": "+err.Error())
			continue
		}
		sent++
	}
	return sent, failed
}

func (s *Shares) sendOne(ctx context.Context, to domain.Recipient, it domain.Delivery) error {
	doc, err := s.deps.Vault.Fetch(ctx, it)
	if err != nil {
		return err
	}
	doc.Caption = it.Caption
	return s.deps.Sender.SendDocument(ctx, to, doc)
}

func wait(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
