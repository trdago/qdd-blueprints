package wa

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waMmsRetry"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// phoneRequests concentra las ÚNICAS peticiones que el vigía hace al teléfono propio:
// re-subir un adjunto expirado y reenviar un tramo de historial de un chat ya vigilado.
// Ninguna llega a otra persona. TestPhoneRequestsOnlyOwnPhone verifica que no haya otros envíos.
type phoneRequests struct {
	cli     *whatsmeow.Client
	timeout time.Duration
	mu      sync.Mutex
	waiting map[types.MessageID]chan *events.MediaRetry
}

func newPhoneRequests(cli *whatsmeow.Client) *phoneRequests {
	return &phoneRequests{cli: cli, timeout: 90 * time.Second, waiting: map[types.MessageID]chan *events.MediaRetry{}}
}

// errPhoneTimeout: el teléfono no respondió (apagado, sin conexión o sin el archivo).
var errPhoneTimeout = errors.New("el teléfono no respondió la re-subida a tiempo")

// expiredMedia: el archivo ya no está en el servidor de WhatsApp, pero puede seguir en el teléfono.
func expiredMedia(err error) bool {
	return errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith403) ||
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404) ||
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410) ||
		errors.Is(err, whatsmeow.ErrNoURLPresent) ||
		errors.Is(err, whatsmeow.ErrInvalidMediaEncSHA256) // el servidor ya no tiene el archivo original
}

// reupload pide al teléfono propio volver a subir el adjunto y lo descarga desde la ruta nueva.
func (p *phoneRequests) reupload(ctx context.Context, info types.MessageInfo, msg whatsmeow.DownloadableMessage) ([]byte, error) {
	ch := p.expect(info.ID)
	defer p.forget(info.ID)
	if err := p.cli.SendMediaRetryReceipt(ctx, &info, msg.GetMediaKey()); err != nil {
		return nil, fmt.Errorf("pedir re-subida: %w", err)
	}
	evt, err := p.await(ctx, ch)
	if err != nil {
		return nil, err
	}
	return p.downloadRetried(ctx, evt, msg)
}

func (p *phoneRequests) downloadRetried(ctx context.Context, evt *events.MediaRetry, msg whatsmeow.DownloadableMessage) ([]byte, error) {
	retry, err := whatsmeow.DecryptMediaRetryNotification(evt, msg.GetMediaKey())
	if err != nil {
		return nil, err
	}
	if retry.GetResult() != waMmsRetry.MediaRetryNotification_SUCCESS {
		return nil, fmt.Errorf("el teléfono no pudo re-subir el archivo: %s", retry.GetResult())
	}
	return p.cli.DownloadMediaWithPath(ctx, retry.GetDirectPath(), msg.GetFileEncSHA256(), msg.GetFileSHA256(),
		msg.GetMediaKey(), whatsmeow.GetMediaType(msg), "", false)
}

func (p *phoneRequests) expect(id types.MessageID) chan *events.MediaRetry {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan *events.MediaRetry, 1)
	p.waiting[id] = ch
	return ch
}

func (p *phoneRequests) forget(id types.MessageID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.waiting, id)
}

// deliver entrega la respuesta del teléfono a quien la espera (si alguien la espera).
func (p *phoneRequests) deliver(evt *events.MediaRetry) {
	p.mu.Lock()
	ch, ok := p.waiting[evt.MessageID]
	p.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- evt:
	default:
	}
}

func (p *phoneRequests) await(ctx context.Context, ch chan *events.MediaRetry) (*events.MediaRetry, error) {
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case evt := <-ch:
		return evt, nil
	case <-timer.C:
		return nil, errPhoneTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// requestHistory pide al teléfono propio los `count` mensajes anteriores al ancla en ese chat.
// La respuesta llega como un aviso de historial ON_DEMAND y sigue el flujo normal (lista blanca incluida).
func (p *phoneRequests) requestHistory(ctx context.Context, anchor types.MessageInfo, count int) error {
	_, err := p.cli.SendPeerMessage(ctx, p.cli.BuildHistorySyncRequest(&anchor, count))
	return err
}
