package wa

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// ThirdPartySender es el ÚNICO punto que escribe a otras personas. El destino sale de un domain.Recipient ya validado por el caso de uso
// y aprobado por la persona con 👍; aquí se revalida que sea un usuario individual (nunca un grupo ni un canal) y que el JID coincida con el teléfono.
// TestThirdPartyOnlyToValidatedRecipient lo verifica con un análisis de código.
type ThirdPartySender struct {
	cli *whatsmeow.Client
}

var _ ports.ThirdPartySender = ThirdPartySender{}

// NewThirdPartySender crea el emisor hacia terceros.
func NewThirdPartySender(cli *whatsmeow.Client) ThirdPartySender {
	return ThirdPartySender{cli: cli}
}

// Lookup comprueba en WhatsApp que el número exista.
func (t ThirdPartySender) Lookup(ctx context.Context, e164 string) (string, bool, error) {
	resp, err := t.cli.IsOnWhatsApp(ctx, []string{e164})
	if err != nil {
		return "", false, err
	}
	if len(resp) == 0 || !resp[0].IsIn {
		return "", false, nil
	}
	return resp[0].JID.String(), true, nil
}

var errBadRecipient = errors.New("destinatario no válido para un envío a terceros")

// recipientJID valida el destino: un usuario individual cuyo número coincide con el teléfono del destinatario.
func recipientJID(r domain.Recipient) (types.JID, error) {
	jid, err := types.ParseJID(r.JID)
	if err != nil {
		return types.JID{}, fmt.Errorf("%w: %v", errBadRecipient, err)
	}
	digits := strings.TrimPrefix(r.E164, "+")
	if jid.Server != types.DefaultUserServer || jid.User != digits || digits == "" {
		return types.JID{}, errBadRecipient
	}
	return jid, nil
}

// SendDocument sube y envía un documento a un destinatario validado.
func (t ThirdPartySender) SendDocument(ctx context.Context, to domain.Recipient, doc ports.OutDocument) error {
	jid, err := recipientJID(to)
	if err != nil {
		return err
	}
	up, err := t.cli.Upload(ctx, doc.Data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	_, err = t.cli.SendMessage(ctx, jid, documentMessage(up, doc))
	return err
}
