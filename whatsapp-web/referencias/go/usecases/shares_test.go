package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
)

type shareRig struct {
	shares *Shares
	ap     *Approvals
	out    *selfOut
	wa     *thirdParty
}

func newShareRig() shareRig {
	out := &selfOut{}
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ap := NewApprovals(ApprovalsDeps{Store: memstore.New(), Out: out, Now: func() time.Time { return now }, Log: quiet})
	wa := &thirdParty{on: map[string]bool{"+56900000000": true}}
	shares := NewShares(SharesDeps{Approvals: ap, Sender: wa, Vault: vault{}, Limits: domain.DefaultShareLimits(), Log: quiet})
	ap.Register(ShareKind, shares)
	return shareRig{shares: shares, ap: ap, out: out, wa: wa}
}

var demoRecipient = domain.Recipient{Name: "Contacto Demo", E164: "9 0000 0000"}

// WAW-T04: sin 👍 no sale nada (ni con un «sí» escrito); con 👍 sale una vez al JID comprobado.
func TestWAW_T04_SinLikeNoSaleNada(t *testing.T) {
	r := newShareRig()
	a, err := r.shares.Prepare(context.Background(), demoRecipient, deliveries("f1.pdf", "f2.pdf"))
	must(t, err)
	handled, _ := r.ap.HandleText(context.Background(), "sí, mándalo")
	check(t, !handled && len(r.wa.sent) == 0, "el texto no envía")
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍"))
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍"))
	check(t, len(r.wa.sent) == 2 && strings.HasPrefix(r.wa.sent[0], "56900000000@s.whatsapp.net"), "una vez, al JID comprobado: %v", r.wa.sent)
}

// WAW-T05: un número inválido o que no está en WhatsApp no se propone.
func TestWAW_T05_NumeroInexistente(t *testing.T) {
	r := newShareRig()
	_, err := r.shares.Prepare(context.Background(), domain.Recipient{E164: "+56900000001"}, deliveries("f.pdf"))
	check(t, errors.Is(err, ErrNotOnWhatsApp), "no está en WhatsApp: %v", err)
	_, err = r.shares.Prepare(context.Background(), domain.Recipient{E164: "123"}, deliveries("f.pdf"))
	check(t, errors.Is(err, ErrBadNumber) && len(r.out.texts) == 0, "inválido y sin propuesta: %v", err)
}

// WAW-T06: más archivos que el tope, o ninguno, no se propone.
func TestWAW_T06_TopeDeArchivos(t *testing.T) {
	r := newShareRig()
	many := make([]string, 11)
	for i := range many {
		many[i] = "f.pdf"
	}
	_, err := r.shares.Prepare(context.Background(), demoRecipient, deliveries(many...))
	check(t, errors.Is(err, ErrTooManyFiles), "tope: %v", err)
	_, err = r.shares.Prepare(context.Background(), demoRecipient, nil)
	check(t, errors.Is(err, ErrNothingToSend), "vacío: %v", err)
}

// WAW-T07: la propuesta muestra el número COMPLETO, cada archivo y si el número es nuevo.
func TestWAW_T07_PropuestaVerificable(t *testing.T) {
	r := newShareRig()
	to := demoRecipient
	to.New = true
	_, err := r.shares.Prepare(context.Background(), to, deliveries("f1.pdf", "f2.pdf"))
	must(t, err)
	text := r.out.texts[0]
	for _, want := range []string{"+56900000000", "• f1.pdf", "• f2.pdf", "número nuevo", "Vence"} {
		check(t, strings.Contains(text, want), "falta %q en %q", want, text)
	}
}
