package server

import (
	"log"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// RTSPServer handles RTSP streaming
type RTSPServer struct {
	mu          sync.RWMutex
	server      *gortsplib.Server
	stream      *gortsplib.ServerStream
	desc        *description.Session
	videoFormat *format.H264
	audioFormat *format.Opus
	videoMedia  *description.Media
	audioMedia  *description.Media
}

// NewRTSPServer creates a new RTSP server
func NewRTSPServer(address string) (*RTSPServer, error) {
	rs := &RTSPServer{}

	// Create H.264 format with default SPS/PPS (will be updated when stream starts)
	rs.videoFormat = &format.H264{
		PayloadTyp:        96,
		PacketizationMode: 1,
	}

	// Create Opus format
	rs.audioFormat = &format.Opus{
		PayloadTyp:   111,
		ChannelCount: 2,
	}

	// Create media descriptions
	rs.videoMedia = &description.Media{
		Type:    description.MediaTypeVideo,
		Formats: []format.Format{rs.videoFormat},
	}

	rs.audioMedia = &description.Media{
		Type:    description.MediaTypeAudio,
		Formats: []format.Format{rs.audioFormat},
	}

	// Create session description
	rs.desc = &description.Session{
		Medias: []*description.Media{
			rs.videoMedia,
			rs.audioMedia,
		},
	}

	// Create server
	rs.server = &gortsplib.Server{
		Handler:     rs,
		RTSPAddress: address,
	}

	return rs, nil
}

// Start starts the RTSP server
func (rs *RTSPServer) Start() error {
	log.Printf("Starting RTSP server on %s", rs.server.RTSPAddress)

	// Create stream immediately so clients can connect
	rs.mu.Lock()
	rs.stream = gortsplib.NewServerStream(rs.server, rs.desc)
	rs.mu.Unlock()

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

// WriteVideoPacket writes an H.264 RTP packet to RTSP clients
func (rs *RTSPServer) WriteVideoPacket(packet *rtp.Packet) error {
	rs.mu.RLock()
	stream := rs.stream
	rs.mu.RUnlock()

	if stream == nil {
		return nil
	}

	return stream.WritePacketRTP(rs.videoMedia, packet)
}

// WriteAudioPacket writes an Opus RTP packet to RTSP clients
func (rs *RTSPServer) WriteAudioPacket(packet *rtp.Packet) error {
	rs.mu.RLock()
	stream := rs.stream
	rs.mu.RUnlock()

	if stream == nil {
		return nil
	}

	return stream.WritePacketRTP(rs.audioMedia, packet)
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
	rs.mu.RUnlock()

	if stream == nil {
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
	rs.mu.RUnlock()

	if stream == nil {
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
