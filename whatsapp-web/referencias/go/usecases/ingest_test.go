package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

const watched = "+56900000000"

type ingestRig struct {
	in    *Ingestor
	store *memstore.Store
	cls   *classifier
}

func newIngestRig(cfg IngestConfig, out domain.Classification) ingestRig {
	store := memstore.New()
	cls := &classifier{out: out}
	in := NewIngestor(IngestDeps{Classifier: cls, Store: store, Media: store, Now: time.Now, Log: quiet}, cfg)
	in.SetDirectory(domain.NewDirectory([]domain.Contact{{PhoneE164: watched, CompanyID: "demo", CompanyName: "Empresa Demo SpA"}}, time.Now()))
	return ingestRig{in: in, store: store, cls: cls}
}

func invoice(phone, id string) domain.IncomingMessage {
	return domain.IncomingMessage{ChatJID: phone[1:] + "@s.whatsapp.net", MessageID: id, ChatPhone: phone,
		Text: "Adjunto la factura N° 4 por $1.200.000", SentAt: time.Now(), Source: domain.SourceRealtime}
}

func counting(n *int) ports.AttachmentFetcher {
	return func(context.Context) ([]byte, error) { *n++; return []byte("pdf"), nil }
}

// WAW-C01: un chat fuera de la lista blanca no produce escrituras, descargas ni llamadas al clasificador.
func TestWAW_C01_ChatNoVigiladoSeDescarta(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{Category: "factura_o_pago", Confidence: 0.9})
	downloads := 0
	msg := invoice("+56900000001", "A1")
	msg.Attachment = &domain.AttachmentRef{FileName: "f.pdf", Bytes: 3}
	must(t, r.in.Handle(context.Background(), msg, counting(&downloads)))
	check(t, len(r.store.Messages) == 0 && downloads == 0 && r.cls.calls == 0, "nada: %d mensajes, %d descargas, %d clasificaciones",
		len(r.store.Messages), downloads, r.cls.calls)
}

// WAW-C03: el mismo (chat, id) dos veces se guarda y se clasifica una sola vez.
func TestWAW_C03_Idempotencia(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{Category: "factura_o_pago", Confidence: 0.9})
	for range 2 {
		must(t, r.in.Handle(context.Background(), invoice(watched, "A1"), nil))
	}
	check(t, len(r.store.Messages) == 1 && r.cls.calls == 1 && len(r.store.Proposals) == 1, "una vez: %d %d %d",
		len(r.store.Messages), r.cls.calls, len(r.store.Proposals))
}

// WAW-C04: un adjunto que declara más bytes que el tope no se descarga; el mensaje sí se guarda.
func TestWAW_C04_AdjuntoSobreElTope(t *testing.T) {
	cfg := DefaultIngestConfig()
	cfg.MaxAttachmentBytes = 10
	r := newIngestRig(cfg, domain.Classification{Category: "otro", Confidence: 0.9})
	downloads := 0
	msg := invoice(watched, "A2")
	msg.Attachment = &domain.AttachmentRef{FileName: "grande.pdf", Bytes: 11}
	must(t, r.in.Handle(context.Background(), msg, counting(&downloads)))
	check(t, downloads == 0 && len(r.store.Messages) == 1 && len(r.store.Media) == 0, "sin descarga, con mensaje")
	small := invoice(watched, "A3")
	small.Attachment = &domain.AttachmentRef{FileName: "f.pdf", Bytes: 3}
	must(t, r.in.Handle(context.Background(), small, counting(&downloads)))
	check(t, downloads == 1 && len(r.store.Media) == 1, "bajo el tope se descarga y se guarda por huella")
}

// WAW-C09: un trivial se guarda marcado y no se clasifica.
func TestWAW_C09_TrivialNoSeClasifica(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{})
	msg := invoice(watched, "T1")
	msg.Text = "gracias!"
	must(t, r.in.Handle(context.Background(), msg, nil))
	check(t, r.cls.calls == 0 && r.store.Messages[msg.Key()].Trivial, "trivial sin clasificar")
}

// WAW-K01: bajo el umbral de confianza no hay propuesta (queda «dudoso» con su confianza).
func TestWAW_K01_BajoElUmbralEsDudoso(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{Category: "factura_o_pago", Confidence: 0.69})
	must(t, r.in.Handle(context.Background(), invoice(watched, "D1"), nil))
	msg := invoice(watched, "D1")
	check(t, len(r.store.Proposals) == 0 && r.store.Classified[msg.Key()].Confidence == 0.69, "dudoso sin propuesta")
}

// WAW-K02: una categoría que no se registra no genera propuesta aunque la confianza sea alta.
func TestWAW_K02_CategoriaSinPropuesta(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{Category: "coordinacion_comercial", Confidence: 0.99})
	must(t, r.in.Handle(context.Background(), invoice(watched, "K2"), nil))
	check(t, len(r.store.Proposals) == 0 && len(r.store.Classified) == 1, "clasificado sin propuesta")
}

// WAW-K03: el clasificador es un puerto: otro clasificador (o uno caído) no cambia el flujo; caído, el mensaje queda pendiente.
func TestWAW_K03_ClasificadorIntercambiable(t *testing.T) {
	r := newIngestRig(DefaultIngestConfig(), domain.Classification{})
	r.cls.err = errors.New("servicio caído")
	must(t, r.in.Handle(context.Background(), invoice(watched, "K3"), nil))
	check(t, len(r.store.Messages) == 1 && len(r.store.Classified) == 0, "guardado y pendiente")
	other := ports.ClassifierFunc(func(context.Context, string) (domain.Classification, error) {
		return domain.Classification{Category: "factura_o_pago", Confidence: 1}, nil
	})
	in := NewIngestor(IngestDeps{Classifier: other, Store: r.store, Media: r.store, Now: time.Now, Log: quiet}, DefaultIngestConfig())
	in.SetDirectory(domain.NewDirectory([]domain.Contact{{PhoneE164: watched}}, time.Now()))
	must(t, in.Handle(context.Background(), invoice(watched, "K4"), nil))
	check(t, len(r.store.Proposals) == 1, "con otro clasificador propone")
}

// WAW-K04: sin cupo diario no se llama al clasificador.
func TestWAW_K04_TopeDiario(t *testing.T) {
	cfg := DefaultIngestConfig()
	cfg.DailyClassifyMax = 1
	r := newIngestRig(cfg, domain.Classification{Category: "otro", Confidence: 1})
	must(t, r.in.Handle(context.Background(), invoice(watched, "Q1"), nil))
	must(t, r.in.Handle(context.Background(), invoice(watched, "Q2"), nil))
	check(t, r.cls.calls == 1 && len(r.store.Messages) == 2, "una clasificación, dos mensajes")
}
