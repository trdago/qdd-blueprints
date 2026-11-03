// Package domain son las reglas puras del vigía de WhatsApp: mensajes, lista blanca, chat propio, aprobación con 👍,
// destinatarios validados y normalización determinista. No hace IO.
package domain

import (
	"strings"
	"time"
	"unicode"
)

// Category es la clasificación de un mensaje para la empresa. Las categorías las elige cada proyecto.
type Category string

// Categorías que asigna el propio vigía (no el clasificador).
const (
	CategoryPending Category = "pendiente" // sin clasificar todavía (sin cupo o el clasificador falló)
	CategoryTrivial Category = "trivial"   // no aporta: no se clasifica
)

// Source indica si el mensaje llegó en vivo o en el historial que entrega WhatsApp.
type Source string

// Fuentes de un mensaje.
const (
	SourceRealtime Source = "en_vivo"
	SourceHistory  Source = "historial"
)

// AttachmentRef describe un adjunto ANTES de descargarlo (la descarga ocurre después de la lista blanca).
type AttachmentRef struct {
	FileName string
	Mime     string
	Bytes    int64
}

// IncomingMessage es un mensaje de WhatsApp ya traducido desde la biblioteca.
type IncomingMessage struct {
	ChatJID    string
	MessageID  string
	ChatPhone  string // E.164 del otro participante en un chat 1:1; vacío en grupos
	IsGroup    bool
	FromMe     bool
	SenderName string
	SentAt     time.Time
	Text       string
	Attachment *AttachmentRef
	Source     Source
}

// Key es la clave de idempotencia de un mensaje: (chat, id del mensaje).
func (m IncomingMessage) Key() string {
	return m.ChatJID + "/" + m.MessageID
}

// Classification es la respuesta del clasificador para un mensaje.
type Classification struct {
	Category     Category
	Confidence   float64
	InputTokens  int
	OutputTokens int
}

// TrivialRules decide sin IA qué mensajes no aportan.
type TrivialRules struct {
	Words    map[string]bool
	MinChars int
}

// DefaultTrivialRules son las palabras y el largo mínimo del proyecto de origen.
func DefaultTrivialRules() TrivialRules {
	words := []string{"ok", "oka", "okey", "okay", "dale", "listo", "gracias", "muchas gracias", "si", "sí", "no",
		"perfecto", "genial", "buenas", "hola", "buenos días", "buenas tardes", "buenas noches", "vale", "de acuerdo", "👍"}
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return TrivialRules{Words: set, MinChars: 12}
}

// IsTrivial: un mensaje con adjunto nunca es trivial; sin adjunto lo es si es una palabra de la lista, no tiene letras ni
// dígitos (solo emojis o signos) o es más corto que MinChars.
func (r TrivialRules) IsTrivial(text string, hasAttachment bool) bool {
	if hasAttachment {
		return false
	}
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(text), "!.¡ "))
	if r.Words[normalized] || !hasLetterOrDigit(normalized) {
		return true
	}
	return len([]rune(normalized)) < r.MinChars
}

func hasLetterOrDigit(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// ClassificationText arma lo que se manda al clasificador: texto, nombre del adjunto y extracto de su contenido,
// recortado a maxChars runas.
func ClassificationText(msg IncomingMessage, attachmentText string, maxChars int) string {
	parts := []string{strings.TrimSpace(msg.Text)}
	if msg.Attachment != nil {
		parts = append(parts, "Adjunto: "+msg.Attachment.FileName)
	}
	parts = append(parts, strings.TrimSpace(attachmentText))
	return truncateRunes(strings.TrimSpace(strings.Join(parts, "\n")), maxChars)
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}
