// Package ports son los contratos entre los casos de uso y los adaptadores (WhatsApp, base de datos, clasificador).
package ports

import (
	"context"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// AttachmentFetcher descarga un adjunto. Se llama solo después de pasar la lista blanca.
type AttachmentFetcher func(ctx context.Context) ([]byte, error)

// Classifier es el clasificador intercambiable (un servicio de clasificación o un LLM). Recibe el texto ya recortado.
type Classifier interface {
	Classify(ctx context.Context, text string) (domain.Classification, error)
}

// ClassifierFunc adapta una función a Classifier.
type ClassifierFunc func(ctx context.Context, text string) (domain.Classification, error)

// Classify implementa Classifier.
func (f ClassifierFunc) Classify(ctx context.Context, text string) (domain.Classification, error) {
	return f(ctx, text)
}

// TextExtractor saca texto de un adjunto (p. ej. un PDF) en memoria.
type TextExtractor interface {
	Extract(data []byte, mime string) (string, error)
}

// SavedAttachment es un adjunto guardado por su huella.
type SavedAttachment struct {
	SHA256 string
	Mime   string
	Bytes  int64
	Text   string
}

// MediaStore guarda los bytes de un adjunto, nombrado por su sha256.
type MediaStore interface {
	Save(ctx context.Context, data []byte, fileName, mime string) (SavedAttachment, error)
}

// StoredMessage es un mensaje de un chat vigilado tal como se guarda.
type StoredMessage struct {
	Message          domain.IncomingMessage
	CompanyID        string
	AttachmentSHA256 string
	Trivial          bool
	ReceivedAt       time.Time
}

// MessageStore guarda mensajes, clasificaciones y propuestas. InsertMessage es idempotente por (chat, id del mensaje):
// inserted=false si ya existía.
type MessageStore interface {
	InsertMessage(ctx context.Context, m StoredMessage) (inserted bool, err error)
	ReserveClassification(ctx context.Context, day time.Time, dailyMax int) (bool, error)
	SetClassification(ctx context.Context, chatJID, messageID string, c domain.Classification, at time.Time) error
	InsertProposal(ctx context.Context, p domain.Proposal) error
}

// OutDocument es un archivo listo para adjuntar.
type OutDocument struct {
	Filename string
	Mime     string
	Data     []byte
	Caption  string
}

// SelfMessenger escribe en el chat propio y devuelve el id del mensaje (para asociarle el 👍).
type SelfMessenger interface {
	SendToSelfID(ctx context.Context, text string) (string, error)
}

// DocumentReplier adjunta un documento en el chat propio. No tiene destinatario: el destino sale de la sesión.
type DocumentReplier interface {
	SendDocumentToSelf(ctx context.Context, doc OutDocument) error
}

// ThirdPartySender es la única salida hacia otra persona. Solo la llama el caso de uso de envío confirmado.
type ThirdPartySender interface {
	Lookup(ctx context.Context, e164 string) (jid string, onWhatsApp bool, err error)
	SendDocument(ctx context.Context, to domain.Recipient, doc OutDocument) error
}

// DocumentFetcher lee los bytes de una entrega desde su fuente (disco local u otra).
type DocumentFetcher interface {
	Fetch(ctx context.Context, d domain.Delivery) (OutDocument, error)
}

// OutboxStore es la cola de salida al chat propio, persistida en la base.
type OutboxStore interface {
	Enqueue(ctx context.Context, items []domain.Delivery, at time.Time) error
	Pending(ctx context.Context, limit, maxAttempts int) ([]domain.QueuedDelivery, error)
	MarkSent(ctx context.Context, id int64, at time.Time) error
	MarkFailed(ctx context.Context, id int64, reason string) error
	MarkRetry(ctx context.Context, id int64, reason string) error
}

// ActionStore guarda las propuestas que se aprueban con 👍. Claim debe ser atómico: solo una aprobación la obtiene.
type ActionStore interface {
	InsertAction(ctx context.Context, a domain.Action) (int64, error)
	ActionByMessage(ctx context.Context, messageID string) (domain.Action, bool, error)
	ClaimAction(ctx context.Context, id int64, by string, at time.Time) (bool, error)
	FinishAction(ctx context.Context, id int64, state, result string) error
	ExpireActions(ctx context.Context, now time.Time) error
}
