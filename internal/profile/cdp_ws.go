package profile

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// cdpWSConn implementa um cliente WebSocket minimalista (RFC 6455) usando apenas a biblioteca padrão do Go.
type cdpWSConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialCDP(wsURL string) (*cdpWSConn, error) {
	wsURL = strings.TrimPrefix(wsURL, "ws://")
	parts := strings.SplitN(wsURL, "/", 2)
	host := parts[0]
	path := "/"
	if len(parts) > 1 {
		path = "/" + parts[1]
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	key := make([]byte, 16)
	_, _ = rand.Read(key)
	secKey := base64.StdEncoding.EncodeToString(key)

	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, host, secKey)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}

	return &cdpWSConn{conn: conn, r: r}, nil
}

func (c *cdpWSConn) Send(payload []byte) error {
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	var buf bytes.Buffer
	buf.WriteByte(0x81) // FIN + Opcode 1 (Text)

	length := len(payload)
	maskKey := make([]byte, 4)
	_, _ = rand.Read(maskKey)

	if length <= 125 {
		buf.WriteByte(byte(length) | 0x80)
	} else if length <= 65535 {
		buf.WriteByte(126 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint16(length))
	} else {
		buf.WriteByte(127 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint64(length))
	}

	buf.Write(maskKey)

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}
	buf.Write(masked)

	_, err := c.conn.Write(buf.Bytes())
	return err
}

func (c *cdpWSConn) Read() ([]byte, error) {
	for {
		b0, err := c.r.ReadByte()
		if err != nil {
			return nil, err
		}
		opcode := b0 & 0x0F

		b1, err := c.r.ReadByte()
		if err != nil {
			return nil, err
		}

		length := int(b1 & 0x7F)
		if length == 126 {
			var l uint16
			if err := binary.Read(c.r, binary.BigEndian, &l); err != nil {
				return nil, err
			}
			length = int(l)
		} else if length == 127 {
			var l uint64
			if err := binary.Read(c.r, binary.BigEndian, &l); err != nil {
				return nil, err
			}
			length = int(l)
		}

		isMasked := (b1 & 0x80) != 0
		var maskKey []byte
		if isMasked {
			maskKey = make([]byte, 4)
			if _, err := io.ReadFull(c.r, maskKey); err != nil {
				return nil, err
			}
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(c.r, payload); err != nil {
			return nil, err
		}

		if isMasked {
			for i := 0; i < length; i++ {
				payload[i] ^= maskKey[i%4]
			}
		}

		switch opcode {
		case 0x08: // Connection Close
			return nil, io.EOF
		case 0x09: // Ping -> responde Pong mascarado
			pongMask := make([]byte, 4)
			_, _ = rand.Read(pongMask)
			pong := make([]byte, 2+4+len(payload))
			pong[0] = 0x8A // FIN + Pong
			pong[1] = byte(len(payload)) | 0x80
			copy(pong[2:6], pongMask)
			for i := 0; i < len(payload); i++ {
				pong[6+i] = payload[i] ^ pongMask[i%4]
			}
			_, _ = c.conn.Write(pong)
			continue
		case 0x0A: // Pong
			continue
		default:
			return payload, nil
		}
	}
}

type cdpMessageProbe struct {
	ID int `json:"id"`
}

// ReadForID lê frames do WebSocket descartando notificações assíncronas do DOM
// até encontrar a resposta com o ID esperado ou atingir o deadline.
func (c *cdpWSConn) ReadForID(expectedID int) ([]byte, error) {
	deadline := time.Now().Add(15 * time.Second)
	_ = c.conn.SetDeadline(deadline)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout aguardando resposta CDP id %d", expectedID)
		}

		payload, err := c.Read()
		if err != nil {
			return nil, err
		}

		var probe cdpMessageProbe
		if err := json.Unmarshal(payload, &probe); err == nil && probe.ID == expectedID {
			return payload, nil
		}
		// Eventos assíncronos (ex: DOM.childNodeCountUpdated, Page.frameNavigated com id==0) são ignorados
	}
}

func (c *cdpWSConn) Close() error {
	return c.conn.Close()
}
