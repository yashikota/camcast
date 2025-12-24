package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/yashikota/camcast/server"
)

//go:embed web/*
var webFS embed.FS

const (
	httpAddr = ":8080"
	rtspAddr = ":8554"
)

var debugMode bool

func main() {
	flag.BoolVar(&debugMode, "debug", false, "Enable debug logging")
	flag.Parse()

	log.Println("Starting CamCast...")
	if debugMode {
		log.Println("Debug mode enabled")
	}

	// Create RTSP server
	rtspServer, err := server.NewRTSPServer(rtspAddr, debugMode)
	if err != nil {
		log.Fatalf("Failed to create RTSP server: %v", err)
	}

	// Start RTSP server
	if err := rtspServer.Start(); err != nil {
		log.Fatalf("Failed to start RTSP server: %v", err)
	}
	defer rtspServer.Close()

	// Create WebRTC receiver
	webrtcReceiver, err := server.NewWebRTCReceiver()
	if err != nil {
		log.Fatalf("Failed to create WebRTC receiver: %v", err)
	}
	defer webrtcReceiver.Close()

	// Create signaling server
	signalingServer := server.NewSignalingServer()

	// Track payload types for RTSP initialization
	var (
		trackMu          sync.Mutex
		videoPayloadType uint8
		audioPayloadType uint8
		hasVideo         bool
		hasAudio         bool
	)

	// Handle track information to initialize RTSP
	webrtcReceiver.SetTrackHandler(func(info server.TrackInfo) {
		trackMu.Lock()
		defer trackMu.Unlock()

		switch info.Kind {
		case webrtc.RTPCodecTypeVideo:
			videoPayloadType = info.PayloadType
			hasVideo = true
			log.Printf("Video track: PayloadType=%d, MimeType=%s", info.PayloadType, info.MimeType)
		case webrtc.RTPCodecTypeAudio:
			audioPayloadType = info.PayloadType
			hasAudio = true
			log.Printf("Audio track: PayloadType=%d, MimeType=%s", info.PayloadType, info.MimeType)
		}

		// Initialize/reinitialize RTSP when we have video
		if hasVideo {
			// Use default audio payload if not received yet
			if !hasAudio {
				audioPayloadType = 111
			}
			rtspServer.InitStream(videoPayloadType, audioPayloadType)
			// Reset flags for next track pair
			hasVideo = false
			hasAudio = false
		}
	})

	// Set up RTP handler to forward packets to RTSP
	webrtcReceiver.SetRTPHandler(func(track *webrtc.TrackRemote, packet *rtp.Packet) {
		switch track.Kind() {
		case webrtc.RTPCodecTypeVideo:
			if err := rtspServer.WriteVideoPacket(packet); err != nil {
				log.Printf("Failed to write video packet: %v", err)
			}
		case webrtc.RTPCodecTypeAudio:
			if err := rtspServer.WriteAudioPacket(packet); err != nil {
				log.Printf("Failed to write audio packet: %v", err)
			}
		}
	})

	// Connect signaling to WebRTC receiver
	signalingServer.SetOfferHandler(webrtcReceiver.HandleOffer)
	signalingServer.SetICEHandler(webrtcReceiver.AddICECandidate)
	webrtcReceiver.SetICECandidateHandler(signalingServer.SendICECandidate)

	// Set up HTTP server
	mux := http.NewServeMux()

	// Serve WebSocket endpoint
	mux.HandleFunc("/ws", signalingServer.HandleWebSocket)

	// Serve static files from embedded filesystem
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to get web content: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Start HTTP server in goroutine
	go func() {
		log.Printf("HTTP server starting on http://localhost%s", httpAddr)
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait a moment for server to start, then open browser
	time.Sleep(500 * time.Millisecond)
	openBrowser("http://localhost" + httpAddr)

	log.Printf("RTSP server available at rtsp://localhost%s/stream", rtspAddr)
	log.Println("Press Ctrl+C to stop")

	// Block forever
	select {}
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		log.Printf("Please open %s in your browser", url)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open %s in your browser", url)
	}
}
