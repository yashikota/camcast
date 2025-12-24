package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// SignalMessage represents a WebRTC signaling message
type SignalMessage struct {
	Type      string          `json:"type"`
	SDP       string          `json:"sdp,omitempty"`
	Candidate json.RawMessage `json:"candidate,omitempty"`
}

// SignalingServer handles WebSocket connections for WebRTC signaling
type SignalingServer struct {
	mu      sync.RWMutex
	conn    *websocket.Conn
	onOffer func(sdp string) (string, error)
	onICE   func(candidate json.RawMessage) error
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer() *SignalingServer {
	return &SignalingServer{}
}

// SetOfferHandler sets the handler for incoming SDP offers
func (s *SignalingServer) SetOfferHandler(handler func(sdp string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOffer = handler
}

// SetICEHandler sets the handler for incoming ICE candidates
func (s *SignalingServer) SetICEHandler(handler func(candidate json.RawMessage) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onICE = handler
}

// SendICECandidate sends an ICE candidate to the connected client
func (s *SignalingServer) SendICECandidate(candidate json.RawMessage) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return nil
	}

	msg := SignalMessage{
		Type:      "candidate",
		Candidate: candidate,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.Write(context.Background(), websocket.MessageText, data)
}

// HandleWebSocket handles WebSocket connections
func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("Failed to accept WebSocket: %v", err)
		return
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	log.Println("WebSocket client connected")

	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		var msg SignalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		switch msg.Type {
		case "offer":
			s.mu.RLock()
			handler := s.onOffer
			s.mu.RUnlock()

			if handler != nil {
				answer, err := handler(msg.SDP)
				if err != nil {
					log.Printf("Failed to handle offer: %v", err)
					continue
				}

				response := SignalMessage{
					Type: "answer",
					SDP:  answer,
				}
				responseData, _ := json.Marshal(response)
				if err := conn.Write(context.Background(), websocket.MessageText, responseData); err != nil {
					log.Printf("Failed to send answer: %v", err)
				}
			}

		case "candidate":
			s.mu.RLock()
			handler := s.onICE
			s.mu.RUnlock()

			if handler != nil {
				if err := handler(msg.Candidate); err != nil {
					log.Printf("Failed to handle ICE candidate: %v", err)
				}
			}
		}
	}
}
