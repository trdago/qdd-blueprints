// Package usecases son los flujos del vigía: ingesta con lista blanca, cola de salida al chat propio, aprobación con 👍
// y envío confirmado a terceros.
package usecases

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// IngestConfig son los umbrales de la ingesta.
type IngestConfig struct {
	MinConfidence      float64
	DailyClassifyMax   int
	MaxAttachmentBytes int64
	MaxClassifyChars   int
	Proposable         map[domain.Category]bool // categorías que generan propuesta
	Trivial            domain.TrivialRules
	OwnRUT             string
}

// DefaultIngestConfig son los valores del proyecto de origen.
func DefaultIngestConfig() IngestConfig {
	return IngestConfig{MinConfidence: 0.70, DailyClassifyMax: 300, MaxAttachmentBytes: 25 << 20, MaxClassifyChars: 4000,
		Proposable: map[domain.Category]bool{"factura_o_pago": true, "documento_legal": true}, Trivial: domain.DefaultTrivialRules()}
}

// IngestDeps son los puertos de la ingesta. Text puede ser nil (sin extracción de texto).
type IngestDeps struct {
	Classifier ports.Classifier
	Store      ports.MessageStore
	Media      ports.MediaStore
	Text       ports.TextExtractor
	Now        func() time.Time
	Log        *slog.Logger
}

// Ingestor: lista blanca → adjunto con tope → trivial → clasificador → propuesta. Un mensaje fuera de la lista blanca se
// descarta sin guardar, descargar ni clasificar.
type Ingestor struct {
	deps      IngestDeps
	cfg       IngestConfig
	directory atomic.Pointer[domain.Directory]
}

// NewIngestor arranca con la lista blanca vacía: no acepta nada hasta SetDirectory.
func NewIngestor(deps IngestDeps, cfg IngestConfig) *Ingestor {
	in := &Ingestor{deps: deps, cfg: cfg}
	in.SetDirectory(domain.NewDirectory(nil, time.Time{}))
	return in
}

// SetDirectory reemplaza la lista blanca de forma atómica.
func (in *Ingestor) SetDirectory(d domain.Directory) {
	in.directory.Store(&d)
}

// Accepts dice si el mensaje es de un chat vigilado. Se llama ANTES de cualquier otra cosa.
func (in *Ingestor) Accepts(msg domain.IncomingMessage) (domain.Contact, bool) {
	return in.directory.Load().Match(msg)
}

// Handle procesa un mensaje. fetch descarga el adjunto (puede ser nil).
func (in *Ingestor) Handle(ctx context.Context, msg domain.IncomingMessage, fetch ports.AttachmentFetcher) error {
	contact, ok := in.Accepts(msg)
	if !ok || contact.SkipsText(msg) {
		return nil
	}
	att, proceed, err := in.persist(ctx, msg, contact, fetch)
	if err != nil || !proceed {
		return err
	}
	return in.classify(ctx, msg, contact, att)
}

// persist guarda el adjunto y el mensaje. proceed=false si el mensaje ya existía (idempotencia) o es trivial.
func (in *Ingestor) persist(ctx context.Context, msg domain.IncomingMessage, contact domain.Contact, fetch ports.AttachmentFetcher) (ports.SavedAttachment, bool, error) {
	att, err := in.saveAttachment(ctx, msg, fetch)
	if err != nil {
		return att, false, err
	}
	trivial := in.cfg.Trivial.IsTrivial(msg.Text, msg.Attachment != nil)
	inserted, err := in.deps.Store.InsertMessage(ctx, ports.StoredMessage{
		Message: msg, CompanyID: contact.CompanyID, AttachmentSHA256: att.SHA256, Trivial: trivial, ReceivedAt: in.deps.Now(),
	})
	return att, inserted && !trivial, err
}

// downloadable: hay adjunto y su tamaño declarado no pasa el tope (se decide antes de descargar).
func (in *Ingestor) downloadable(msg domain.IncomingMessage, fetch ports.AttachmentFetcher) bool {
	return msg.Attachment != nil && fetch != nil && msg.Attachment.Bytes <= in.cfg.MaxAttachmentBytes
}

func (in *Ingestor) saveAttachment(ctx context.Context, msg domain.IncomingMessage, fetch ports.AttachmentFetcher) (ports.SavedAttachment, error) {
	if !in.downloadable(msg, fetch) {
		return ports.SavedAttachment{}, nil
	}
	data, err := fetch(ctx)
	if err != nil {
		in.deps.Log.Warn("no se pudo descargar el adjunto; se guarda solo el mensaje", "id", msg.MessageID, "err", err)
		return ports.SavedAttachment{}, nil
	}
	att, err := in.deps.Media.Save(ctx, data, msg.Attachment.FileName, msg.Attachment.Mime)
	att.Text = in.extractText(data, att.Mime)
	return att, err
}

func (in *Ingestor) extractText(data []byte, mime string) string {
	if in.deps.Text == nil {
		return ""
	}
	text, err := in.deps.Text.Extract(data, mime)
	if err != nil {
		in.deps.Log.Warn("no se pudo extraer texto del adjunto", "mime", mime, "err", err)
	}
	return text
}

// classify consume un cupo diario. Sin cupo o con el clasificador caído, el mensaje queda pendiente (se reintenta después).
func (in *Ingestor) classify(ctx context.Context, msg domain.IncomingMessage, contact domain.Contact, att ports.SavedAttachment) error {
	reserved, err := in.deps.Store.ReserveClassification(ctx, in.deps.Now(), in.cfg.DailyClassifyMax)
	if err != nil || !reserved {
		return err
	}
	cls, err := in.deps.Classifier.Classify(ctx, domain.ClassificationText(msg, att.Text, in.cfg.MaxClassifyChars))
	if err != nil {
		in.deps.Log.Warn("el clasificador no respondió; el mensaje queda pendiente", "id", msg.MessageID, "err", err)
		return nil
	}
	return in.record(ctx, msg, contact, att, cls)
}

func (in *Ingestor) record(ctx context.Context, msg domain.IncomingMessage, contact domain.Contact, att ports.SavedAttachment, cls domain.Classification) error {
	now := in.deps.Now()
	if err := in.deps.Store.SetClassification(ctx, msg.ChatJID, msg.MessageID, cls, now); err != nil {
		return err
	}
	if !in.proposable(cls) {
		return nil
	}
	return in.deps.Store.InsertProposal(ctx, domain.BuildProposal(domain.ProposalInput{
		Message: msg, Contact: contact, Category: cls.Category, AttachmentText: att.Text,
		AttachmentSHA256: att.SHA256, Now: now, OwnRUT: in.cfg.OwnRUT,
	}))
}

// proposable: categoría que se registra y confianza suficiente. Lo que queda bajo el umbral es «dudoso»: se guarda con
// su confianza y se revisa a mano.
func (in *Ingestor) proposable(cls domain.Classification) bool {
	return in.cfg.Proposable[cls.Category] && cls.Confidence >= in.cfg.MinConfidence
}
