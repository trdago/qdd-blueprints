package domain

import (
	"strings"
	"unicode"
)

// phoneRules se evalúan en orden; la primera que acepta el número gana.
var phoneRules = []func(raw, digits string) (string, bool){
	internationalPhone,
	chileMobileNine,
	chileMobileEleven,
}

// NormalizePhoneE164 lleva un teléfono a E.164. Reglas para Chile: 9 dígitos que empiezan con 9 → +56; 11 dígitos que
// empiezan con 569 → +. Otros países solo si ya vienen con «+» (8 a 15 dígitos). false si no es válido.
func NormalizePhoneE164(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	digits := onlyDigits(raw)
	for _, rule := range phoneRules {
		if phone, ok := rule(raw, digits); ok {
			return phone, true
		}
	}
	return "", false
}

func internationalPhone(raw, digits string) (string, bool) {
	ok := strings.HasPrefix(raw, "+") && len(digits) >= 8 && len(digits) <= 15
	return "+" + digits, ok
}

func chileMobileNine(_, digits string) (string, bool) {
	return "+56" + digits, len(digits) == 9 && digits[0] == '9'
}

func chileMobileEleven(_, digits string) (string, bool) {
	return "+" + digits, len(digits) == 11 && strings.HasPrefix(digits, "569")
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MaskPhone oculta el centro de un teléfono para logs y bitácora (+569****0000).
func MaskPhone(e164 string) string {
	if len(e164) < 9 {
		return "****"
	}
	return e164[:4] + "****" + e164[len(e164)-4:]
}
