package wa

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// TestWAW_T02_UnSoloEnvioAlJIDValidado: third_party.go tiene UN envío y su destino es el JID que valida recipientJID.
func TestWAW_T02_UnSoloEnvioAlJIDValidado(t *testing.T) {
	src, err := os.ReadFile("third_party.go")
	must(t, err)
	sends := regexp.MustCompile(`\.SendMessage\(`).FindAll(src, -1)
	toJID := regexp.MustCompile(`t\.cli\.SendMessage\(ctx, jid, `).FindAll(src, -1)
	check(t, len(sends) == 1 && len(toJID) == 1, "un solo envío, a 'jid': %d de %d", len(toJID), len(sends))
	check(t, regexp.MustCompile(`jid, err := recipientJID\(to\)`).Match(src), "'jid' debe salir de recipientJID")
	rest := regexp.MustCompile(`\.(SendMessage|Upload)\(`).ReplaceAll(src, nil)
	check(t, forbidden.Find(rest) == nil, "third_party.go usa otra API de escritura: %s", forbidden.Find(rest))
}

// TestWAW_T03_SoloElCasoDeUsoLlamaAlEnvio: solo el caso de uso (shares.go) llama al emisor a terceros; el adaptador lo
// implementa y el puerto lo declara, pero ninguno de los dos lo llama.
func TestWAW_T03_SoloElCasoDeUsoLlamaAlEnvio(t *testing.T) {
	call := regexp.MustCompile(`\.SendDocument\(`)
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || !isProductionGo(path, info) {
			return err
		}
		src, rerr := os.ReadFile(path)
		check(t, rerr == nil && !call.Match(src), "%s llama al envío a terceros fuera de shares.go", path)
		return rerr
	})
	must(t, err)
}

// isProductionGo: archivo .go que no es prueba ni el caso de uso, único que llama al emisor a terceros.
func isProductionGo(path string, info os.FileInfo) bool {
	allowed := map[string]bool{"shares.go": true}
	return !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !allowed[filepath.Base(path)]
}

// TestWAW_T01_SinGruposNiOtroNumero: el destino es un usuario individual cuyo JID calza con su teléfono.
func TestWAW_T01_SinGruposNiOtroNumero(t *testing.T) {
	good := domain.Recipient{E164: "+56900000000", JID: "56900000000@s.whatsapp.net"}
	jid, err := recipientJID(good)
	check(t, err == nil && jid.User == "56900000000" && jid.Server == types.DefaultUserServer, "válido: %v %v", jid, err)
	for name, r := range map[string]domain.Recipient{
		"grupo":            {E164: "+56900000000", JID: "120363000000000000@g.us"},
		"otro número":      {E164: "+56900000000", JID: "56900000001@s.whatsapp.net"},
		"sin teléfono":     {E164: "", JID: "56900000000@s.whatsapp.net"},
		"JID inválido":     {E164: "+56900000000", JID: "%%%"},
		"canal/newsletter": {E164: "+56900000000", JID: "56900000000@newsletter"},
		"difusión":         {E164: "+56900000000", JID: "status@broadcast"},
	} {
		_, err := recipientJID(r)
		check(t, err != nil, "%s debe rechazarse", name)
	}
}
