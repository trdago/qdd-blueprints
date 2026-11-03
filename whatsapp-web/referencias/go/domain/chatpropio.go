package domain

import "strings"

// Marcas con que empiezan los mensajes que escribe el agente en el chat propio (anti-bucle): lo que empieza así nunca se
// vuelve a procesar como pregunta.
const (
	SelfReplyMark  = "🤖"
	AudioHeardMark = "🎙️"
)

// LikeEmoji es la única reacción que aprueba una propuesta.
const LikeEmoji = "👍"

// IsSelfReply indica si un texto del chat propio lo escribió el agente.
func IsSelfReply(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, SelfReplyMark) || strings.HasPrefix(t, AudioHeardMark)
}

// WithReplyMark antepone la marca del agente si el texto no la trae.
func WithReplyMark(text string) string {
	if IsSelfReply(text) {
		return text
	}
	return SelfReplyMark + " " + text
}

// IsLike: la reacción aprueba (👍 con o sin tono de piel). Una reacción vacía es «quitó la reacción» y no aprueba.
func IsLike(emoji string) bool {
	return strings.HasPrefix(emoji, LikeEmoji)
}

// ParseSelfCommand reconoce «<prefijo> <pregunta>» sin distinguir mayúsculas. El prefijo debe ir seguido de espacio,
// «:», «,» o salto de línea, para no confundirlo con el comienzo de otra palabra.
func ParseSelfCommand(text, prefix string) (string, bool) {
	t := strings.TrimSpace(text)
	if !startsWithFold(t, prefix) {
		return "", false
	}
	rest := t[len(prefix):]
	if rest != "" && !strings.ContainsAny(rest[:1], " :,\n") {
		return "", false
	}
	body := strings.TrimSpace(strings.TrimLeft(rest, ":,"))
	return body, body != ""
}

func startsWithFold(t, prefix string) bool {
	return prefix != "" && len(t) >= len(prefix) && strings.EqualFold(t[:len(prefix)], prefix)
}
