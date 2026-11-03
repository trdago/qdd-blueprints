package wa

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// supportedChat: solo chats 1:1 (número o LID) y grupos. Estados, difusiones y canales se ignoran.
var supportedChat = map[string]bool{
	types.DefaultUserServer: true,
	types.HiddenUserServer:  true,
	types.GroupServer:       true,
}

// fetcherFactory crea la descarga de un adjunto (con re-subida desde el teléfono si expiró).
type fetcherFactory func(info types.MessageInfo, msg whatsmeow.DownloadableMessage) ports.AttachmentFetcher

// toIncoming traduce un mensaje de whatsmeow. ok=false si no es un chat soportado o no se
// pudo resolver el teléfono del otro participante (en ese caso no puede estar en la lista blanca).
func toIncoming(ctx context.Context, cli *whatsmeow.Client, evt *events.Message, source domain.Source, newFetch fetcherFactory) (domain.IncomingMessage, ports.AttachmentFetcher, bool) {
	info := evt.Info
	if !supportedChat[info.Chat.Server] || evt.Message == nil {
		return domain.IncomingMessage{}, nil, false
	}
	phone, ok := chatPhone(ctx, cli, info)
	if !ok {
		return domain.IncomingMessage{}, nil, false
	}
	ref, fetch := attachmentOf(func(m whatsmeow.DownloadableMessage) ports.AttachmentFetcher { return newFetch(info, m) }, evt.Message)
	return domain.IncomingMessage{
		ChatJID:    info.Chat.ToNonAD().String(),
		MessageID:  info.ID,
		ChatPhone:  phone,
		IsGroup:    info.IsGroup,
		FromMe:     info.IsFromMe,
		SenderName: info.PushName,
		SentAt:     info.Timestamp,
		Text:       messageText(evt.Message),
		Attachment: ref,
		Source:     source,
	}, fetch, true
}

// chatPhone devuelve el E.164 del otro participante en un chat 1:1 (vacío en grupos).
func chatPhone(ctx context.Context, cli *whatsmeow.Client, info types.MessageInfo) (string, bool) {
	if info.IsGroup {
		return "", true
	}
	pn := phoneJID(ctx, cli, info)
	if pn.Server != types.DefaultUserServer {
		return "", false
	}
	return domain.NormalizePhoneE164("+" + pn.User)
}

// phoneJID resuelve el JID de número cuando el chat llega con LID (identificador de privacidad).
func phoneJID(ctx context.Context, cli *whatsmeow.Client, info types.MessageInfo) types.JID {
	if info.Chat.Server != types.HiddenUserServer {
		return info.Chat
	}
	if alt := altAddress(info); alt.Server == types.DefaultUserServer {
		return alt
	}
	pn, err := cli.Store.LIDs.GetPNForLID(ctx, info.Chat)
	if err != nil {
		return types.EmptyJID
	}
	return pn
}

// altAddress: en mensajes recibidos el número viene en SenderAlt; en los propios, en RecipientAlt.
func altAddress(info types.MessageInfo) types.JID {
	if info.IsFromMe {
		return info.RecipientAlt
	}
	return info.SenderAlt
}

var textGetters = []func(*waE2E.Message) string{
	(*waE2E.Message).GetConversation,
	func(m *waE2E.Message) string { return m.GetExtendedTextMessage().GetText() },
	func(m *waE2E.Message) string { return m.GetDocumentMessage().GetCaption() },
	func(m *waE2E.Message) string { return m.GetImageMessage().GetCaption() },
}

func messageText(m *waE2E.Message) string {
	for _, get := range textGetters {
		if text := get(m); text != "" {
			return text
		}
	}
	return ""
}

// attachmentOf solo describe el adjunto; la descarga ocurre después de pasar la lista blanca.
func attachmentOf(newFetch func(whatsmeow.DownloadableMessage) ports.AttachmentFetcher, m *waE2E.Message) (*domain.AttachmentRef, ports.AttachmentFetcher) {
	if doc := m.GetDocumentMessage(); doc != nil {
		ref := &domain.AttachmentRef{FileName: doc.GetFileName(), Mime: doc.GetMimetype(), Bytes: int64(doc.GetFileLength())}
		return ref, newFetch(doc)
	}
	if img := m.GetImageMessage(); img != nil {
		ref := &domain.AttachmentRef{FileName: "imagen", Mime: img.GetMimetype(), Bytes: int64(img.GetFileLength())}
		return ref, newFetch(img)
	}
	return nil, nil
}
