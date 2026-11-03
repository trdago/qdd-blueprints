package domain

import (
	"strings"
	"testing"
	"time"
)

func TestTelefonoE164(t *testing.T) {
	for raw, want := range map[string]string{"900000000": "+56900000000", "56900000000": "+56900000000",
		"+56 9 0000 0000": "+56900000000", "+5491100000000": "+5491100000000"} {
		got, ok := NormalizePhoneE164(raw)
		check(t, ok && got == want, "%q → %q", raw, got)
	}
	_, ok := NormalizePhoneE164("12345")
	check(t, !ok, "número corto inválido")
	check(t, MaskPhone("+56900000000") == "+569****0000", "enmascarado: %s", MaskPhone("+56900000000"))
}

// WAW-C01 (parte de dominio): solo calza un chat de la lista blanca; un grupo solo si se agregó a mano.
func TestWAW_C01_ListaBlanca(t *testing.T) {
	d := NewDirectory([]Contact{{PhoneE164: "+56900000000", CompanyID: "b"}, {PhoneE164: "900000000", CompanyID: "a"},
		{ChatJID: "1@g.us", CompanyID: "g"}}, time.Now())
	c, ok := d.Match(IncomingMessage{ChatPhone: "+56900000000"})
	check(t, ok && c.CompanyID == "a", "gana la primera empresa por id: %+v", c)
	_, other := d.Match(IncomingMessage{ChatPhone: "+56900000001"})
	check(t, !other, "un número fuera de la lista no calza")
	_, group := d.Match(IncomingMessage{IsGroup: true, ChatJID: "2@g.us"})
	check(t, !group, "un grupo no agregado no calza")
	_, empty := NewDirectory(nil, time.Time{}).Match(IncomingMessage{})
	check(t, !empty && d.Size() == 2, "vacía no acepta nada; tamaño %d", d.Size())
}

// WAW-C08: en modo solo documentos un texto sin adjunto se descarta.
func TestWAW_C08_SoloDocumentos(t *testing.T) {
	c := Contact{Mode: WatchDocsOnly}
	check(t, c.SkipsText(IncomingMessage{Text: "hola"}), "texto sin adjunto se descarta")
	check(t, !c.SkipsText(IncomingMessage{Attachment: &AttachmentRef{}}), "con adjunto pasa")
	check(t, !Contact{Mode: WatchFull}.SkipsText(IncomingMessage{Text: "hola"}), "completo acepta texto")
}

// WAW-C09: los triviales se reconocen sin IA.
func TestWAW_C09_Trivial(t *testing.T) {
	r := DefaultTrivialRules()
	for _, text := range []string{"ok", "Gracias!", "👍👍", "dale", "nos vemos"} {
		check(t, r.IsTrivial(text, false), "%q es trivial", text)
	}
	check(t, !r.IsTrivial("adjunto la factura de octubre", false), "texto largo no es trivial")
	check(t, !r.IsTrivial("ok", true), "con adjunto nunca es trivial")
}

func TestClassificationTextRecorta(t *testing.T) {
	msg := IncomingMessage{Text: "hola", Attachment: &AttachmentRef{FileName: "f.pdf"}}
	got := ClassificationText(msg, strings.Repeat("x", 100), 20)
	check(t, len([]rune(got)) == 20 && strings.Contains(got, "Adjunto: f.pdf"), "recorte: %q", got)
}

// WAW-A04 (parte de dominio): solo 👍 aprueba (con tono de piel); quitar la reacción o 👎 no.
func TestWAW_A04_SoloLikeAprueba(t *testing.T) {
	check(t, IsLike("👍") && IsLike("👍🏽"), "👍 aprueba")
	check(t, !IsLike("") && !IsLike("👎") && !IsLike("❤️"), "lo demás no")
}

// WAW-P02 (parte de dominio): lo que escribe el agente lleva marca y se reconoce.
func TestWAW_P02_MarcaDelAgente(t *testing.T) {
	check(t, IsSelfReply(WithReplyMark("hola")) && IsSelfReply("🎙️ Escuché: x"), "marcas")
	check(t, WithReplyMark("🤖 ya") == "🤖 ya", "no duplica la marca")
	check(t, !IsSelfReply("hola"), "texto de la persona")
}

// WAW-P06: solo «<prefijo> <pregunta>»; una palabra que empieza igual no activa.
func TestWAW_P06_SoloConPrefijo(t *testing.T) {
	body, ok := ParseSelfCommand("BOT: ¿cómo va?", "bot")
	check(t, ok && body == "¿cómo va?", "prefijo: %q", body)
	_, word := ParseSelfCommand("botella nueva", "bot")
	check(t, !word, "no confunde una palabra que empieza igual")
	_, bare := ParseSelfCommand("bot", "bot")
	check(t, !bare, "prefijo sin pregunta")
}

// WAW-A06: aviso de avance a lo más uno cada intervalo, el primero un intervalo después de ver la tarea.
func TestWAW_A06_AvisoDeAvance(t *testing.T) {
	seen := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	every := 30 * time.Minute
	check(t, !ProgressDue(seen, nil, seen.Add(10*time.Minute), every), "muy pronto")
	check(t, ProgressDue(seen, nil, seen.Add(every), every), "primer aviso")
	last := seen.Add(every)
	check(t, !ProgressDue(seen, &last, last.Add(time.Minute), every), "no dos seguidos")
}

// WAW-A03 (parte de dominio): una propuesta vencida o ya tomada no se aprueba.
func TestWAW_A03_Vigencia(t *testing.T) {
	now := time.Now()
	a := Action{State: ActionPending, ExpiresAt: now.Add(time.Minute)}
	check(t, a.Approvable(now), "vigente")
	check(t, !a.Approvable(now.Add(2*time.Minute)), "vencida")
	a.State = ActionRunning
	check(t, !a.Approvable(now), "ya tomada")
	check(t, strings.Contains(FormatProposal(Action{Summary: "x", ExpiresAt: now}), "👍"), "la propuesta dice cómo aprobar")
}

// WAW-G01: un grupo solo calza si se agregó a mano.
func TestWAW_G01_GrupoNoAgregado(t *testing.T) {
	d := NewDirectory([]Contact{{ChatJID: "1@g.us", CompanyID: "g"}}, time.Now())
	_, added := d.Match(IncomingMessage{IsGroup: true, ChatJID: "1@g.us"})
	_, other := d.Match(IncomingMessage{IsGroup: true, ChatJID: "2@g.us"})
	check(t, added && !other, "agregado %v, otro %v", added, other)
}
