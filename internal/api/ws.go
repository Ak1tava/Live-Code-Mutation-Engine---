package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"lcme/internal/audit"
)

type peer struct {
	role   audit.Role
	send   chan []byte
	cancel context.CancelFunc
}

type clientMessage struct {
	Type     string  `json:"type"`
	Path     string  `json:"path"`
	Content  *string `json:"content"`
	Revision *int    `json:"revision"`
	Kind     string  `json:"kind"`
	Size     int     `json:"size"`
}

type serverMessage struct {
	Type     string  `json:"type"`
	View     any     `json:"view,omitempty"`
	Path     string  `json:"path,omitempty"`
	Content  *string `json:"content,omitempty"`
	Revision *int    `json:"revision,omitempty"`
	Message  string  `json:"message,omitempty"`
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request, id string, role audit.Role) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	// JSON can encode one byte of file content as six bytes of escapes.
	conn.SetReadLimit(6*audit.MaxFileSize + (32 << 10))
	ctx, cancel := context.WithCancel(r.Context())
	defer conn.CloseNow()
	defer cancel()

	rt := s.runtime(id)
	p := &peer{role: role, send: make(chan []byte, 8), cancel: cancel}
	rt.mu.Lock()
	rt.peers[p] = struct{}{}
	s.d.Store.View(id, func(v *audit.Session) {
		p.enqueue(encode(serverMessage{Type: "snapshot", View: s.newViews(v, rt, role == audit.RoleAuditor)}))
	})
	rt.mu.Unlock()
	defer func() {
		rt.mu.Lock()
		delete(rt.peers, p)
		rt.mu.Unlock()
	}()
	go p.write(ctx, conn)

	for {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if role != audit.RoleStudent {
			p.enqueue(encode(serverMessage{Type: "error", Message: "У аудитора нет права отправлять изменения."}))
			continue
		}
		var msg clientMessage
		if kind != websocket.MessageText || json.Unmarshal(data, &msg) != nil {
			p.enqueue(encode(serverMessage{Type: "error", Message: "Неверный формат сообщения."}))
			continue
		}
		switch msg.Type {
		case "file":
			s.editFile(id, rt, p, msg)
		case "event":
			err := s.change(id, func(v *audit.Session) error {
				now := s.d.Now()
				if err := checkOpen(v, now); err != nil {
					return err
				}
				return v.AddEvent(msg.Kind, msg.Size, now)
			})
			if err != nil && !errors.Is(err, audit.ErrBadState) && !errors.Is(err, audit.ErrExpired) {
				_, message := sessionError(err)
				p.enqueue(encode(serverMessage{Type: "error", Message: message}))
			}
		default:
			p.enqueue(encode(serverMessage{Type: "error", Message: "Неизвестный тип сообщения."}))
		}
	}
}

func (s *Server) editFile(id string, rt *sessionRuntime, p *peer, msg clientMessage) {
	if msg.Content == nil {
		p.enqueue(encode(serverMessage{Type: "error", Path: msg.Path, Revision: msg.Revision, Message: "Передайте содержимое файла."}))
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	err := s.d.Store.Update(id, func(v *audit.Session) error {
		now := s.d.Now()
		if err := checkOpen(v, now); err != nil {
			return err
		}
		return v.UpdateFile(msg.Path, []byte(*msg.Content), now)
	})
	if err != nil {
		_, message := sessionError(err)
		p.enqueue(encode(serverMessage{Type: "error", Path: msg.Path, Revision: msg.Revision, Message: message}))
		return
	}
	p.enqueue(encode(serverMessage{Type: "fileAck", Path: msg.Path, Content: msg.Content, Revision: msg.Revision}))
	file := encode(serverMessage{Type: "file", Path: msg.Path, Content: msg.Content})
	for target := range rt.peers {
		if target.role == audit.RoleAuditor {
			target.enqueue(file)
		}
	}
	s.broadcastLocked(id, rt)
}

func encode(msg serverMessage) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("api: cannot encode WebSocket message: %v", err)
	}
	return data
}

func (s *Server) broadcastLocked(id string, rt *sessionRuntime) {
	if len(rt.peers) == 0 {
		return
	}
	var student, auditor any
	s.d.Store.View(id, func(v *audit.Session) {
		for p := range rt.peers {
			if p.role == audit.RoleStudent && student == nil {
				student = s.newViews(v, rt, false)
			}
			if p.role == audit.RoleAuditor && auditor == nil {
				auditor = s.newViews(v, rt, true)
			}
		}
	})
	var studentMsg, auditorMsg []byte
	if student != nil {
		studentMsg = encode(serverMessage{Type: "snapshot", View: student})
	}
	if auditor != nil {
		auditorMsg = encode(serverMessage{Type: "snapshot", View: auditor})
	}
	for p := range rt.peers {
		if p.role == audit.RoleStudent {
			p.enqueue(studentMsg)
		} else {
			p.enqueue(auditorMsg)
		}
	}
}

func (s *Server) broadcastErrorLocked(rt *sessionRuntime, message string) {
	data := encode(serverMessage{Type: "error", Message: message})
	for p := range rt.peers {
		p.enqueue(data)
	}
}

func (p *peer) enqueue(data []byte) {
	if len(data) == 0 {
		return
	}
	select {
	case p.send <- data:
	default:
		// The client can reconnect for a current snapshot instead of blocking
		// persistence and the server timer behind a slow network connection.
		p.cancel()
	}
}

func (p *peer) write(ctx context.Context, conn *websocket.Conn) {
	defer p.cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-p.send:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
