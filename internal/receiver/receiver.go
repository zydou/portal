package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/SpatiumPortae/portal/internal/conn"
	"github.com/SpatiumPortae/portal/internal/password"
	"github.com/SpatiumPortae/portal/protocol/rendezvous"
	"github.com/SpatiumPortae/portal/protocol/transfer"
	"github.com/schollz/pake/v3"
	"nhooyr.io/websocket"
)

// ConnectRendezvous makes the initial connection to the rendezvous server.
func ConnectRendezvous(addr string) (conn.Rendezvous, error) {
	ws, _, err := websocket.Dial(context.Background(), fmt.Sprintf("ws://%s/establish-receiver", addr), nil)
	if err != nil {
		return conn.Rendezvous{}, err
	}
	return conn.Rendezvous{Conn: &conn.WS{Conn: ws}}, nil
}

// SecureConnection performs the cryptographic handshake to resolve a secure connection.
func SecureConnection(ctx context.Context, rc conn.Rendezvous, pass string) (conn.Transfer, error) {
	// Convenience for messaging in this function.
	type pakeMsg struct {
		pake *pake.Pake
		err  error
	}
	pakeCh := make(chan pakeMsg)

	// Init pake curve in background.
	go func() {
		p, err := pake.InitCurve([]byte(pass), 1, "p256")
		pakeCh <- pakeMsg{pake: p, err: err}
	}()

	if err := rc.WriteMsg(ctx, rendezvous.Msg{
		Type: rendezvous.ReceiverToRendezvousEstablish,
		Payload: rendezvous.Payload{
			Password: password.Hashed(pass),
		},
	}); err != nil {
		return conn.Transfer{}, err
	}

	msg, err := rc.ReadMsg(ctx, rendezvous.RendezvousToReceiverPAKE)
	if err != nil {
		return conn.Transfer{}, err
	}
	pm := <-pakeCh

	if pm.err != nil {
		return conn.Transfer{}, err
	}
	p := pm.pake

	err = p.Update(msg.Payload.Bytes)
	if err != nil {
		return conn.Transfer{}, err
	}

	if err = rc.WriteMsg(ctx, rendezvous.Msg{
		Type: rendezvous.ReceiverToRendezvousPAKE,
		Payload: rendezvous.Payload{
			Bytes: p.Bytes(),
		},
	}); err != nil {
		return conn.Transfer{}, err
	}

	session, err := p.SessionKey()
	if err != nil {
		return conn.Transfer{}, err
	}

	msg, err = rc.ReadMsg(ctx, rendezvous.RendezvousToReceiverSalt)
	if err != nil {
		return conn.Transfer{}, err
	}

	return conn.TransferFromSession(rc.Conn, session, msg.Payload.Salt), nil
}

// Receive receives the payload over the relayed transfer connection and writes it into the provided destination.
// The msgs channel communicates information about the receiving process while running.
func Receive(ctx context.Context, tc conn.Transfer, dst io.Writer, msgs ...chan interface{}) error {
	if err := tc.WriteMsg(ctx, transfer.Msg{Type: transfer.ReceiverHandshake}); err != nil {
		return err
	}

	msg, err := tc.ReadMsg(ctx, transfer.SenderHandshake)
	if err != nil {
		return err
	}

	if len(msgs) > 0 {
		msgs[0] <- msg.Payload.PayloadSize
	}

	if err := tc.WriteMsg(ctx, transfer.Msg{Type: transfer.ReceiverRequestPayload}); err != nil {
		return err
	}
	if err := receivePayload(ctx, tc, dst, msgs...); err != nil {
		return err
	}

	// Closing handshake.
	if err := tc.WriteMsg(ctx, transfer.Msg{Type: transfer.ReceiverPayloadAck}); err != nil {
		return err
	}
	if _, err := tc.ReadMsg(ctx, transfer.SenderClosing); err != nil {
		return err
	}
	if err := tc.WriteMsg(ctx, transfer.Msg{Type: transfer.ReceiverClosingAck}); err != nil {
		return err
	}

	// Tell rendezvous to close the connection.
	rc := conn.Rendezvous{Conn: tc.Conn}
	return rc.WriteMsg(ctx, rendezvous.Msg{Type: rendezvous.ReceiverToRendezvousClose})
}

// receivePayload receives the payload over the provided connection and writes it into the desired location.
func receivePayload(ctx context.Context, tc conn.Transfer, dst io.Writer, msgs ...chan interface{}) error {
	writtenBytes := 0
	for {
		b, err := tc.ReadRaw(ctx)
		if err != nil {
			return err
		}
		msg := transfer.Msg{}
		err = json.Unmarshal(b, &msg)
		if err != nil {
			n, err := dst.Write(b)
			if err != nil {
				return err
			}
			writtenBytes += n
			if len(msgs) > 0 {
				msgs[0] <- writtenBytes
			}
		} else {
			if msg.Type != transfer.SenderPayloadSent {
				return transfer.Error{Expected: []transfer.MsgType{transfer.SenderPayloadSent}, Got: msg.Type}
			}
			break
		}
	}
	return nil
}
