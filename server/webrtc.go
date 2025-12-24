package server

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// RTPHandler is a callback for handling RTP packets
type RTPHandler func(track *webrtc.TrackRemote, packet *rtp.Packet)

// TrackInfo contains information about a received track
type TrackInfo struct {
	Kind        webrtc.RTPCodecType
	PayloadType uint8
	MimeType    string
}

// TrackHandler is a callback for when a new track is received
type TrackHandler func(info TrackInfo)

// WebRTCReceiver handles WebRTC connections and receives media streams
type WebRTCReceiver struct {
	mu             sync.RWMutex
	peerConnection *webrtc.PeerConnection
	onRTP          RTPHandler
	onTrack        TrackHandler
	onICECandidate func(candidate json.RawMessage) error
}

// NewWebRTCReceiver creates a new WebRTC receiver
func NewWebRTCReceiver() (*WebRTCReceiver, error) {
	return &WebRTCReceiver{}, nil
}

// SetRTPHandler sets the handler for incoming RTP packets
func (w *WebRTCReceiver) SetRTPHandler(handler RTPHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onRTP = handler
}

// SetTrackHandler sets the handler for new tracks
func (w *WebRTCReceiver) SetTrackHandler(handler TrackHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onTrack = handler
}

// SetICECandidateHandler sets the handler for outgoing ICE candidates
func (w *WebRTCReceiver) SetICECandidateHandler(handler func(candidate json.RawMessage) error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onICECandidate = handler
}

// HandleOffer processes an SDP offer and returns an SDP answer
func (w *WebRTCReceiver) HandleOffer(offerSDP string) (string, error) {
	// Create a MediaEngine with H.264 and Opus codecs
	mediaEngine := &webrtc.MediaEngine{}

	// Register H.264 codec for video
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return "", err
	}

	// Register Opus codec for audio
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return "", err
	}

	// Create API with MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	// Create PeerConnection configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	// Create PeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	w.mu.Lock()
	// Close existing connection if any
	if w.peerConnection != nil {
		w.peerConnection.Close()
	}
	w.peerConnection = peerConnection
	w.mu.Unlock()

	// Set up track handler
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		codec := track.Codec()
		log.Printf("Track received: %s (MimeType: %s, PayloadType: %d)",
			track.Kind().String(), codec.MimeType, codec.PayloadType)

		// Notify about new track
		w.mu.RLock()
		trackHandler := w.onTrack
		w.mu.RUnlock()

		if trackHandler != nil {
			trackHandler(TrackInfo{
				Kind:        track.Kind(),
				PayloadType: uint8(codec.PayloadType),
				MimeType:    codec.MimeType,
			})
		}

		w.mu.RLock()
		rtpHandler := w.onRTP
		w.mu.RUnlock()

		if rtpHandler == nil {
			return
		}

		for {
			packet, _, err := track.ReadRTP()
			if err != nil {
				log.Printf("Error reading RTP: %v", err)
				return
			}
			rtpHandler(track, packet)
		}
	})

	// Handle ICE candidates
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		w.mu.RLock()
		handler := w.onICECandidate
		w.mu.RUnlock()

		if handler != nil {
			candidateJSON, err := json.Marshal(candidate.ToJSON())
			if err != nil {
				log.Printf("Failed to marshal ICE candidate: %v", err)
				return
			}
			if err := handler(candidateJSON); err != nil {
				log.Printf("Failed to send ICE candidate: %v", err)
			}
		}
	})

	// Handle connection state changes
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection state changed: %s", state.String())
	})

	// Set remote description (offer)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	// Set local description (answer)
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return answer.SDP, nil
}

// AddICECandidate adds a remote ICE candidate
func (w *WebRTCReceiver) AddICECandidate(candidateJSON json.RawMessage) error {
	w.mu.RLock()
	pc := w.peerConnection
	w.mu.RUnlock()

	if pc == nil {
		return nil
	}

	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
		return err
	}

	return pc.AddICECandidate(candidate)
}

// Close closes the WebRTC connection
func (w *WebRTCReceiver) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.peerConnection != nil {
		return w.peerConnection.Close()
	}
	return nil
}
