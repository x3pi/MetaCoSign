package tcp_server

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	pkgCommon "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/network"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	t_network "github.com/meta-node-blockchain/meta-node/types/network"
	"google.golang.org/protobuf/proto"
)

// handleGetRegistrationMessage - lấy 32-byte hashToSign từ chain qua TCP
func (srv *RpcTcpServer) handleGetRegistrationMessage(request t_network.Request) error {
	conn := request.Connection()
	msg := request.Message()
	if msg == nil {
		return nil
	}
	body := msg.Body()
	if len(body) < common.AddressLength {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: GetRegistrationMessage body must be at least 20-byte address",
		})
	}
	addr := common.BytesToAddress(body[:common.AddressLength])

	if srv.AppCtx == nil || srv.AppCtx.ChainPool == nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Chain connection pool is not available",
		})
	}

	chainClient, err := srv.AppCtx.ChainPool.Get()
	if err != nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to get chain connection: " + err.Error(),
		})
	}

	hashBytes, err := chainClient.GetRegistrationMessage(addr.Bytes(), 30*time.Second)
	if err != nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "GetRegistrationMessage TCP error: " + err.Error(),
		})
	}

	respMsg := network.NewMessage(&pb.Message{
		Header: &pb.Header{
			Command:   pkgCommon.RegistrationMessage,
			Version:   msg.Version(),
			ToAddress: conn.Address().Bytes(),
			ID:        msg.ID(),
		},
		Body: hashBytes,
	})

	return conn.SendMessage(respMsg)
}

// handleGetRegistrationStatus - tra cứu trạng thái đăng ký của địa chỉ
func (srv *RpcTcpServer) handleGetRegistrationStatus(request t_network.Request) error {
	conn := request.Connection()
	msg := request.Message()
	if msg == nil {
		return nil
	}
	body := msg.Body()
	if len(body) < common.AddressLength {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: GetRegistrationStatus body must be at least 20-byte address",
		})
	}
	addr := common.BytesToAddress(body[:common.AddressLength])

	if srv.AppCtx == nil || srv.AppCtx.ChainPool == nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Chain connection pool is not available",
		})
	}

	chainClient, err := srv.AppCtx.ChainPool.Get()
	if err != nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to get chain connection: " + err.Error(),
		})
	}

	statusBytes, err := chainClient.GetRegistrationStatus(addr.Bytes(), 30*time.Second)
	if err != nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "GetRegistrationStatus TCP error: " + err.Error(),
		})
	}

	respMsg := network.NewMessage(&pb.Message{
		Header: &pb.Header{
			Command:   pkgCommon.RegistrationStatus,
			Version:   msg.Version(),
			ToAddress: conn.Address().Bytes(),
			ID:        msg.ID(),
		},
		Body: statusBytes,
	})
	logger.Info("✅ TCP GetRegistrationStatus: sent RegistrationStatus for addr=%s reqID=%s (%d bytes)",
		addr.Hex(), msg.ID(), len(statusBytes))
	return conn.SendMessage(respMsg)
}

// handleRegisterAccount - nhận yêu cầu đăng ký tài khoản, forward lên chain node, gửi ACK và Async Push kết quả về client
func (srv *RpcTcpServer) handleRegisterAccount(request t_network.Request) error {
	conn := request.Connection()
	msg := request.Message()
	if msg == nil {
		return nil
	}
	body := msg.Body()

	if srv.AppCtx == nil || srv.AppCtx.ChainPool == nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Chain connection pool is not available",
		})
	}

	chainClient, err := srv.AppCtx.ChainPool.Get()
	if err != nil {
		return srv.sendRpcResponse(conn, msg.ID(), nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to get chain connection: " + err.Error(),
		})
	}

	logger.Info("🔄 TCP RegisterAccount from %s (msgID=%s, %d bytes)", conn.RemoteAddrSafe(), msg.ID(), len(body))

	onAck := func(ackBytes []byte) {
		ackMsg := network.NewMessage(&pb.Message{
			Header: &pb.Header{
				Command:   pkgCommon.RegisterAccountAck,
				Version:   msg.Version(),
				ToAddress: conn.Address().Bytes(),
				ID:        msg.ID(),
			},
			Body: ackBytes,
		})
		if err := conn.SendMessage(ackMsg); err != nil {
			logger.Warn("Failed to send RegisterAccountAck to client %s: %v", conn.RemoteAddrSafe(), err)
		} else {
			logger.Info("✅ TCP RegisterAccountAck forwarded to %s (msgID=%s)", conn.RemoteAddrSafe(), msg.ID())
		}
	}

	resBytes, isPush, err := chainClient.RegisterAccount(body, onAck, 60*time.Second)
	if err != nil {
		logger.Error("❌ TCP RegisterAccount error: %v", err)
		errRes := &pb.RegistrationResult{
			Status: "FAILED",
			Reason: err.Error(),
		}
		errBytes, _ := proto.Marshal(errRes)
		errMsg := network.NewMessage(&pb.Message{
			Header: &pb.Header{
				Command:   pkgCommon.RegisterAccountAck,
				Version:   msg.Version(),
				ToAddress: conn.Address().Bytes(),
				ID:        msg.ID(),
			},
			Body: errBytes,
		})
		return conn.SendMessage(errMsg)
	}

	if isPush && len(resBytes) > 0 {
		resMsg := network.NewMessage(&pb.Message{
			Header: &pb.Header{
				Command:   pkgCommon.RegistrationResult,
				Version:   msg.Version(),
				ToAddress: conn.Address().Bytes(),
				ID:        msg.ID(),
			},
			Body: resBytes,
		})
		if err := conn.SendMessage(resMsg); err != nil {
			logger.Warn("Failed to send RegistrationResult to client %s: %v", conn.RemoteAddrSafe(), err)
			return err
		}
		logger.Info("✅ TCP RegistrationResult forwarded to %s (msgID=%s)", conn.RemoteAddrSafe(), msg.ID())
	}

	return nil
}
