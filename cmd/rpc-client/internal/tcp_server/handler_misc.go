package tcp_server

import (
	"encoding/json"
	"fmt"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	t_network "github.com/meta-node-blockchain/meta-node/types/network"
)

// handleChainIdDirect - lấy chainId qua TCP trực tiếp
func (srv *RpcTcpServer) handleChainIdDirect(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	chainId := srv.AppCtx.Cfg.ChainId
	// Convert uint64 → hex string giống eth_chainId format
	hexStr := fmt.Sprintf("0x%x", chainId)
	resultBytes, _ := json.Marshal(hexStr)
	return srv.sendRpcResponse(conn, msgID, resultBytes, nil)
}

// handleRegisterBlsKey - xử lý giống HTTP handler
func (srv *RpcTcpServer) handleRegisterBlsKey(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params for rpc_registerBlsKeyWithSignature",
		})
	}

	logger.Info("🔄 TCP rpc_registerBlsKeyWithSignature from %s", conn.RemoteAddrSafe())
	httpResp := handlers.HandleRpcRegisterBlsKeyWithSignatureRaw(srv.AppCtx, params[0], msgID)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP rpc_registerBlsKeyWithSignature result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}
