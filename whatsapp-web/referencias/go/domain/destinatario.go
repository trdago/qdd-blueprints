package domain

import "time"

// Recipient es el destinatario de un envío a un tercero. JID lo llena la comprobación en WhatsApp, nunca la persona.
type Recipient struct {
	Name string
	E164 string
	JID  string
	New  bool // nunca antes fue destinatario: la propuesta lo advierte
}

// Delivery es un archivo a entregar (por la cola al chat propio o en un envío a un tercero).
type Delivery struct {
	DocType  string
	Label    string
	Path     string
	Filename string
	Mime     string
	Caption  string
}

// QueuedDelivery es una entrega en la cola de salida.
type QueuedDelivery struct {
	Delivery
	ID       int64
	Attempts int
	QueuedAt time.Time
}

// Estados de una entrega en la cola de salida.
const (
	OutboxPending = "pendiente"
	OutboxSent    = "enviado"
	OutboxError   = "error"
)

// ShareLimits son los topes de un envío a un tercero.
type ShareLimits struct {
	MaxFiles int
	TTL      time.Duration
}

// DefaultShareLimits son los del proyecto de origen.
func DefaultShareLimits() ShareLimits {
	return ShareLimits{MaxFiles: 10, TTL: 10 * time.Minute}
}
