package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
)

type RPCRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type RPCResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  string `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	url := "https://rpc-proxy-sequoia.ibe.app:8446"

	requestBody := RPCRequest{
		Jsonrpc: "2.0",
		Method:  "eth_chainId",
		Params:  []interface{}{},
		ID:      1,
	}

	data, err := json.Marshal(requestBody)
	if err != nil {
		panic(err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var rpcResp RPCResponse
	err = json.Unmarshal(body, &rpcResp)
	if err != nil {
		panic(err)
	}

	if rpcResp.Error != nil {
		fmt.Printf("Lỗi: %s\n", rpcResp.Error.Message)
		return
	}

	fmt.Printf("Chain ID (hex): %s\n", rpcResp.Result)

	// Chuyển từ hex sang decimal
	chainID := new(big.Int)
	chainID.SetString(strings.Replace(rpcResp.Result, "0x", "", 1), 16)
	fmt.Printf("Chain ID (decimal): %s\n", chainID.String())
}
