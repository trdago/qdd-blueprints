package wa

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// SelfReplier es el ÚNICO punto del vigía que envía mensajes: siempre al chat propio.
// El destino sale de la sesión; no hay parámetro de destinatario. TestSelfReplyOnlyToOwnChat lo verifica.
type SelfReplier struct {
	cli *whatsmeow.Client
}

var (
	_ ports.DocumentReplier = SelfReplier{}
	_ ports.SelfMessenger   = SelfReplier{}
)

// NewSelfReplier crea el emisor hacia el chat propio.
func NewSelfReplier(cli *whatsmeow.Client) SelfReplier {
	return SelfReplier{cli: cli}
}

// errNoOwnNumber: la sesión aún no tiene número propio (no enrolada).
var errNoOwnNumber = errors.New("la sesión de WhatsApp no tiene número propio")

// SendToSelf escribe text en el chat "Mensaje a ti mismo".
func (r SelfReplier) SendToSelf(ctx context.Context, text string) error {
	_, err := r.SendToSelfID(ctx, text)
	return err
}

// SendToSelfID escribe en el chat propio y devuelve el ID del mensaje, para asociar el 👍 a su propuesta.
func (r SelfReplier) SendToSelfID(ctx context.Context, text string) (string, error) {
	self, ok := ownChat(r.cli)
	if !ok {
		return "", errNoOwnNumber
	}
	resp, err := r.cli.SendMessage(ctx, self, &waE2E.Message{Conversation: proto.String(text)})
	return resp.ID, err
}

// SendDocumentToSelf adjunta un documento en el chat "Mensaje a ti mismo" (mismo destino único).
func (r SelfReplier) SendDocumentToSelf(ctx context.Context, doc ports.OutDocument) error {
	self, ok := ownChat(r.cli)
	if !ok {
		return errNoOwnNumber
	}
	up, err := r.cli.Upload(ctx, doc.Data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	_, err = r.cli.SendMessage(ctx, self, documentMessage(up, doc))
	return err
}

func documentMessage(up whatsmeow.UploadResponse, doc ports.OutDocument) *waE2E.Message {
	return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(doc.Data))),
		Mimetype:      proto.String(doc.Mime),
		FileName:      proto.String(doc.Filename),
		Caption:       optionalString(doc.Caption),
	}}
}

// ownChat es el JID del propio número, sin dispositivo.
func ownChat(cli *whatsmeow.Client) (types.JID, bool) {
	if cli.Store.ID == nil {
		return types.EmptyJID, false
	}
	return cli.Store.ID.ToNonAD(), true
}

// optionalString devuelve nil si el texto está vacío (el campo no se envía).
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return proto.String(s)
}
