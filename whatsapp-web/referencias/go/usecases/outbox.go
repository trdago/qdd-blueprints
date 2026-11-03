package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// OutboxDeps son las dependencias del drenador de la cola de salida.
type OutboxDeps struct {
	Store  ports.OutboxStore
	Vault  ports.DocumentFetcher
	Sender ports.DocumentReplier
	Now    func() time.Time
	Log    *slog.Logger
}

// OutboxConfig ajusta el lote y el máximo de intentos por entrega.
type OutboxConfig struct {
	Batch       int
	MaxAttempts int
}

// Outbox drena la cola: lee cada archivo y lo adjunta en el chat propio. Quien encola no envía (puede ser otro proceso
// sin la sesión); solo el proceso con la sesión drena.
type Outbox struct {
	deps OutboxDeps
	cfg  OutboxConfig
}

// NewOutbox crea el drenador; lote 20 y 5 intentos si no se indican.
func NewOutbox(deps OutboxDeps, cfg OutboxConfig) *Outbox {
	if cfg.Batch <= 0 {
		cfg.Batch = 20
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	return &Outbox{deps: deps, cfg: cfg}
}

// Run envía las entregas pendientes en orden de encolado; un fallo en una no detiene a las demás.
func (o *Outbox) Run(ctx context.Context) error {
	items, err := o.deps.Store.Pending(ctx, o.cfg.Batch, o.cfg.MaxAttempts)
	if err != nil {
		return err
	}
	for i := range items {
		o.sendOne(ctx, items[i])
	}
	return nil
}

func (o *Outbox) sendOne(ctx context.Context, item domain.QueuedDelivery) {
	doc, err := o.deps.Vault.Fetch(ctx, item.Delivery)
	if err != nil {
		o.mark(o.deps.Store.MarkFailed(ctx, item.ID, err.Error()), item, "error permanente", err)
		return
	}
	doc.Caption = item.Caption
	if err := o.deps.Sender.SendDocumentToSelf(ctx, doc); err != nil {
		o.mark(o.deps.Store.MarkRetry(ctx, item.ID, err.Error()), item, "queda pendiente", err)
		return
	}
	o.mark(o.deps.Store.MarkSent(ctx, item.ID, o.deps.Now()), item, "enviado", nil)
}

// mark registra el resultado de una entrega (y si no se pudo anotar en la base).
func (o *Outbox) mark(storeErr error, item domain.QueuedDelivery, outcome string, cause error) {
	if cause != nil {
		o.deps.Log.Warn("entrega del chat propio: "+outcome, "archivo", item.Filename, "err", cause)
	}
	if storeErr != nil {
		o.deps.Log.Warn("no se pudo anotar la entrega en la cola", "id", item.ID, "err", storeErr)
	}
}
