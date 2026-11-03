package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
)

func newOutboxRig(out *selfOut, missing map[string]bool) (*Outbox, *memstore.Store) {
	store := memstore.New()
	o := NewOutbox(OutboxDeps{Store: store, Vault: vault{missing: missing}, Sender: out, Now: time.Now, Log: quiet},
		OutboxConfig{Batch: 2, MaxAttempts: 2})
	return o, store
}

func deliveries(names ...string) []domain.Delivery {
	out := make([]domain.Delivery, 0, len(names))
	for _, n := range names {
		out = append(out, domain.Delivery{Path: "/datos/" + n, Filename: n, Caption: "*Archivo:* " + n})
	}
	return out
}

// WAW-O01: se envía en orden de encolado, por lotes, con su pie de foto.
func TestWAW_O01_OrdenYLote(t *testing.T) {
	out := &selfOut{}
	o, store := newOutboxRig(out, nil)
	must(t, store.Enqueue(context.Background(), deliveries("1.pdf", "2.pdf", "3.pdf"), time.Now()))
	must(t, o.Run(context.Background()))
	check(t, len(out.docs) == 2 && out.docs[0].Filename == "1.pdf" && out.docs[0].Caption == "*Archivo:* 1.pdf", "primer lote: %+v", out.docs)
	must(t, o.Run(context.Background()))
	check(t, len(out.docs) == 3 && out.docs[2].Filename == "3.pdf", "segundo lote")
}

// WAW-O02: si el envío falla queda pendiente y se reintenta, hasta el máximo de intentos.
func TestWAW_O02_ReintentoHastaElMaximo(t *testing.T) {
	out := &selfOut{fail: errors.New("sin conexión")}
	o, store := newOutboxRig(out, nil)
	must(t, store.Enqueue(context.Background(), deliveries("1.pdf"), time.Now()))
	for range 3 {
		must(t, o.Run(context.Background()))
	}
	state, attempts := store.OutboxStatus(1)
	check(t, state == domain.OutboxPending && attempts == 2, "dos intentos y no más: %s %d", state, attempts)
	out.fail = nil
	must(t, o.Run(context.Background()))
	check(t, len(out.docs) == 0, "agotados los intentos ya no se toma")
}

// WAW-O03: un archivo que no se puede leer es error permanente: no se reintenta y no frena a los demás.
func TestWAW_O03_ErrorPermanente(t *testing.T) {
	out := &selfOut{}
	o, store := newOutboxRig(out, map[string]bool{"/datos/falta.pdf": true})
	must(t, store.Enqueue(context.Background(), deliveries("falta.pdf", "ok.pdf"), time.Now()))
	must(t, o.Run(context.Background()))
	state, attempts := store.OutboxStatus(1)
	check(t, state == domain.OutboxError && attempts == 1 && len(out.docs) == 1, "error permanente y el otro sale: %s %d", state, attempts)
}
