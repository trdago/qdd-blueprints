package wa

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// ErrAlreadyEnrolled indica que la sesión ya tiene un dispositivo vinculado.
var ErrAlreadyEnrolled = errors.New("la sesión ya tiene un dispositivo vinculado; bórrala para volver a vincular")

// Pair vincula el número con un código de 8 caracteres que el usuario ingresa en
// WhatsApp → Dispositivos vinculados → Vincular con número de teléfono.
// showCode recibe el código apenas existe; Pair vuelve cuando el teléfono confirma o expira.
func Pair(ctx context.Context, cli *whatsmeow.Client, phone string, showCode func(string)) error {
	if IsEnrolled(cli) {
		return ErrAlreadyEnrolled
	}
	e164, ok := domain.NormalizePhoneE164(phone)
	if !ok {
		return fmt.Errorf("teléfono inválido %q: usa formato E.164, p. ej. +56900000000", phone)
	}
	qr, err := cli.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("canal de vinculación: %w", err)
	}
	if err := cli.ConnectContext(ctx); err != nil {
		return fmt.Errorf("conectar a WhatsApp: %w", err)
	}
	return awaitPairing(ctx, cli, qr, strings.TrimPrefix(e164, "+"), showCode)
}

func awaitPairing(ctx context.Context, cli *whatsmeow.Client, qr <-chan whatsmeow.QRChannelItem, digits string, showCode func(string)) error {
	requested := false
	for item := range qr {
		if err := maybeRequestCode(ctx, cli, item, &requested, digits, showCode); err != nil {
			return err
		}
		if done, err := pairingOutcome(item); done {
			return err
		}
	}
	return errors.New("la vinculación terminó sin confirmación")
}

// maybeRequestCode pide el código de vinculación una sola vez, con el primer evento "code".
func maybeRequestCode(ctx context.Context, cli *whatsmeow.Client, item whatsmeow.QRChannelItem, requested *bool, digits string, showCode func(string)) error {
	if item.Event != whatsmeow.QRChannelEventCode || *requested {
		return nil
	}
	*requested = true
	return requestCode(ctx, cli, digits, showCode)
}

func requestCode(ctx context.Context, cli *whatsmeow.Client, digits string, showCode func(string)) error {
	code, err := cli.PairPhone(ctx, digits, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		return fmt.Errorf("pedir código de vinculación: %w", err)
	}
	showCode(code)
	return nil
}

// pairingOutcome traduce los eventos finales del canal de vinculación.
func pairingOutcome(item whatsmeow.QRChannelItem) (bool, error) {
	switch item.Event {
	case whatsmeow.QRChannelSuccess.Event:
		return true, nil
	case whatsmeow.QRChannelTimeout.Event:
		return true, errors.New("se agotó el tiempo para ingresar el código (unos 2 minutos)")
	case whatsmeow.QRChannelEventError:
		return true, fmt.Errorf("WhatsApp rechazó la vinculación: %v", item.Error)
	case whatsmeow.QRChannelErrUnexpectedEvent.Event:
		return true, errors.New("estado inesperado durante la vinculación; vuelve a intentarlo")
	}
	return false, nil
}

// PairQR vincula escaneando un QR. onQR recibe cada código nuevo (WhatsApp lo renueva cada ~20 s).
func PairQR(ctx context.Context, cli *whatsmeow.Client, onQR func(string)) error {
	if IsEnrolled(cli) {
		return ErrAlreadyEnrolled
	}
	qr, err := cli.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("canal de vinculación: %w", err)
	}
	if err := cli.ConnectContext(ctx); err != nil {
		return fmt.Errorf("conectar a WhatsApp: %w", err)
	}
	return awaitQR(qr, onQR)
}

func awaitQR(qr <-chan whatsmeow.QRChannelItem, onQR func(string)) error {
	for item := range qr {
		if item.Event == whatsmeow.QRChannelEventCode {
			onQR(item.Code)
			continue
		}
		if done, err := pairingOutcome(item); done {
			return err
		}
	}
	return errors.New("la vinculación terminó sin confirmación")
}
