package wa

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// forbidden son las APIs de whatsmeow que cambian algo visible en WhatsApp. Solo tres archivos pueden usar alguna, y
// cada uno tiene su prueba que dice cuál y hacia dónde.
var forbidden = regexp.MustCompile(`\.(SendMessage|SendPeerMessage|MarkRead|MarkNotDirty|SendPresence|SendChatPresence|` +
	`SubscribePresence|RevokeMessage|BuildRevoke|BuildEdit|BuildReaction|SendMediaRetryReceipt|SetStatusMessage|` +
	`SetGroup\w*|JoinGroup\w*|LeaveGroup|UpdateGroupParticipants|SendAppState|SetDisappearingTimer|` +
	`SetPrivacySetting|Logout|UpdateBlocklist|SendFBMessage|Upload)\(`)

// exempt: las pruebas guardianas y los únicos emisores permitidos (verificados aparte).
var exempt = map[string]bool{"readonly_test.go": true, "self_test.go": true, "third_party_test.go": true,
	"self_reply.go": true, "third_party.go": true, "phone_requests.go": true}

// TestWAW_C02_SinAPIsDeEscritura recorre todo el módulo y falla si aparece una API de escritura fuera de los emisores.
func TestWAW_C02_SinAPIsDeEscritura(t *testing.T) {
	must(t, filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || !scanned(path, info) {
			return err
		}
		src, rerr := os.ReadFile(path)
		check(t, rerr == nil && forbidden.Find(src) == nil, "%s usa una API de escritura de WhatsApp: %s", path, forbidden.Find(src))
		return rerr
	}))
}

// scanned: todo archivo .go del módulo salvo los emisores permitidos y sus pruebas guardianas.
func scanned(path string, info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(path, ".go") && !exempt[filepath.Base(path)]
}

// TestWAW_R01_PeticionesSoloAlTelefonoPropio: phone_requests.go solo pide re-subir un adjunto y un tramo de historial
// al teléfono propio; nada más.
func TestWAW_R01_PeticionesSoloAlTelefonoPropio(t *testing.T) {
	src, err := os.ReadFile("phone_requests.go")
	must(t, err)
	allowed := regexp.MustCompile(`\.(SendMediaRetryReceipt|SendPeerMessage)\(`)
	check(t, len(allowed.FindAll(src, -1)) == 2, "solo una re-subida y una petición de historial")
	check(t, regexp.MustCompile(`SendPeerMessage\(ctx, p\.cli\.BuildHistorySyncRequest\(`).Match(src), "SendPeerMessage solo para pedir historial")
	rest := allowed.ReplaceAll(src, nil)
	check(t, forbidden.Find(rest) == nil, "phone_requests.go usa otra API de escritura: %s", forbidden.Find(rest))
}

// TestWAW_P03_SoloAlChatPropio: self_reply.go envía texto y documentos SIEMPRE al JID propio de la sesión; la única subida
// es la del documento propio.
func TestWAW_P03_SoloAlChatPropio(t *testing.T) {
	src, err := os.ReadFile("self_reply.go")
	must(t, err)
	sends := regexp.MustCompile(`\.SendMessage\(`).FindAll(src, -1)
	selfSends := regexp.MustCompile(`r\.cli\.SendMessage\(ctx, self, `).FindAll(src, -1)
	check(t, len(sends) == 2 && len(selfSends) == len(sends), "texto y documento, ambos a 'self': %d de %d", len(selfSends), len(sends))
	check(t, regexp.MustCompile(`self, ok := ownChat\(r\.cli\)`).Match(src), "'self' debe salir de ownChat (la sesión)")
	uploads := regexp.MustCompile(`\.Upload\(`).FindAll(src, -1)
	check(t, len(uploads) == 1, "una sola subida: %d", len(uploads))
	rest := regexp.MustCompile(`\.(SendMessage|Upload)\(`).ReplaceAll(src, nil)
	check(t, forbidden.Find(rest) == nil, "self_reply.go usa otra API de escritura: %s", forbidden.Find(rest))
}
