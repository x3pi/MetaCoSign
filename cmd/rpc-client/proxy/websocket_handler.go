package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  65536, // 64KB
	WriteBufferSize: 65536, // 64KB
}

func (p *RpcReverseProxy) ServeWebSocket(w http.ResponseWriter, r *http.Request, targetURL string) {
	// Upgrade HTTP connection to WebSocket
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed for %s: %v", r.RemoteAddr, err)
		return
	}
	defer clientConn.Close()

	clientWriter := NewWebSocketWriter(clientConn)

	// Validate target URL
	if targetURL == "" {
		logger.Error("Target WebSocket URL not configured for client %s", clientConn.RemoteAddr())
		_ = clientWriter.WriteCloseMessage(websocket.CloseInternalServerErr, "Target WebSocket URL not configured")
		return
	}

	// Connect to upstream WebSocket server
	targetConn, err := p.dialUpstreamWebSocket(targetURL, r, clientConn.RemoteAddr().String())
	if err != nil {
		logger.Error("Failed to connect to upstream WebSocket %s: %v", targetURL, err)
		_ = clientWriter.WriteCloseMessage(websocket.CloseGoingAway, fmt.Sprintf("Failed to connect to upstream: %v", err))
		return
	}
	defer targetConn.Close()

	targetWriter := NewWebSocketWriter(targetConn)

	// Proxy bidirectional traffic
	p.proxyWebSocketTraffic(clientConn, targetConn, clientWriter, targetWriter, r)
}

// dialUpstreamWebSocket establishes connection to upstream WebSocket server
func (p *RpcReverseProxy) dialUpstreamWebSocket(targetURL string, r *http.Request, clientAddr string) (*websocket.Conn, error) {
	targetHeaders := make(http.Header)
	if origin := r.Header.Get("Origin"); origin != "" {
		targetHeaders.Set("Origin", origin)
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		ReadBufferSize:    65536,
		WriteBufferSize:   65536,
		EnableCompression: false,
		NetDialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	targetConn, resp, err := dialer.Dial(targetURL, targetHeaders)
	if err != nil {
		return p.handleDialError(err, resp, targetURL, clientAddr)
	}

	return targetConn, nil
}

// handleDialError xử lý chi tiết lỗi khi kết nối upstream WebSocket
func (p *RpcReverseProxy) handleDialError(err error, resp *http.Response, targetURL, clientAddr string) (*websocket.Conn, error) {
	// Default error message
	errMsg := fmt.Sprintf("Could not connect to target WebSocket %s", targetURL)
	fullErrMsgLog := fmt.Sprintf("%s for client %s: %v", errMsg, clientAddr, err)
	fullErrMsgClient := fmt.Sprintf("%s: %v", errMsg, err)

	if resp != nil {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)

		errMsg = fmt.Sprintf("Target WebSocket handshake failed with HTTP status %s", resp.Status)
		fullErrMsgLog = fmt.Sprintf("%s for client %s. Response Body: %s", errMsg, clientAddr, string(bodyBytes))
		fullErrMsgClient = fmt.Sprintf("%s. Server responded with: %s", errMsg, string(bodyBytes))
		// ✅ Xử lý case đặc biệt: 200 OK thay vì 101 Switching Protocols
		if resp.StatusCode == http.StatusOK {
			logger.Error("Handshake Error: Target server responded with 200 OK instead of 101 Switching Protocols. Ensure WSS URL path is correct (e.g., includes /ws if required).")
			fullErrMsgClient = "WebSocket handshake error: Target server returned HTTP 200 OK, not a WebSocket upgrade. Check WSS URL path in config."
		} else if resp.StatusCode >= http.StatusInternalServerError {
			fullErrMsgClient = fmt.Sprintf("Upstream server error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
		} else if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
			fullErrMsgClient = fmt.Sprintf("Client/config error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
		}
	}
	logger.Error(fullErrMsgLog)

	return nil, fmt.Errorf("%s", fullErrMsgClient)
}

// proxyWebSocketTraffic handles bidirectional message forwarding
func (p *RpcReverseProxy) proxyWebSocketTraffic(
	clientConn, targetConn *websocket.Conn,
	clientWriter, targetWriter *WebSocketWriter,
	r *http.Request,
) {
	ctx := r.Context()
	errChan := make(chan error, 2)
	quit := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(2)

	// Goroutine 1: Client → Upstream (with RPC method handling)
	go func() {
		defer wg.Done()
		p.proxyClientToUpstream(clientConn, targetConn, clientWriter, targetWriter, errChan, quit)
	}()

	// Goroutine 2: Upstream → Client (passthrough)
	go func() {
		defer wg.Done()
		p.proxyUpstreamToClient(targetConn, clientWriter, errChan, quit)
	}()

	// Wait for error or context cancellation
	var finalError error
	select {
	case err := <-errChan:
		finalError = err
	case err := <-errChan:
		if finalError == nil {
			finalError = err
		}
	case <-ctx.Done():
		finalError = ctx.Err()
	}

	close(quit)

	// Send close message to client
	if finalError != nil {
		if !isExpectedCloseError(finalError) {
			logger.Error("WebSocket proxy error for %s: %v", clientConn.RemoteAddr(), finalError)
			_ = clientWriter.WriteCloseMessage(websocket.CloseInternalServerErr, "Proxy error")
		}
	} else {
		_ = clientWriter.WriteCloseMessage(websocket.CloseNormalClosure, "Connection closing normally")
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Debug("WebSocket goroutines finished for %s", clientConn.RemoteAddr())
	case <-time.After(5 * time.Second):
		logger.Warn("Timeout waiting for WebSocket goroutines for %s", clientConn.RemoteAddr())
	}
}

// proxyClientToUpstream handles Client → Upstream traffic with RPC routing
func (p *RpcReverseProxy) proxyClientToUpstream(
	clientConn, targetConn *websocket.Conn,
	clientWriter, targetWriter *WebSocketWriter,
	errChan chan<- error,
	quit <-chan struct{},
) {
	for {
		select {
		case <-quit:
			return
		default:
		}

		// Read JSON-RPC request from client
		var req models.JSONRPCRequestRaw
		if err := clientConn.SetReadDeadline(time.Now().Add(180 * time.Second)); err != nil {
			logger.Warn("Error setting read deadline: %v", err)
		}
		readErr := clientConn.ReadJSON(&req)
		_ = clientConn.SetReadDeadline(time.Time{})

		if readErr != nil {
			if !isExpectedCloseError(readErr) {
				logger.Error("Error reading from client %s: %v", clientConn.RemoteAddr(), readErr)
				select {
				case errChan <- fmt.Errorf("client read error: %w", readErr):
				case <-quit:
				}
			}
			return
		}

		// Try to handle RPC method locally
		rpcResp, handled := p.RouteWebSocketMessage(req)
		if handled && rpcResp != nil {
			// Send response back to client
			if err := clientWriter.WriteJSON(rpcResp); err != nil {
				logger.Error("Error writing RPC response to client %s: %v", clientConn.RemoteAddr(), err)
				select {
				case errChan <- fmt.Errorf("client write error: %w", err):
				case <-quit:
				}
				return
			}
		} else {
			// Forward to upstream server
			if err := targetWriter.WriteJSON(req); err != nil {
				logger.Error("Error writing to upstream for client %s: %v", clientConn.RemoteAddr(), err)
				select {
				case errChan <- fmt.Errorf("upstream write error: %w", err):
				case <-quit:
				}
				return
			}
		}
	}
}

// proxyUpstreamToClient handles Upstream → Client traffic (passthrough)
func (p *RpcReverseProxy) proxyUpstreamToClient(
	targetConn *websocket.Conn,
	clientWriter *WebSocketWriter,
	errChan chan<- error,
	quit <-chan struct{},
) {
	for {
		select {
		case <-quit:
			return
		default:
		}

		// Read message from upstream
		if err := targetConn.SetReadDeadline(time.Now().Add(180 * time.Second)); err != nil {
			logger.Warn("Error setting read deadline: %v", err)
		}
		messageType, message, readErr := targetConn.ReadMessage()
		_ = targetConn.SetReadDeadline(time.Time{})

		if readErr != nil {
			if !isExpectedCloseError(readErr) {
				logger.Error("Error reading from upstream: %v", readErr)
				select {
				case errChan <- fmt.Errorf("upstream read error: %w", readErr):
				case <-quit:
				}
			}
			return
		}

		// Forward message to client
		if err := clientWriter.WriteMessage(messageType, message); err != nil {
			logger.Error("Error writing to client: %v", err)
			select {
			case errChan <- fmt.Errorf("client write error: %w", err):
			case <-quit:
			}
			return
		}
	}
}

// isExpectedCloseError checks if error is an expected close error
func isExpectedCloseError(err error) bool {
	if err == io.EOF {
		return true
	}
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
		websocket.CloseNoStatusReceived) {
		return true
	}
	if strings.Contains(err.Error(), "client read error") ||
		strings.Contains(err.Error(), "client write error") {
		return true
	}
	return false
}
