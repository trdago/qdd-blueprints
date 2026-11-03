package wa

import (
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

var (
	ownPN   = types.JID{User: "56900000000", Server: types.DefaultUserServer, Device: 7}
	ownLID  = types.JID{User: "1234567890", Server: types.HiddenUserServer}
	someone = types.NewJID("56900000001", types.DefaultUserServer)
)

func info(chat types.JID, fromMe, group bool, senderDevice uint16) types.MessageInfo {
	return types.MessageInfo{MessageSource: types.MessageSource{
		Chat: chat, IsFromMe: fromMe, IsGroup: group,
		Sender: types.JID{User: ownPN.User, Server: types.DefaultUserServer, Device: senderDevice},
	}}
}

// TestWAW_P01_SoloElChatPropio: solo el chat con uno mismo (por número o LID), escrito por la persona, va al agente.
func TestWAW_P01_SoloElChatPropio(t *testing.T) {
	cases := map[string]struct {
		info types.MessageInfo
		want bool
	}{
		"chat propio por número":  {info(ownPN.ToNonAD(), true, false, 0), true},
		"chat propio por LID":     {info(ownLID, true, false, 0), true},
		"mensaje mío a otro":      {info(someone, true, false, 0), false},
		"mensaje de otra persona": {info(someone, false, false, 0), false},
		"grupo":                   {info(types.NewJID("1", types.GroupServer), true, true, 0), false},
		"otro LID":                {info(types.JID{User: "999", Server: types.HiddenUserServer}, true, false, 0), false},
	}
	for name, tc := range cases {
		check(t, isSelfChat(tc.info, ownPN, ownLID) == tc.want, "%s: got %v", name, !tc.want)
	}
}

// TestWAW_P02_HistorialDispositivoYMarcaNoDisparan: ni el historial, ni lo enviado por este dispositivo, ni lo que lleva la
// marca del agente disparan una respuesta (sin bucles).
func TestWAW_P02_HistorialDispositivoYMarcaNoDisparan(t *testing.T) {
	fromPhone := info(ownPN.ToNonAD(), true, false, 0)
	fromThisDevice := info(ownPN.ToNonAD(), true, false, ownPN.Device)
	check(t, !ignoredSelf(fromPhone, ownPN, "hola", domain.SourceRealtime), "pregunta desde el teléfono se atiende")
	check(t, ignoredSelf(fromPhone, ownPN, "hola", domain.SourceHistory), "el historial nunca dispara respuestas")
	check(t, ignoredSelf(fromThisDevice, ownPN, "hola", domain.SourceRealtime), "lo enviado por este dispositivo se ignora")
	check(t, ignoredSelf(fromPhone, ownPN, "🤖 respuesta", domain.SourceRealtime), "las respuestas del agente se ignoran")
}

// TestWAW_A04_ReaccionLeida: la reacción trae el id del mensaje reaccionado; quitarla llega con emoji vacío.
func TestWAW_A04_ReaccionLeida(t *testing.T) {
	like := &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("WA1")}, Text: proto.String("👍")}}
	got, ok := reactionOf(like)
	check(t, ok && got.target == "WA1" && got.emoji == "👍", "reacción = %+v", got)
	removed := &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("WA1")}, Text: proto.String("")}}
	again, ok := reactionOf(removed)
	check(t, ok && again.emoji == "", "quitar la reacción: %+v", again)
	_, plain := reactionOf(&waE2E.Message{Conversation: proto.String("sí")})
	check(t, !plain, "un texto no es una reacción")
	check(t, selfJobOf(&events.Message{Message: like}).reaction != nil, "la reacción va al canal como reacción")
}

// TestWAW_P04_SinSesionNoHayDestino: sin número propio en la sesión no hay a dónde enviar.
func TestWAW_P04_SinSesionNoHayDestino(t *testing.T) {
	_, ok := ownChat(&whatsmeow.Client{Store: &store.Device{}})
	check(t, !ok, "sin sesión vinculada no hay chat propio")
	id := ownPN
	self, ok := ownChat(&whatsmeow.Client{Store: &store.Device{ID: &id}})
	check(t, ok && self.Device == 0 && self.User == ownPN.User, "el chat propio es el número sin dispositivo: %v", self)
}
