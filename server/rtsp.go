package server

import (
	"log"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
)

// RTSPServer handles RTSP streaming
type RTSPServer struct {
	mu             sync.RWMutex
	server         *gortsplib.Server
	stream         *gortsplib.ServerStream
	videoMedia     *description.Media
	audioMedia     *description.Media
	videoFormat    *format.H264
	initialized    bool
	videoPayload   uint8
	audioPayload   uint8
	sps            []byte
	pps            []byte
	spsReceived    bool
	ppsReceived    bool
	debug          bool
	videoPacketCnt int
}

// NewRTSPServer creates a new RTSP server
func NewRTSPServer(address string, debug bool) (*RTSPServer, error) {
	rs := &RTSPServer{
		debug: debug,
	}

	// Create server (stream will be created when first packet arrives)
	rs.server = &gortsplib.Server{
		Handler:     rs,
		RTSPAddress: address,
	}

	return rs, nil
}

// InitStream initializes the RTSP stream with the given payload types
func (rs *RTSPServer) InitStream(videoPayloadType, audioPayloadType uint8) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Reset stream if already initialized (reconnection)
	if rs.initialized {
		if rs.stream != nil {
			rs.stream.Close()
			rs.stream = nil
		}
		rs.initialized = false
		rs.spsReceived = false
		rs.ppsReceived = false
		rs.sps = nil
		rs.pps = nil
		rs.videoPacketCnt = 0
		log.Printf("RTSP stream reset for new connection")
	}

	rs.videoPayload = videoPayloadType
	rs.audioPayload = audioPayloadType

	log.Printf("RTSP waiting for SPS/PPS (video PT: %d, audio PT: %d)", videoPayloadType, audioPayloadType)
}

// initializeStream creates the RTSP stream after SPS/PPS are received
func (rs *RTSPServer) initializeStream() {
	// Create H.264 format with SPS/PPS
	rs.videoFormat = &format.H264{
		PayloadTyp:        rs.videoPayload,
		PacketizationMode: 1,
		SPS:               rs.sps,
		PPS:               rs.pps,
	}

	// Create Opus format
	audioFormat := &format.Opus{
		PayloadTyp:   rs.audioPayload,
		ChannelCount: 2,
	}

	// Create media descriptions
	rs.videoMedia = &description.Media{
		Type:    description.MediaTypeVideo,
		Formats: []format.Format{rs.videoFormat},
	}

	rs.audioMedia = &description.Media{
		Type:    description.MediaTypeAudio,
		Formats: []format.Format{audioFormat},
	}

	// Create session description
	desc := &description.Session{
		Medias: []*description.Media{
			rs.videoMedia,
			rs.audioMedia,
		},
	}

	// Create and initialize stream
	rs.stream = &gortsplib.ServerStream{
		Server: rs.server,
		Desc:   desc,
	}
	if err := rs.stream.Initialize(); err != nil {
		log.Printf("Failed to initialize RTSP stream: %v", err)
		return
	}
	rs.initialized = true

	log.Printf("RTSP stream initialized with SPS/PPS")
}

// Start starts the RTSP server
func (rs *RTSPServer) Start() error {
	log.Printf("Starting RTSP server on %s", rs.server.RTSPAddress)
	return rs.server.Start()
}

// Close stops the RTSP server
func (rs *RTSPServer) Close() {
	rs.mu.Lock()
	if rs.stream != nil {
		rs.stream.Close()
	}
	rs.mu.Unlock()
	rs.server.Close()
}

// extractSPSPPS extracts SPS and PPS from H.264 RTP packet
func (rs *RTSPServer) extractSPSPPS(payload []byte) {
	if len(payload) < 1 {
		return
	}

	// Get NAL unit type (lower 5 bits of first byte)
	nalType := h264.NALUType(payload[0] & 0x1F)

	switch nalType {
	case h264.NALUTypeSPS:
		if !rs.spsReceived {
			rs.sps = make([]byte, len(payload))
			copy(rs.sps, payload)
			rs.spsReceived = true
			log.Printf("SPS received (%d bytes)", len(rs.sps))
		}
	case h264.NALUTypePPS:
		if !rs.ppsReceived {
			rs.pps = make([]byte, len(payload))
			copy(rs.pps, payload)
			rs.ppsReceived = true
			log.Printf("PPS received (%d bytes)", len(rs.pps))
		}
	case 24: // STAP-A - Single-Time Aggregation Packet
		// STAP-A can contain multiple NAL units including SPS/PPS
		rs.parseSTAPA(payload[1:])
	}
}

// parseSTAPA parses STAP-A packet to extract SPS/PPS
func (rs *RTSPServer) parseSTAPA(payload []byte) {
	for len(payload) >= 2 {
		// Get NAL unit size (2 bytes big-endian)
		nalSize := int(payload[0])<<8 | int(payload[1])
		payload = payload[2:]

		if len(payload) < nalSize || nalSize < 1 {
			break
		}

		nalType := h264.NALUType(payload[0] & 0x1F)
		nalData := payload[:nalSize]

		switch nalType {
		case h264.NALUTypeSPS:
			if !rs.spsReceived {
				rs.sps = make([]byte, nalSize)
				copy(rs.sps, nalData)
				rs.spsReceived = true
				log.Printf("SPS received from STAP-A (%d bytes)", len(rs.sps))
			}
		case h264.NALUTypePPS:
			if !rs.ppsReceived {
				rs.pps = make([]byte, nalSize)
				copy(rs.pps, nalData)
				rs.ppsReceived = true
				log.Printf("PPS received from STAP-A (%d bytes)", len(rs.pps))
			}
		}

		payload = payload[nalSize:]
	}
}

// WriteVideoPacket writes an H.264 RTP packet to RTSP clients
func (rs *RTSPServer) WriteVideoPacket(packet *rtp.Packet) error {
	if len(packet.Payload) == 0 {
		return nil
	}

	rs.mu.Lock()

	// Try to extract SPS/PPS if not initialized
	if !rs.initialized {
		rs.extractSPSPPS(packet.Payload)

		// Initialize stream once we have both SPS and PPS
		if rs.spsReceived && rs.ppsReceived && !rs.initialized {
			rs.initializeStream()
		}
	}

	stream := rs.stream
	videoMedia := rs.videoMedia
	initialized := rs.initialized
	rs.mu.Unlock()

	if !initialized || stream == nil || videoMedia == nil {
		return nil
	}

	// Debug: log first few packets
	rs.mu.Lock()
	rs.videoPacketCnt++
	cnt := rs.videoPacketCnt
	rs.mu.Unlock()

	if rs.debug && cnt <= 10 {
		nalType := packet.Payload[0] & 0x1F
		log.Printf("[DEBUG] Video RTP #%d: seq=%d, ts=%d, PT=%d, payload=%d bytes, NAL type=%d",
			cnt, packet.SequenceNumber, packet.Timestamp, packet.PayloadType, len(packet.Payload), nalType)
	}

	return stream.WritePacketRTP(videoMedia, packet)
}

// WriteAudioPacket writes an Opus RTP packet to RTSP clients
func (rs *RTSPServer) WriteAudioPacket(packet *rtp.Packet) error {
	rs.mu.RLock()
	stream := rs.stream
	audioMedia := rs.audioMedia
	initialized := rs.initialized
	rs.mu.RUnlock()

	if !initialized || stream == nil || audioMedia == nil {
		return nil
	}

	return stream.WritePacketRTP(audioMedia, packet)
}

// OnConnOpen implements gortsplib.ServerHandler
func (rs *RTSPServer) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("RTSP client connected: %v", ctx.Conn.NetConn().RemoteAddr())
}

// OnConnClose implements gortsplib.ServerHandler
func (rs *RTSPServer) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("RTSP client disconnected: %v", ctx.Conn.NetConn().RemoteAddr())
}

// OnSessionOpen implements gortsplib.ServerHandler
func (rs *RTSPServer) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("RTSP session opened")
}

// OnSessionClose implements gortsplib.ServerHandler
func (rs *RTSPServer) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("RTSP session closed")
}

// OnDescribe implements gortsplib.ServerHandler
func (rs *RTSPServer) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("RTSP DESCRIBE request for path: %s", ctx.Path)

	rs.mu.RLock()
	stream := rs.stream
	initialized := rs.initialized
	rs.mu.RUnlock()

	if !initialized || stream == nil {
		log.Printf("RTSP stream not ready yet - waiting for WebRTC connection and SPS/PPS")
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, stream, nil
}

// OnAnnounce implements gortsplib.ServerHandler
func (rs *RTSPServer) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// OnSetup implements gortsplib.ServerHandler
func (rs *RTSPServer) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("RTSP SETUP request")

	rs.mu.RLock()
	stream := rs.stream
	initialized := rs.initialized
	rs.mu.RUnlock()

	if !initialized || stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, stream, nil
}

// OnPlay implements gortsplib.ServerHandler
func (rs *RTSPServer) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("RTSP PLAY request - client started playback")
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// OnRecord implements gortsplib.ServerHandler
func (rs *RTSPServer) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// OnPause implements gortsplib.ServerHandler
func (rs *RTSPServer) OnPause(ctx *gortsplib.ServerHandlerOnPauseCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}
