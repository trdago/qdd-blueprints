package domain

import (
	"fmt"
	"time"
)

// Estados de una acción propuesta en el chat propio.
const (
	ActionPending   = "pendiente"
	ActionRunning   = "ejecutando"
	ActionDone      = "hecha"
	ActionFailed    = "fallida"
	ActionExpired   = "vencida"
	ApprovedByLike  = "reaccion"
	DefaultProposal = 15 * time.Minute
)

// Action es algo que el agente propone en un mensaje propio y que solo corre con un 👍 de la persona sobre ese mensaje.
type Action struct {
	ID         int64
	Kind       string // qué ejecutor la corre
	MessageID  string // mensaje de WhatsApp de la propuesta: el 👍 va sobre este
	Summary    string // lo que la persona aprueba, tal cual se le mostró
	Payload    []byte // datos del ejecutor (p. ej. destinatario y archivos), opacos para el motor
	State      string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ApprovedAt *time.Time
	ApprovedBy string
	Result     string
}

// Approvable: la propuesta sigue pendiente y no venció.
func (a Action) Approvable(now time.Time) bool {
	return a.State == ActionPending && now.Before(a.ExpiresAt)
}

// FormatProposal es el mensaje de una propuesta: una acción por mensaje, para que el 👍 caiga sobre una sola.
func FormatProposal(a Action) string {
	return WithReplyMark(fmt.Sprintf("*Propuesta*\n*Qué:* %s\n*Aprobar:* reacciona %s a este mensaje\n_Vence a las %s_",
		a.Summary, LikeEmoji, a.ExpiresAt.Local().Format("15:04")))
}

// FormatResult es el aviso de cómo terminó una acción aprobada.
func FormatResult(a Action) string {
	return WithReplyMark(fmt.Sprintf("*Resultado*\n*Qué:* %s\n*Estado:* %s\n*Detalle:* %s", a.Summary, a.State, a.Result))
}

// ProgressDue dice si toca un aviso de avance: el primero sale `every` después de ver la tarea (el lanzamiento ya avisó)
// y luego a lo más uno cada `every`.
func ProgressDue(seenAt time.Time, lastNotice *time.Time, now time.Time, every time.Duration) bool {
	since := seenAt
	if lastNotice != nil {
		since = *lastNotice
	}
	return !now.Before(since.Add(every))
}
