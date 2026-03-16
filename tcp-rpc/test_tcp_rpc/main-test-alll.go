package main

// import (
// 	"crypto/ecdsa"
// 	"encoding/hex"
// 	"encoding/json"
// 	"flag"
// 	"fmt"
// 	"math/big"
// 	"os"
// 	"strings"
// 	"sync"
// 	"sync/atomic"
// 	"time"

// 	"github.com/ethereum/go-ethereum/accounts/abi"
// 	"github.com/ethereum/go-ethereum/common"
// 	e_types "github.com/ethereum/go-ethereum/core/types"
// 	"github.com/ethereum/go-ethereum/crypto"

// 	"github.com/meta-node-blockchain/meta-node/pkg/logger"
// 	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
// 	client_tcp "github.com/meta-node-blockchain/meta-node/tcp-rpc/client-tcp"
// 	tcp_config "github.com/meta-node-blockchain/meta-node/tcp-rpc/client-tcp/config"
// 	"google.golang.org/protobuf/proto"
// )

// // ===================== HELPERS =====================

// // waitReceipt đợi receipt với timeout 30s, retry 10ms
// func waitReceipt(tcpClient *client_tcp.Client, txHash string) *pb.RpcReceipt {
// 	if txHash == "" {
// 		return nil
// 	}
// 	timer := time.NewTimer(30 * time.Second)
// 	defer timer.Stop()
// 	for {
// 		receipt, err := tcpClient.RpcGetTransactionReceipt(txHash)
// 		if err != nil {
// 			fmt.Printf("  ❌ Receipt error: %v\n", err)
// 			return nil
// 		}
// 		if receipt != nil {
// 			fmt.Printf("  ✅ Receipt: status=%s, gasUsed=%s, logs=%d\n",
// 				receipt.Status, receipt.GasUsed, len(receipt.Logs))
// 			return receipt
// 		}
// 		select {
// 		case <-timer.C:
// 			fmt.Println("  ⚠️ Timeout waiting for receipt")
// 			return nil
// 		case <-time.After(10 * time.Millisecond):
// 		}
// 	}
// }

// // sendTxAndWait tạo, ký, gửi transaction + đợi receipt
// func sendTxAndWait(
// 	tcpClient *client_tcp.Client,
// 	privKey *ecdsa.PrivateKey,
// 	fromAddr common.Address,
// 	toAddr common.Address,
// 	parsedABI abi.ABI,
// 	signer e_types.Signer,
// 	method string,
// 	args ...interface{},
// ) (string, *pb.RpcReceipt) {
// 	nonce, err := tcpClient.RpcGetPendingNonce(fromAddr)
// 	if err != nil {
// 		fmt.Printf("  ❌ GetNonce: %v\n", err)
// 		return "", nil
// 	}

// 	inputData, err := parsedABI.Pack(method, args...)
// 	if err != nil {
// 		fmt.Printf("  ❌ Pack %s: %v\n", method, err)
// 		return "", nil
// 	}

// 	tx := e_types.NewTransaction(nonce, toAddr, big.NewInt(0), 20000000, big.NewInt(10000000), inputData)
// 	signedTx, _ := e_types.SignTx(tx, signer, privKey)
// 	rawTxBytes, _ := signedTx.MarshalBinary()
// 	rawTxHex := "0x" + hex.EncodeToString(rawTxBytes)

// 	txHash, err := tcpClient.RpcSendRawTransaction(rawTxHex)
// 	if err != nil {
// 		fmt.Printf("  ❌ SendTx %s: %v\n", method, err)
// 		return "", nil
// 	}
// 	fmt.Printf("  ✅ txHash: %s\n", txHash)

// 	receipt := waitReceipt(tcpClient, txHash)
// 	return txHash, receipt
// }

// // makeEventHandler tạo callback log event chung
// func makeEventHandler(eventName string, wg *sync.WaitGroup, once *sync.Once) func([]byte) {
// 	return func(eventData []byte) {
// 		event := &pb.RpcEvent{}
// 		if err := proto.Unmarshal(eventData, event); err != nil {
// 			fmt.Printf("  ❌ Failed to parse RpcEvent: %v\n", err)
// 			return
// 		}

// 		fmt.Printf("\n  📡 [%s] EVENT RECEIVED!\n", eventName)
// 		fmt.Printf("  ├─ SubscriptionID: %s\n", event.SubscriptionId)
// 		if event.Log != nil {
// 			fmt.Printf("  ├─ Contract:       %s\n", event.Log.Address)
// 			fmt.Printf("  ├─ BlockNumber:    %s\n", event.Log.BlockNumber)
// 			fmt.Printf("  ├─ TxHash:         %s\n", event.Log.TransactionHash)
// 			fmt.Printf("  ├─ Topics:         %v\n", event.Log.Topics)
// 			fmt.Printf("  └─ Data:           %s\n", event.Log.Data)
// 		}

// 		if once != nil && wg != nil {
// 			once.Do(func() {
// 				wg.Done()
// 			})
// 		}
// 	}
// }

// // ===================== TEST SUITES =====================

// // testDemoContract test các function cơ bản: subscribe events, setValue, increaseValue, verify
// func testDemoContract(
// 	tcpClient *client_tcp.Client,
// 	demoABI abi.ABI,
// 	contractAddr common.Address,
// 	privKey *ecdsa.PrivateKey,
// 	fromAddr common.Address,
// 	signer e_types.Signer,
// ) {
// 	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
// 	fmt.Println("║  TEST: Demo Contract (setValue + increaseValue)      ║")
// 	fmt.Println("╚══════════════════════════════════════════════════════╝")

// 	// 1. Read current value
// 	// fmt.Println("\n─── getValue (trước) ───")
// 	// getValueData, _ := demoABI.Pack("getValue")
// 	// resultBytes, err := tcpClient.RpcEthCall(contractAddr, getValueData)
// 	// if err != nil {
// 	// 	fmt.Printf("  ❌ %v\n", err)
// 	// 	return
// 	// }
// 	// results, _ := demoABI.Unpack("getValue", resultBytes)
// 	// oldValue := results[0].(*big.Int)
// 	// fmt.Printf("  ✅ getValue() = %s\n", oldValue.String())

// 	// // 2. Subscribe 2 events
// 	// fmt.Println("\n─── Subscribe ValueChanged + ValueIncreased ───")
// 	// var wgChanged, wgIncreased sync.WaitGroup
// 	// var onceChanged, onceIncreased sync.Once
// 	// wgChanged.Add(1)
// 	// wgIncreased.Add(1)

// 	// sub1, _ := tcpClient.RpcSubscribe(
// 	// 	[]string{contractAddr.Hex()},
// 	// 	[]string{demoABI.Events["ValueChanged"].ID.Hex()},
// 	// 	makeEventHandler("ValueChanged", &wgChanged, &onceChanged),
// 	// )
// 	// fmt.Printf("  ✅ ValueChanged subID=%s\n", sub1)

// 	// sub2, _ := tcpClient.RpcSubscribe(
// 	// 	[]string{contractAddr.Hex()},
// 	// 	[]string{demoABI.Events["ValueIncreased"].ID.Hex()},
// 	// 	makeEventHandler("ValueIncreased", &wgIncreased, &onceIncreased),
// 	// )
// 	// fmt.Printf("  ✅ ValueIncreased subID=%s\n", sub2)

// 	// 3. setValue(789) + wait receipt
// 	fmt.Println("\n─── setValue(789) ───")
// 	sendTxAndWait(tcpClient, privKey, fromAddr, contractAddr, demoABI, signer, "setValue", big.NewInt(789))

// 	// 4. increaseValue(100) + wait receipt
// 	fmt.Println("\n─── increaseValue(100) ───")
// 	sendTxAndWait(tcpClient, privKey, fromAddr, contractAddr, demoABI, signer, "increaseValue", big.NewInt(100))

// 	// 5. Wait events
// 	fmt.Println("\n  ⏳ Waiting for both events (max 10s)...")
// 	// allDone := make(chan struct{})
// 	// go func() {
// 	// 	wgChanged.Wait()
// 	// 	wgIncreased.Wait()
// 	// 	close(allDone)
// 	// }()
// 	// select {
// 	// case <-allDone:
// 	// 	fmt.Println("  ✅ Both events received!")
// 	// case <-time.After(10 * time.Second):
// 	// 	fmt.Println("  ⚠️ Timeout")
// 	// }

// 	// 6. Unsubscribe
// 	// tcpClient.RpcUnsubscribe(sub1)
// 	// tcpClient.RpcUnsubscribe(sub2)

// 	// 7. Verify value
// 	// fmt.Println("\n─── getValue (sau) ───")
// 	// resultBytes2, _ := tcpClient.RpcEthCall(contractAddr, getValueData)
// 	// results2, _ := demoABI.Unpack("getValue", resultBytes2)
// 	// newValue := results2[0].(*big.Int)
// 	// fmt.Printf("  ✅ getValue() = %s", newValue.String())
// 	// if newValue.Int64() == 889 {
// 	// 	fmt.Println(" ✅ (789 + 100 = 889)")
// 	// } else {
// 	// 	fmt.Println()
// 	// }
// }

// // testBlsRegistration test flow: tạo key → getPublickeyBls → setBlsPublicKey → confirmAccountWithoutSign
// func testBlsRegistration(
// 	tcpClient *client_tcp.Client,
// 	accountABI abi.ABI,
// 	accountContract common.Address,
// 	adminPrivKey *ecdsa.PrivateKey,
// 	adminAddr common.Address,
// 	signer e_types.Signer,
// ) {
// 	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
// 	fmt.Println("║  TEST: BLS Registration + Confirm                    ║")
// 	fmt.Println("╚══════════════════════════════════════════════════════╝")

// 	// === Step 1: Tạo private key mới ===
// 	fmt.Println("\n─── Step 1: Tạo ETH private key mới ───")
// 	newPrivKey, err := crypto.GenerateKey()
// 	if err != nil {
// 		fmt.Printf("  ❌ GenerateKey: %v\n", err)
// 		return
// 	}
// 	newAddr := crypto.PubkeyToAddress(newPrivKey.PublicKey)
// 	newPrivKeyHex := hex.EncodeToString(crypto.FromECDSA(newPrivKey))
// 	fmt.Printf("  ✅ New address:     %s\n", newAddr.Hex())
// 	fmt.Printf("  ✅ New private key: %s\n", newPrivKeyHex)

// 	// === Step 2: getPublickeyBls (eth_call đến RPC server) ===
// 	fmt.Println("\n─── Step 2: getPublickeyBls (eth_call) ───")
// 	getBlsData, err := accountABI.Pack("getPublickeyBls")
// 	if err != nil {
// 		fmt.Printf("  ❌ Pack getPublickeyBls: %v\n", err)
// 		return
// 	}
// 	blsResult, err := tcpClient.RpcEthCall(accountContract, getBlsData)
// 	if err != nil {
// 		fmt.Printf("  ❌ eth_call getPublickeyBls: %v\n", err)
// 		return
// 	}
// 	// Server trả JSON string (double-encoded): hex decode → "0x86d5..." (có quotes)
// 	// Strip quotes để lấy raw BLS pubkey hex
// 	serverBlsPubKey := strings.Trim(string(blsResult), "\"")
// 	// Decode hex → bytes để dùng cho setBlsPublicKey
// 	blsPubKeyHex := strings.TrimPrefix(serverBlsPubKey, "0x")
// 	blsPubKey, _ := hex.DecodeString(blsPubKeyHex)
// 	fmt.Printf("  ✅ Server BLS PublicKey: %s (%d bytes)\n", serverBlsPubKey, len(blsPubKey))

// 	// === Step 3: Subscribe RegisterBls + setBlsPublicKey ===
// 	fmt.Println("\n─── Step 3: Subscribe RegisterBls + setBlsPublicKey ───")

// 	// Subscribe RegisterBls event trước khi gửi tx
// 	registerBlsTopic := accountABI.Events["RegisterBls"].ID.Hex()
// 	fmt.Printf("  RegisterBls topic: %s\n", registerBlsTopic)

// 	var wgRegister sync.WaitGroup
// 	var onceRegister sync.Once
// 	wgRegister.Add(1)

// 	subBls, err := tcpClient.RpcSubscribe(
// 		[]string{accountContract.Hex()},
// 		[]string{registerBlsTopic},
// 		makeEventHandler("RegisterBls", &wgRegister, &onceRegister),
// 	)
// 	if err != nil {
// 		fmt.Printf("  ❌ Subscribe RegisterBls: %v\n", err)
// 	} else {
// 		fmt.Printf("  ✅ Subscribe RegisterBls: subID=%s\n", subBls)
// 	}

// 	// Gửi setBlsPublicKey với BLS key từ server (interceptor xử lý — không có receipt)
// 	nonce, _ := tcpClient.RpcGetPendingNonce(newAddr)
// 	inputData, _ := accountABI.Pack("setBlsPublicKey", blsPubKey)
// 	tx := e_types.NewTransaction(nonce, accountContract, big.NewInt(0), 20000000, big.NewInt(10000000), inputData)
// 	signedTx, _ := e_types.SignTx(tx, signer, newPrivKey)
// 	rawTxBytes, _ := signedTx.MarshalBinary()
// 	txHash, err := tcpClient.RpcSendRawTransaction("0x" + hex.EncodeToString(rawTxBytes))
// 	if err != nil {
// 		fmt.Printf("  ❌ setBlsPublicKey: %v\n", err)
// 	} else {
// 		fmt.Printf("  ✅ setBlsPublicKey sent: txHash=%s\n", txHash)
// 	}

// 	// Đợi RegisterBls event
// 	fmt.Println("  ⏳ Waiting for RegisterBls event (max 10s)...")
// 	regDone := make(chan struct{})
// 	go func() {
// 		wgRegister.Wait()
// 		close(regDone)
// 	}()
// 	select {
// 	case <-regDone:
// 		fmt.Println("  ✅ RegisterBls event received!")
// 	case <-time.After(10 * time.Second):
// 		fmt.Println("  ⚠️ Timeout waiting for RegisterBls event")
// 	}

// 	tcpClient.RpcUnsubscribe(subBls)

// 	// === Step 5: confirmAccountWithoutSign (admin confirm) ===
// 	fmt.Println("\n─── Step 5: confirmAccountWithoutSign (admin confirm) ───")
// 	fmt.Printf("  ℹ️  Admin %s confirming %s...\n", adminAddr.Hex(), newAddr.Hex())

// 	txHash2, receipt2 := sendTxAndWait(
// 		tcpClient, adminPrivKey, adminAddr, accountContract,
// 		accountABI, signer,
// 		"confirmAccountWithoutSign", newAddr,
// 	)
// 	if receipt2 != nil {
// 		fmt.Printf("  ✅ confirmAccountWithoutSign OK: txHash=%s\n", txHash2)
// 	} else {
// 		fmt.Println("  ⚠️ confirmAccountWithoutSign — receipt not available (may be intercepted)")
// 	}

// 	fmt.Println("\n  ✅ BLS Registration flow completed!")
// 	fmt.Printf("     New address:     %s\n", newAddr.Hex())
// 	fmt.Printf("     New private key: %s\n", newPrivKeyHex)
// 	fmt.Printf("     BLS PubKey:      0x%s\n", blsPubKeyHex)
// }

// // testBatchBlsRegistration đăng ký hàng loạt 1000 ví và lưu config
// func testBatchBlsRegistration(
// 	tcpClient *client_tcp.Client,
// 	accountABI abi.ABI,
// 	accountContract common.Address,
// 	adminPrivKey *ecdsa.PrivateKey,
// 	adminAddr common.Address,
// 	signer e_types.Signer,
// 	count int,
// 	baseCfg *tcp_config.ClientConfig,
// ) {
// 	fmt.Printf("\n🚀 BATCH BLS REGISTRATION: %d wallets\n", count)

// 	// 1. Lấy BLS PubKey từ server một lần duy nhất
// 	getBlsData, _ := accountABI.Pack("getPublickeyBls")
// 	blsResult, err := tcpClient.RpcEthCall(accountContract, getBlsData)
// 	if err != nil {
// 		fmt.Printf("  ❌ Không lấy được BLS PubKey: %v\n", err)
// 		return
// 	}
// 	serverBlsPubKey := strings.Trim(string(blsResult), "\"")
// 	blsPubKeyHex := strings.TrimPrefix(serverBlsPubKey, "0x")
// 	blsPubKey, _ := hex.DecodeString(blsPubKeyHex)
// 	fmt.Printf("  ✅ Server BLS PubKey: %s\n", serverBlsPubKey)

// 	// Tạo thư mục lưu wallets
// 	os.MkdirAll("wallets", 0755)

// 	// Lấy nonce admin hiện tại
// 	adminNonce, _ := tcpClient.RpcGetPendingNonce(adminAddr)

// 	// Danh sách chứa tất cả config
// 	allConfigs := make([]map[string]interface{}, 0, count)

// 	for i := 1; i <= count; i++ {
// 		// a. Tạo ETH key mới
// 		newPrivKey, _ := crypto.GenerateKey()
// 		newAddr := crypto.PubkeyToAddress(newPrivKey.PublicKey)
// 		newPrivKeyHex := hex.EncodeToString(crypto.FromECDSA(newPrivKey))

// 		// b. setBlsPublicKey (nonce = 0 vì ví mới)
// 		inputData, _ := accountABI.Pack("setBlsPublicKey", blsPubKey)
// 		tx := e_types.NewTransaction(0, accountContract, big.NewInt(0), 2000000, big.NewInt(1000000), inputData)
// 		signedTx, _ := e_types.SignTx(tx, signer, newPrivKey)
// 		rawTxBytes, _ := signedTx.MarshalBinary()
// 		_, err := tcpClient.RpcSendRawTransaction("0x" + hex.EncodeToString(rawTxBytes))
// 		if err != nil {
// 			fmt.Printf("  [%d/%d] ❌ setBlsPublicKey %s: %v\n", i, count, newAddr.Hex(), err)
// 			continue
// 		}

// 		// c. confirmAccountWithoutSign từ Admin
// 		confirmData, _ := accountABI.Pack("confirmAccountWithoutSign", newAddr)
// 		adminTx := e_types.NewTransaction(adminNonce, accountContract, big.NewInt(0), 2000000, big.NewInt(1000000), confirmData)
// 		signedAdminTx, _ := e_types.SignTx(adminTx, signer, adminPrivKey)
// 		adminRawTxBytes, _ := signedAdminTx.MarshalBinary()
// 		_, err = tcpClient.RpcSendRawTransaction("0x" + hex.EncodeToString(adminRawTxBytes))
// 		if err != nil {
// 			fmt.Printf("  [%d/%d] ❌ AdminConfirm %s: %v\n", i, count, newAddr.Hex(), err)
// 			continue
// 		}
// 		adminNonce++

// 		// d. Gom config vào mảng
// 		walletCfg := map[string]interface{}{
// 			"private_key":               baseCfg.PrivateKey_, // Cùng BLS key với server
// 			"version":                   baseCfg.Version_,
// 			"parent_connection_address": baseCfg.ParentConnectionAddress,
// 			"chain_id":                  baseCfg.ChainId,
// 			"nation_id":                 baseCfg.NationId,
// 			"parent_connection_type":    baseCfg.ParentConnectionType,
// 			"parent_address":            newAddr.Hex(), // Mỗi ví dùng địa chỉ riêng của mình làm ParentAddress (Identity)
// 			"eth_private_key":           newPrivKeyHex,
// 			"demo_abi_path":             baseCfg.DemoAbiPath_,
// 			"demo_contract_address":     baseCfg.DemoContractAddress,
// 			"contact_private":           "0x00000000000000000000000000000000D844bb55",
// 		}
// 		allConfigs = append(allConfigs, walletCfg)

// 		if i%10 == 0 || i == count {
// 			fmt.Printf("  ✅ [%d/%d] Registered: %s\n", i, count, newAddr.Hex())
// 		}
// 	}

// 	// Viết tất cả vào 1 file duy nhất
// 	finalCtx, _ := json.MarshalIndent(allConfigs, "", "  ")
// 	os.WriteFile("wallets.json", finalCtx, 0644)

// 	fmt.Printf("\n✨ Done! %d configs saved in wallets.json\n", len(allConfigs))
// }

// // ===================== MAIN =====================

// func main() {
// 	logger.SetConfig(&logger.LoggerConfig{
// 		Flag:    logger.FLAG_INFO,
// 		Outputs: []*os.File{os.Stdout},
// 	})

// 	configPath := flag.String("config", "config-test.json", "Path to TCP client config")
// 	testSuite := flag.String("test", "bls", "Test suite: demo, bls, tps, tps-sendtx, tps-single, all, batch-bls")
// 	tpsCount := flag.Int("n", 1000, "Number of requests for TPS test / wallets for batch-bls")
// 	tpsConcurrency := flag.Int("c", 50, "Concurrency for TPS test")
// 	tpsRounds := flag.Int("rounds", 3, "Number of TPS benchmark rounds")
// 	tpsPause := flag.Int("pause", 5, "Seconds to pause between rounds (for chain to settle)")
// 	tpsMode := flag.String("mode", "tcp", "Transport mode for tps-single: tcp or http")
// 	tpsWallets := flag.Int("wallets", 200, "Number of wallets for tps-single")
// 	tpsTxPerWallet := flag.Int("txpw", 5, "Transactions per wallet for tps-single")
// 	flag.Parse()

// 	fmt.Println("╔══════════════════════════════════════════════════════╗")
// 	fmt.Println("║       TCP-RPC Test Suite                             ║")
// 	fmt.Println("╚══════════════════════════════════════════════════════╝")

// 	cfgRaw, _ := tcp_config.LoadConfig(*configPath)
// 	cfg := cfgRaw.(*tcp_config.ClientConfig)

// 	// Khởi tạo client
// 	tcpClient, err := client_tcp.NewClient(cfg)
// 	if err != nil {
// 		logger.Error("Failed to create TCP client: %v", err)
// 		os.Exit(1)
// 	}
// 	time.Sleep(1 * time.Second)

// 	// Common setup
// 	ethPrivKey, _ := crypto.HexToECDSA(cfg.EthPrivateKey)
// 	fromAddr := crypto.PubkeyToAddress(ethPrivKey.PublicKey)
// 	chainIdBig := big.NewInt(int64(cfg.ChainId))
// 	signer := e_types.NewEIP155Signer(chainIdBig)

// 	fmt.Printf("\n  Admin address: %s\n", fromAddr.Hex())
// 	fmt.Printf("  Chain ID: %d\n", cfg.ChainId)

// 	// === Test 0: Basic RPC ===
// 	fmt.Println("\n─── Basic: RpcGetChainId ───")
// 	// chainId, _ := tcpClient.RpcGetChainId()
// 	// fmt.Printf("  ✅ ChainId: %s\n", chainId)
// 	// === Run test suites ===
// 	switch *testSuite {
// 	case "demo":
// 		// Load demo ABI
// 		demoAbiBytes, _ := os.ReadFile(cfg.DemoAbiPath_)
// 		demoABI, _ := abi.JSON(strings.NewReader(string(demoAbiBytes)))
// 		contractAddr := common.HexToAddress(cfg.DemoContractAddress)
// 		testDemoContract(tcpClient, demoABI, contractAddr, ethPrivKey, fromAddr, signer)
// 	case "bls":
// 		// Load account ABI
// 		accountABI, _ := abi.JSON(strings.NewReader(accountAbiJSON))
// 		accountContract := common.HexToAddress("0x00000000000000000000000000000000D844bb55")
// 		testBlsRegistration(tcpClient, accountABI, accountContract, ethPrivKey, fromAddr, signer)
// 	case "tps":
// 		testChainIdTPS(tcpClient, *tpsCount, *tpsConcurrency)
// 	case "batch-bls":
// 		// Load account ABI
// 		accountABI, _ := abi.JSON(strings.NewReader(accountAbiJSON))
// 		accountContract := common.HexToAddress("0x00000000000000000000000000000000D844bb55")
// 		testBatchBlsRegistration(tcpClient, accountABI, accountContract, ethPrivKey, fromAddr, signer, *tpsCount, cfg)
// 	case "tps-single":
// 		testSingleTPS(tcpClient, signer, cfg, *tpsMode, *tpsWallets, *tpsTxPerWallet, *tpsRounds, *tpsPause)
// 	case "all":
// 		// Demo test
// 		demoAbiBytes, _ := os.ReadFile(cfg.DemoAbiPath_)
// 		demoABI, _ := abi.JSON(strings.NewReader(string(demoAbiBytes)))
// 		contractAddr := common.HexToAddress(cfg.DemoContractAddress)
// 		testDemoContract(tcpClient, demoABI, contractAddr, ethPrivKey, fromAddr, signer)

// 		// BLS test
// 		accountABI, _ := abi.JSON(strings.NewReader(accountAbiJSON))
// 		accountContract := common.HexToAddress("0x00000000000000000000000000000000D844bb55")
// 		testBlsRegistration(tcpClient, accountABI, accountContract, ethPrivKey, fromAddr, signer)
// 	}

// 	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
// 	fmt.Println("║       All tests completed!                           ║")
// 	fmt.Println("╚══════════════════════════════════════════════════════╝")
// }

// // testChainIdTPS benchmark TPS cho RpcGetChainId qua TCP
// func testChainIdTPS(tcpClient *client_tcp.Client, totalRequests, concurrency int) {
// 	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
// 	fmt.Println("║  TPS Benchmark: GetChainId via TCP                    ║")
// 	fmt.Println("╚══════════════════════════════════════════════════════╝")
// 	fmt.Printf("  📋 Total requests: %d, Concurrency: %d\n\n", totalRequests, concurrency)

// 	// Warm up
// 	fmt.Println("  🔥 Warming up (10 requests)...")
// 	for i := 0; i < 10; i++ {
// 		tcpClient.RpcGetChainId()
// 	}
// 	fmt.Println("  ✅ Warm-up done")

// 	// Benchmark
// 	fmt.Printf("\n─── eth_getChainId via TCP ───\n")
// 	okCount, failCount, duration := runBenchmark(totalRequests, concurrency, func() error {
// 		_, err := tcpClient.RpcGetChainId()
// 		return err
// 	})
// 	tps := float64(okCount) / duration.Seconds()
// 	avgLatency := duration / time.Duration(totalRequests)
// 	fmt.Printf("  ✅ OK: %d, ❌ Fail: %d\n", okCount, failCount)
// 	fmt.Printf("  ⏱️  Duration: %s\n", duration.Round(time.Millisecond))
// 	fmt.Printf("  📊 TPS: %.2f req/s\n", tps)
// 	fmt.Printf("  📊 Avg latency: %s\n", avgLatency.Round(time.Microsecond))
// }

// // runBenchmark chạy benchmark concurrent
// func runBenchmark(total, concurrency int, fn func() error) (okCount int64, failCount int64, duration time.Duration) {
// 	var ok, fail int64
// 	sem := make(chan struct{}, concurrency)
// 	var wg sync.WaitGroup

// 	start := time.Now()
// 	for i := 0; i < total; i++ {
// 		wg.Add(1)
// 		sem <- struct{}{}
// 		go func() {
// 			defer wg.Done()
// 			defer func() { <-sem }()
// 			if err := fn(); err != nil {
// 				atomic.AddInt64(&fail, 1)
// 			} else {
// 				atomic.AddInt64(&ok, 1)
// 			}
// 		}()
// 	}
// 	wg.Wait()
// 	elapsed := time.Since(start)
// 	return ok, fail, elapsed
// }

// // ===================== TPS SendTx =====================

// // walletEntry cấu trúc wallet từ wallets.json
// type walletEntry struct {
// 	EthPrivateKey string `json:"eth_private_key"`
// 	ParentAddress string `json:"parent_address"`
// 	ChainId       uint64 `json:"chain_id"`
// }

// // prebuiltTx chứa signed raw tx hex, sẵn sàng bắn
// type prebuiltTx struct {
// 	RawTxHex string
// 	FromAddr common.Address
// }

// // roundResult lưu kết quả 1 round
// type roundResult struct {
// 	TcpTPS   float64
// 	HttpTPS  float64
// 	TcpDur   time.Duration
// 	HttpDur  time.Duration
// 	TcpOK    int64
// 	HttpOK   int64
// 	TcpFail  int64
// 	HttpFail int64
// }

// // testSingleTPS test chỉ 1 transport (TCP hoặc HTTP) với multi-round
// // Công bằng: chỉ test 1 loại, không bị ảnh hưởng bởi transport kia
// func testSingleTPS(
// 	tcpClient *client_tcp.Client,
// 	signer e_types.Signer,
// 	cfg *tcp_config.ClientConfig,
// 	mode string,
// 	numWallets int,
// 	txPerWallet int,
// 	rounds int,
// 	pauseSec int,
// ) {
// 	useHTTP := strings.ToLower(mode) == "http"
// 	label := "TCP-Direct"
// 	if useHTTP {
// 		label = "HTTP-Forward"
// 	}
// 	totalTxs := numWallets * txPerWallet

// 	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
// 	fmt.Printf("║  TPS Benchmark: %s ONLY                          \n", strings.ToUpper(label))
// 	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
// 	fmt.Printf("║  Wallets: %d | TX/wallet: %d | Total TX: %d\n", numWallets, txPerWallet, totalTxs)
// 	fmt.Printf("║  Rounds: %d | Pause: %ds\n", rounds, pauseSec)
// 	fmt.Println("╚══════════════════════════════════════════════════════════════╝")

// 	// Load wallets
// 	fmt.Println("\n─── Loading wallets ───")
// 	walletData, err := os.ReadFile("wallets.json")
// 	if err != nil {
// 		fmt.Printf("  ❌ Cannot read wallets.json: %v\n", err)
// 		fmt.Println("  ℹ️  Run with -test=batch-bls first to generate wallets")
// 		return
// 	}
// 	var wallets []walletEntry
// 	if err := json.Unmarshal(walletData, &wallets); err != nil {
// 		fmt.Printf("  ❌ Cannot parse wallets.json: %v\n", err)
// 		return
// 	}

// 	// Lấy đúng numWallets ví
// 	if numWallets > len(wallets) {
// 		numWallets = len(wallets)
// 		totalTxs = numWallets * txPerWallet
// 		fmt.Printf("  ⚠️ Only %d wallets available, adjusting\n", numWallets)
// 	}

// 	privKeys := make([]*ecdsa.PrivateKey, 0, numWallets)
// 	addrs := make([]common.Address, 0, numWallets)
// 	for _, w := range wallets[:numWallets] {
// 		pk, err := crypto.HexToECDSA(w.EthPrivateKey)
// 		if err != nil {
// 			continue
// 		}
// 		addr := crypto.PubkeyToAddress(pk.PublicKey)
// 		privKeys = append(privKeys, pk)
// 		addrs = append(addrs, addr)
// 	}
// 	fmt.Printf("  ✅ Using %d wallets, %d tx/wallet = %d total txs\n", len(privKeys), txPerWallet, len(privKeys)*txPerWallet)

// 	// Run rounds
// 	type singleResult struct {
// 		TPS      float64
// 		Duration time.Duration
// 		OK       int64
// 		Fail     int64
// 	}
// 	results := make([]singleResult, 0, rounds)

// 	for round := 1; round <= rounds; round++ {
// 		fmt.Printf("\n╔═══════════════════════════════════════╗\n")
// 		fmt.Printf("║     ROUND %d / %d (%s)            \n", round, rounds, label)
// 		fmt.Printf("╚═══════════════════════════════════════╝\n")

// 		// Fetch nonces
// 		nonces := fetchNoncesForKeys(tcpClient, privKeys, addrs)

// 		// Build txs grouped per wallet
// 		txsPerWallet := buildTxsGroupedByWallet(privKeys, addrs, nonces, signer, txPerWallet)
// 		totalTxCount := 0
// 		for _, wTxs := range txsPerWallet {
// 			totalTxCount += len(wTxs)
// 		}
// 		fmt.Printf("  📋 Built %d txs (%d wallets × %d tx/wallet)\n", totalTxCount, len(privKeys), txPerWallet)

// 		// Fire! Mỗi wallet gửi tuần tự, nhưng 200 wallets song song
// 		fmt.Printf("  🚀 Firing %s (per-wallet sequential)...\n", label)
// 		ok, fail, dur := fireTxsPerWallet(tcpClient, txsPerWallet, useHTTP)
// 		tps := float64(ok) / dur.Seconds()
// 		avgLat := dur / time.Duration(totalTxCount)

// 		fmt.Printf("  ✅ OK: %d, ❌ Fail: %d\n", ok, fail)
// 		fmt.Printf("  ⏱️  Duration: %s\n", dur.Round(time.Millisecond))
// 		fmt.Printf("  📊 TPS: %.2f tx/s\n", tps)
// 		fmt.Printf("  📊 Avg latency: %s\n", avgLat.Round(time.Microsecond))

// 		results = append(results, singleResult{
// 			TPS:      tps,
// 			Duration: dur,
// 			OK:       ok,
// 			Fail:     fail,
// 		})

// 		// Pause between rounds
// 		if round < rounds && pauseSec > 0 {
// 			fmt.Printf("\n  ⏳ Waiting %ds for chain to settle...\n", pauseSec)
// 			time.Sleep(time.Duration(pauseSec) * time.Second)
// 		}
// 	}

// 	// ==================== Aggregate ====================
// 	var sum, min, max float64
// 	min = 1e18
// 	var totalOK, totalFail int64
// 	for _, r := range results {
// 		sum += r.TPS
// 		if r.TPS < min {
// 			min = r.TPS
// 		}
// 		if r.TPS > max {
// 			max = r.TPS
// 		}
// 		totalOK += r.OK
// 		totalFail += r.Fail
// 	}
// 	avg := sum / float64(len(results))

// 	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
// 	fmt.Printf("║  AGGREGATE: %s (%d rounds)                         \n", strings.ToUpper(label), rounds)
// 	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
// 	fmt.Printf("║  Config:  %d wallets × %d tx/wallet = %d total/round\n", len(privKeys), txPerWallet, len(privKeys)*txPerWallet)
// 	fmt.Println("║  ─────────────────────────────────────────────────")
// 	fmt.Printf("║  Avg TPS:     %8.0f tx/s\n", avg)
// 	fmt.Printf("║  Min TPS:     %8.0f tx/s\n", min)
// 	fmt.Printf("║  Max TPS:     %8.0f tx/s\n", max)
// 	fmt.Printf("║  Total OK:    %8d / %d\n", totalOK, totalOK+totalFail)
// 	fmt.Println("║  ─────────────────────────────────────────────────")
// 	fmt.Println("║  Round │     TPS      │  Duration  │  OK/Total")
// 	for i, r := range results {
// 		fmt.Printf("║  %5d │ %8.0f tx/s │ %9s  │  %d/%d\n",
// 			i+1, r.TPS, r.Duration.Round(time.Millisecond), r.OK, r.OK+r.Fail)
// 	}
// 	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
// }

// // fetchNoncesForKeys lấy nonce cho tất cả wallets song song
// func fetchNoncesForKeys(
// 	tcpClient *client_tcp.Client,
// 	privKeys []*ecdsa.PrivateKey,
// 	addrs []common.Address,
// ) []uint64 {
// 	nonces := make([]uint64, len(addrs))
// 	var wg sync.WaitGroup
// 	sem := make(chan struct{}, 100) // max 100 concurrent nonce fetches

// 	for i := range addrs {
// 		wg.Add(1)
// 		sem <- struct{}{}
// 		go func(idx int) {
// 			defer wg.Done()
// 			defer func() { <-sem }()
// 			nonce, err := tcpClient.RpcGetPendingNonce(addrs[idx])
// 			if err != nil {
// 				logger.Warn("GetNonce failed for wallet %d (%s): %v", idx, addrs[idx].Hex(), err)
// 				return
// 			}
// 			nonces[idx] = nonce
// 		}(i)
// 	}
// 	wg.Wait()
// 	return nonces
// }

// // buildSignedTxsBatch tạo signed raw tx hex cho mỗi wallet (1 tx/wallet)
// func buildSignedTxsBatch(
// 	privKeys []*ecdsa.PrivateKey,
// 	addrs []common.Address,
// 	nonces []uint64,
// 	signer e_types.Signer,
// ) []prebuiltTx {
// 	return buildSignedTxsMulti(privKeys, addrs, nonces, signer, 1)
// }

// // buildSignedTxsMulti tạo signed raw tx hex cho mỗi wallet với txPerWallet giao dịch
// // Mỗi tx dùng nonce tăng dần: nonces[i], nonces[i]+1, ..., nonces[i]+txPerWallet-1
// func buildSignedTxsMulti(
// 	privKeys []*ecdsa.PrivateKey,
// 	addrs []common.Address,
// 	nonces []uint64,
// 	signer e_types.Signer,
// 	txPerWallet int,
// ) []prebuiltTx {
// 	txs := make([]prebuiltTx, 0, len(privKeys)*txPerWallet)
// 	for i, pk := range privKeys {
// 		for j := 0; j < txPerWallet; j++ {
// 			nonce := nonces[i] + uint64(j)
// 			tx := e_types.NewTransaction(nonce, common.HexToAddress("0x2C71210D239D472e963a7Be8362eCBdeD5337fE6"), big.NewInt(0), 21000, big.NewInt(1000000), nil)
// 			signedTx, err := e_types.SignTx(tx, signer, pk)
// 			if err != nil {
// 				fmt.Printf("  ⚠️ SignTx failed for wallet %d tx %d: %v\n", i, j, err)
// 				continue
// 			}
// 			rawTxBytes, _ := signedTx.MarshalBinary()
// 			txs = append(txs, prebuiltTx{
// 				RawTxHex: "0x" + hex.EncodeToString(rawTxBytes),
// 				FromAddr: addrs[i],
// 			})
// 		}
// 	}
// 	return txs
// }

// // buildTxsGroupedByWallet tạo txs nhóm theo wallet (mỗi wallet = []prebuiltTx)
// // Để có thể gửi tuần tự per-wallet, song song giữa các wallet
// func buildTxsGroupedByWallet(
// 	privKeys []*ecdsa.PrivateKey,
// 	addrs []common.Address,
// 	nonces []uint64,
// 	signer e_types.Signer,
// 	txPerWallet int,
// ) [][]prebuiltTx {
// 	result := make([][]prebuiltTx, len(privKeys))
// 	for i, pk := range privKeys {
// 		walletTxs := make([]prebuiltTx, 0, txPerWallet)
// 		for j := 0; j < txPerWallet; j++ {
// 			nonce := nonces[i] + uint64(j)
// 			tx := e_types.NewTransaction(nonce, common.HexToAddress("0x2C71210D239D472e963a7Be8362eCBdeD5337fE6"), big.NewInt(0), 21000, big.NewInt(1000000), nil)
// 			signedTx, err := e_types.SignTx(tx, signer, pk)
// 			if err != nil {
// 				continue
// 			}
// 			rawTxBytes, _ := signedTx.MarshalBinary()
// 			walletTxs = append(walletTxs, prebuiltTx{
// 				RawTxHex: "0x" + hex.EncodeToString(rawTxBytes),
// 				FromAddr: addrs[i],
// 			})
// 		}
// 		result[i] = walletTxs
// 	}
// 	return result
// }

// // fireTxsPerWallet gửi txs per-wallet: mỗi wallet gửi TUẦN TỰ (đảm bảo thứ tự nonce),
// // nhưng 200 wallets chạy SONG SONG → tối đa throughput mà không lỗi nonce
// func fireTxsPerWallet(
// 	tcpClient *client_tcp.Client,
// 	txsPerWallet [][]prebuiltTx,
// 	useHTTP bool,
// ) (okCount int64, failCount int64, duration time.Duration) {
// 	var ok, fail int64
// 	var wg sync.WaitGroup

// 	const maxErrSamples = 10
// 	var errMu sync.Mutex
// 	errSamples := make([]string, 0, maxErrSamples)

// 	start := time.Now()
// 	for walletIdx := range txsPerWallet {
// 		wg.Add(1)
// 		go func(wIdx int) {
// 			defer wg.Done()
// 			// Gửi tuần tự cho wallet này
// 			for txIdx, ptx := range txsPerWallet[wIdx] {
// 				var err error
// 				if useHTTP {
// 					_, err = tcpClient.RpcHttpSendRawTransaction(ptx.RawTxHex)
// 				} else {
// 					_, err = tcpClient.RpcSendRawTransaction(ptx.RawTxHex)
// 				}
// 				if err != nil {
// 					n := atomic.AddInt64(&fail, 1)
// 					if n <= int64(maxErrSamples) {
// 						errMu.Lock()
// 						errSamples = append(errSamples, fmt.Sprintf("wallet[%d] tx[%d] %s: %v", wIdx, txIdx, ptx.FromAddr.Hex()[:10], err))
// 						errMu.Unlock()
// 					}
// 					// Nếu 1 tx fail (nonce lỗi), skip các tx còn lại của wallet này
// 					remaining := len(txsPerWallet[wIdx]) - txIdx - 1
// 					atomic.AddInt64(&fail, int64(remaining))
// 					break
// 				}
// 				atomic.AddInt64(&ok, 1)
// 			}
// 		}(walletIdx)
// 	}
// 	wg.Wait()
// 	elapsed := time.Since(start)

// 	finalFail := atomic.LoadInt64(&fail)
// 	if finalFail > 0 {
// 		fmt.Printf("\n  ⚠️  %d errors detected (showing first %d):\n", finalFail, len(errSamples))
// 		for _, msg := range errSamples {
// 			fmt.Printf("    ❌ %s\n", msg)
// 		}
// 	}

// 	return atomic.LoadInt64(&ok), finalFail, elapsed
// }

// // fireTxsBatch gửi tất cả txs song song, đo thời gian
// // useHTTP=true → http_sendRawTransaction, false → eth_sendRawTransaction
// func fireTxsBatch(
// 	tcpClient *client_tcp.Client,
// 	txs []prebuiltTx,
// 	useHTTP bool,
// ) (okCount int64, failCount int64, duration time.Duration) {
// 	var ok, fail int64
// 	var wg sync.WaitGroup

// 	// Collect first N error messages (thread-safe)
// 	const maxErrSamples = 10
// 	var errMu sync.Mutex
// 	errSamples := make([]string, 0, maxErrSamples)

// 	start := time.Now()
// 	for i := range txs {
// 		wg.Add(1)
// 		go func(idx int) {
// 			defer wg.Done()
// 			var err error
// 			if useHTTP {
// 				_, err = tcpClient.RpcHttpSendRawTransaction(txs[idx].RawTxHex)
// 			} else {
// 				_, err = tcpClient.RpcSendRawTransaction(txs[idx].RawTxHex)
// 			}
// 			if err != nil {
// 				n := atomic.AddInt64(&fail, 1)
// 				if n <= int64(maxErrSamples) {
// 					errMu.Lock()
// 					errSamples = append(errSamples, fmt.Sprintf("[%d] %s: %v", idx, txs[idx].FromAddr.Hex()[:10], err))
// 					errMu.Unlock()
// 				}
// 			} else {
// 				atomic.AddInt64(&ok, 1)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// 	elapsed := time.Since(start)

// 	// Print error summary
// 	finalFail := atomic.LoadInt64(&fail)
// 	if finalFail > 0 {
// 		fmt.Printf("\n  ⚠️  %d errors detected (showing first %d):\n", finalFail, len(errSamples))
// 		for _, msg := range errSamples {
// 			fmt.Printf("    ❌ %s\n", msg)
// 		}
// 	}

// 	return atomic.LoadInt64(&ok), finalFail, elapsed
// }

// // Account ABI (chỉ các function cần dùng)
// const accountAbiJSON = `[
// 	{
// 		"anonymous": false,
// 		"inputs": [
// 			{"indexed": false, "internalType": "address", "name": "account", "type": "address"},
// 			{"indexed": false, "internalType": "uint256", "name": "time", "type": "uint256"},
// 			{"indexed": false, "internalType": "bytes", "name": "publicKey", "type": "bytes"},
// 			{"indexed": false, "internalType": "string", "name": "message", "type": "string"}
// 		],
// 		"name": "RegisterBls",
// 		"type": "event"
// 	},
// 	{
// 		"anonymous": false,
// 		"inputs": [
// 			{"indexed": false, "internalType": "address", "name": "account", "type": "address"},
// 			{"indexed": false, "internalType": "uint256", "name": "time", "type": "uint256"},
// 			{"indexed": false, "internalType": "string", "name": "message", "type": "string"}
// 		],
// 		"name": "AccountConfirmed",
// 		"type": "event"
// 	},
// 	{
// 		"inputs": [{"internalType":"bytes","name":"_publicKey","type":"bytes"}],
// 		"name": "setBlsPublicKey",
// 		"outputs": [],
// 		"stateMutability": "nonpayable",
// 		"type": "function"
// 	},
// 	{
// 		"inputs": [{"internalType":"address","name":"_account","type":"address"}],
// 		"name": "confirmAccountWithoutSign",
// 		"outputs": [],
// 		"stateMutability": "nonpayable",
// 		"type": "function"
// 	},
// 	{
// 		"inputs": [],
// 		"name": "getPublickeyBls",
// 		"outputs": [],
// 		"stateMutability": "nonpayable",
// 		"type": "function"
// 	}
// ]`
