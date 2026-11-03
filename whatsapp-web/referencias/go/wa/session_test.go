package wa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// TestWAW_C10_SesionPrivada: la sesión es SQLite con permisos 600 en un directorio 700 y arranca sin vincular.
func TestWAW_C10_SesionPrivada(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datos", "session.db")
	cli, err := OpenClient(context.Background(), path, "Empresa Demo Vigía")
	must(t, err)
	file, err := os.Stat(path)
	must(t, err)
	dir, err := os.Stat(filepath.Dir(path))
	must(t, err)
	check(t, file.Mode().Perm() == 0o600 && dir.Mode().Perm() == 0o700, "permisos %v %v", file.Mode().Perm(), dir.Mode().Perm())
	check(t, !IsEnrolled(cli) && cli.ManualHistorySyncDownload, "sin vincular y con historial a mano")
}

// TestWAW_C06_ErrorTerminalNoReinicia: sesión revocada o reemplazada sale con el código que el servicio no reinicia.
func TestWAW_C06_ErrorTerminalNoReinicia(t *testing.T) {
	check(t, ExitCode(fmt.Errorf("x: %w", ErrSessionRevoked)) == ExitTerminal, "revocada")
	check(t, ExitCode(ErrStreamReplaced) == ExitTerminal, "reemplazada")
	check(t, ExitCode(errors.New("sin red")) == 1 && ExitCode(nil) == 0, "las demás fallas se reintentan")
}

// TestWAW_V01_YaVinculadoNoRevincula: con la sesión vinculada no se vuelve a vincular (ni por código ni por QR).
func TestWAW_V01_YaVinculadoNoRevincula(t *testing.T) {
	id := ownPN
	cli := &whatsmeow.Client{Store: &store.Device{ID: &id}}
	check(t, errors.Is(Pair(context.Background(), cli, "+56900000000", func(string) {}), ErrAlreadyEnrolled), "código")
	check(t, errors.Is(PairQR(context.Background(), cli, func(string) {}), ErrAlreadyEnrolled), "QR")
}

// TestWAW_V02_TelefonoInvalidoNoConecta: un número inválido se rechaza antes de abrir la conexión.
func TestWAW_V02_TelefonoInvalidoNoConecta(t *testing.T) {
	err := Pair(context.Background(), &whatsmeow.Client{Store: &store.Device{}}, "123", func(string) {})
	check(t, err != nil && !errors.Is(err, ErrAlreadyEnrolled), "número inválido: %v", err)
}

// TestWAW_V03_FinDeLaVinculacion: los eventos finales del canal se traducen a un resultado claro.
func TestWAW_V03_FinDeLaVinculacion(t *testing.T) {
	done, err := pairingOutcome(whatsmeow.QRChannelSuccess)
	check(t, done && err == nil, "éxito")
	done, err = pairingOutcome(whatsmeow.QRChannelTimeout)
	check(t, done && err != nil, "tiempo agotado")
	done, _ = pairingOutcome(whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode})
	check(t, !done, "un código no termina la vinculación")
}

// TestWAW_V04_CadaQRSeMuestra: cada código nuevo del canal se muestra y el éxito termina sin error.
func TestWAW_V04_CadaQRSeMuestra(t *testing.T) {
	ch := make(chan whatsmeow.QRChannelItem, 3)
	ch <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "a"}
	ch <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "b"}
	ch <- whatsmeow.QRChannelSuccess
	close(ch)
	var shown []string
	must(t, awaitQR(ch, func(c string) { shown = append(shown, c) }))
	check(t, len(shown) == 2, "QR mostrados: %v", shown)
}

// TestWAW_C07_ChatsSoportados: solo chats 1:1 (número o LID) y grupos; estados, difusiones y canales se ignoran.
func TestWAW_C07_ChatsSoportados(t *testing.T) {
	for server, want := range map[string]bool{
		types.DefaultUserServer: true, types.HiddenUserServer: true, types.GroupServer: true,
		types.BroadcastServer: false, types.NewsletterServer: false,
	} {
		check(t, supportedChat[server] == want, "servidor %s soportado=%v", server, supportedChat[server])
	}
	pn := types.NewJID("56900000001", types.DefaultUserServer)
	received := types.MessageInfo{MessageSource: types.MessageSource{SenderAlt: pn, RecipientAlt: ownPN}}
	sent := types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: true, SenderAlt: ownPN, RecipientAlt: pn}}
	check(t, altAddress(received) == pn && altAddress(sent) == pn, "el número del otro sale de SenderAlt o RecipientAlt")
}

// TestWAW_C04_AdjuntoSoloDescrito: el adjunto se describe (nombre, tipo, tamaño) sin descargarlo.
func TestWAW_C04_AdjuntoSoloDescrito(t *testing.T) {
	downloads := 0
	factory := func(whatsmeow.DownloadableMessage) ports.AttachmentFetcher {
		return func(context.Context) ([]byte, error) { downloads++; return nil, nil }
	}
	doc := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("f.pdf"), Caption: proto.String("factura"),
		FileLength: proto.Uint64(10)}}
	ref, fetch := attachmentOf(factory, doc)
	check(t, ref != nil && ref.Bytes == 10 && fetch != nil && downloads == 0, "descrito sin descargar: %+v", ref)
	check(t, messageText(doc) == "factura", "el pie es el texto")
	none, _ := attachmentOf(factory, &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}})
	check(t, none == nil, "un sticker no es adjunto")
}

// TestWAW_C05_HistorialGuardadoSinLeer: el aviso se guarda (idempotente) y se borra solo cuando se procesó bien.
func TestWAW_C05_HistorialGuardadoSinLeer(t *testing.T) {
	v, err := NewHistoryVault(filepath.Join(t.TempDir(), "historial"))
	must(t, err)
	notif := &waE2E.HistorySyncNotification{DirectPath: proto.String("/v/1")}
	must(t, v.Save(notif))
	must(t, v.Save(notif))
	check(t, v.Pending() == 1, "idempotente por contenido: %d", v.Pending())
	failing := func(context.Context, *waE2E.HistorySyncNotification) error { return errors.New("sin red") }
	check(t, v.Drain(context.Background(), failing) != nil && v.Pending() == 1, "si falla, queda pendiente")
	must(t, v.Drain(context.Background(), func(context.Context, *waE2E.HistorySyncNotification) error { return nil }))
	check(t, v.Pending() == 0, "procesado y borrado")
}

// TestWAW_C11_EsperaCreciente: la espera entre reintentos se duplica hasta un minuto.
func TestWAW_C11_EsperaCreciente(t *testing.T) {
	check(t, nextBackoff(time.Second) == 2*time.Second, "duplica")
	check(t, nextBackoff(45*time.Second) == time.Minute, "tope de un minuto")
}

// TestWAW_R02_AdjuntoExpirado: un adjunto que el servidor ya no tiene se pide al teléfono; otros errores no.
func TestWAW_R02_AdjuntoExpirado(t *testing.T) {
	check(t, expiredMedia(fmt.Errorf("x: %w", whatsmeow.ErrMediaDownloadFailedWith404)), "404 expiró")
	check(t, expiredMedia(whatsmeow.ErrNoURLPresent), "sin URL")
	check(t, !expiredMedia(errors.New("sin red")) && !expiredMedia(nil), "otros errores no")
}
