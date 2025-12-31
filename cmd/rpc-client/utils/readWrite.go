package utils

import (
	"encoding/json"
	"net/http"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/tidwall/gjson"
)

func ExtractRequestID(body []byte) interface{} {
	idResult := gjson.GetBytes(body, "id")
	if !idResult.Exists() {
		return nil
	}
	var id interface{}
	if err := json.Unmarshal([]byte(idResult.Raw), &id); err != nil {
		return nil
	}
	return id
}

func WriteJSON(w http.ResponseWriter, resp rpc_client.JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")

	// Đảm bảo Result luôn là string (không phải interface{}) để tránh serialize thành null
	// Đặc biệt quan trọng cho robot transactions
	if resp.Result != nil {
		if resultStr, ok := resp.Result.(string); ok && len(resultStr) > 0 {
			// Đảm bảo Result là string trước khi serialize
			resp.Result = resultStr
			if resultStr[:2] == "0x" {
				logger.Info("📤 [WriteJSON] Sending response: id=%v, result=%s (type=string, len=%d)",
					resp.Id, resultStr, len(resultStr))
			}
		} else {
			// Nếu Result không phải string, log warning
			logger.Warn("⚠️ [WriteJSON] Result is not string: id=%v, type=%T, value=%v",
				resp.Id, resp.Result, resp.Result)
		}
	} else if resp.Error == nil {
		logger.Warn("⚠️ [WriteJSON] Response has no Result and no Error: id=%v", resp.Id)
	}

	// Marshal JSON để kiểm tra output trước khi gửi
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		logger.Error("❌ [WriteJSON] Failed to marshal JSON response: %v, id=%v", err, resp.Id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Log JSON output để debug (chỉ cho robot transactions với txHash)
	if resp.Result != nil {
		if resultStr, ok := resp.Result.(string); ok && len(resultStr) > 0 && resultStr[:2] == "0x" {
			logger.Info("📤 [WriteJSON] JSON output: %s", string(jsonBytes))
		}
	}

	// Gửi response
	if _, err := w.Write(jsonBytes); err != nil {
		logger.Error("❌ [WriteJSON] Failed to write response: %v, id=%v", err, resp.Id)
	} else {
		if resp.Result != nil {
			logger.Debug("✅ [WriteJSON] Response sent successfully: id=%v", resp.Id)
		}
	}
}
