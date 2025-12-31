import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useRef,
} from "react";
import {
  createWalletClient,
  createPublicClient,
  http,
  encodeFunctionData,
  decodeEventLog,
  keccak256,
  toHex,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { contracts } from "~/constants/contracts";
import { chain991, WSS_RPC, WSS_RPC_INTERCEPTOR, GO_BACKEND_RPC_URL } from "~/constants/customeChain";

// Event types (using JSDoc for documentation)
/**
 * @typedef {Object} RobotEvent
 * @property {string} id
 * @property {"SessionCreated" | "SentenceEmitted" | "AIRequest"} eventName
 * @property {bigint} sessionId
 * @property {bigint} timestamp
 * @property {Object} data
 * @property {string} [data.robot]
 * @property {bigint} [data.sentenceIndex]
 * @property {string} [data.sentence]
 * @property {string} [data.requestData]
 */

const RobotContext = createContext(undefined);

export function RobotProvider({ children }) {
  const [account, setAccount] = useState(null);
  const [chainEvents, setChainEvents] = useState([]); // Events từ chain
  const [interceptorEvents, setInterceptorEvents] = useState([]); // Events từ interceptor
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);

  // Wallet client refs
  const walletClientRef = useRef(null);
  const publicClientRef = useRef(null);

  // WebSocket refs - 2 connections riêng biệt
  const wsChainRef = useRef(null); // WebSocket cho chain (không interceptor)
  const wsInterceptorRef = useRef(null); // WebSocket cho interceptor
  const reconnectChainTimeoutRef = useRef(null);
  const reconnectInterceptorTimeoutRef = useRef(null);

  // Connect wallet from private key
  const connectWallet = useCallback((privateKey) => {
    try {
      // Remove 0x prefix if present
      const cleanKey = privateKey.startsWith("0x") ? privateKey.slice(2) : privateKey;
      const privateKeyHex = `0x${cleanKey}`;

      // Create account from private key
      const account = privateKeyToAccount(privateKeyHex);

      // Create wallet client
      const walletClient = createWalletClient({
        account,
        chain: chain991,
        transport: http(GO_BACKEND_RPC_URL),
      });

      // Create public client
      const publicClient = createPublicClient({
        chain: chain991,
        transport: http(GO_BACKEND_RPC_URL),
      });

      walletClientRef.current = walletClient;
      publicClientRef.current = publicClient;
      setAccount(account.address);
      setError(null);
    } catch (err) {
      console.error("Error connecting wallet:", err);
      setError(err instanceof Error ? err.message : "Failed to connect wallet");
    }
  }, []);

  // Disconnect wallet
  const disconnectWallet = useCallback(() => {
    walletClientRef.current = null;
    publicClientRef.current = null;
    setAccount(null);
    if (wsChainRef.current) {
      wsChainRef.current.close();
      wsChainRef.current = null;
    }
    if (wsInterceptorRef.current) {
      wsInterceptorRef.current.close();
      wsInterceptorRef.current = null;
    }
    if (reconnectChainTimeoutRef.current) {
      clearTimeout(reconnectChainTimeoutRef.current);
      reconnectChainTimeoutRef.current = null;
    }
    if (reconnectInterceptorTimeoutRef.current) {
      clearTimeout(reconnectInterceptorTimeoutRef.current);
      reconnectInterceptorTimeoutRef.current = null;
    }
  }, []);

  // Create session
  const createSession = useCallback(
    async (sessionId, robotAddress, requestData) => {
      if (!walletClientRef.current || !publicClientRef.current) {
        throw new Error("Wallet not connected");
      }

      setIsLoading(true);
      setError(null);

      try {
        const data = encodeFunctionData({
          abi: contracts.RobotManager.abi,
          functionName: "createSession",
          args: [sessionId, robotAddress, toHex(requestData)],
        });

        const hash = await walletClientRef.current.sendTransaction({
          to: contracts.RobotManager.address,
          data,
        });

        // Wait for transaction receipt
        // await publicClientRef.current.waitForTransactionReceipt({ hash });

        return hash;
      } catch (err) {
        const errorMsg =
          err instanceof Error ? err.message : "Failed to create session";
        setError(errorMsg);
        throw new Error(errorMsg);
      } finally {
        setIsLoading(false);
      }
    },
    []
  );

  // Emit sentence
  const emitSentence = useCallback(
    async (sessionId, sentenceIndex, sentence) => {
      // Validate inputs
      if (!walletClientRef.current || !publicClientRef.current) {
        throw new Error("Wallet not connected");
      }

      if (sessionId === null || sessionId === undefined) {
        console.error("❌ [emitSentence] Session ID is null/undefined");
        throw new Error("Session ID is required");
      }
      if (sentenceIndex === null || sentenceIndex === undefined) {
        console.error("❌ [emitSentence] Sentence Index is null/undefined");
        throw new Error("Sentence Index is required");
      }
      if (sentence === null || sentence === undefined || sentence.trim() === "") {
        console.error("❌ [emitSentence] Sentence is null/undefined/empty");
        throw new Error("Sentence is required");
      }

      setIsLoading(true);
      setError(null);
      
      try {
        console.log(`🔵 [emitSentence] Encoding function data: sessionId=${sessionId}, sentenceIndex=${sentenceIndex}, sentence=${sentence}`);
        
        const data = encodeFunctionData({
          abi: contracts.RobotManager.abi,
          functionName: "emitSentence",
          args: [sessionId, sentenceIndex, sentence],
        });

        console.log(`🔵 [emitSentence] Sending index ${sentenceIndex} transaction with data: ${data}`);

        // Wrap sendTransaction để handle response tốt hơn
        let hash;
        try {
          hash = await walletClientRef.current.sendTransaction({
            to: contracts.RobotManager.address,
            data,
          });
          console.log(`✅ [emitSentence] index ${sentenceIndex} transaction sent successfully: hash=${hash}`);
        } catch (sendErr) {
          // Nếu lỗi là "Cannot convert null to a BigInt", có thể là do response parsing
          // Thử log chi tiết hơn
          console.error(`❌ [emitSentence] sendTransaction error at index ${sentenceIndex}:`, sendErr);
          console.error(`❌ [emitSentence] Error details:`, {
            name: sendErr?.name,
            message: sendErr?.message,
            cause: sendErr?.cause,
            stack: sendErr?.stack,
          });
          
          // Nếu error có request/response info, log ra
          if (sendErr?.request) {
            console.error(`❌ [emitSentence] Request that failed:`, sendErr.request);
          }
          if (sendErr?.response) {
            console.error(`❌ [emitSentence] Response that failed:`, sendErr.response);
          }
          
          throw sendErr;
        }

        // Wait for transaction receipt
        // await publicClientRef.current.waitForTransactionReceipt({ hash });

        return hash;
      } catch (err) {
        const errorMsg =
          err instanceof Error ? err.message : "Failed to emit sentence";
        console.error(`❌ [emitSentence] Error index ${sentenceIndex}:`, err);
        console.error(`❌ [emitSentence] Error message index ${sentenceIndex}:`, errorMsg);
        console.error(`❌ [emitSentence] Error stack index ${sentenceIndex}:`, err instanceof Error ? err.stack : "No stack");
        setError(errorMsg);
        throw new Error(errorMsg);
      } finally {
        setIsLoading(false);
      }
    },
    []
  );

  // Clear events
  const clearChainEvents = useCallback(() => {
    setChainEvents([]);
  }, []);

  const clearInterceptorEvents = useCallback(() => {
    setInterceptorEvents([]);
  }, []);

  const clearAllEvents = useCallback(() => {
    setChainEvents([]);
    setInterceptorEvents([]);
  }, []);

  // Helper function to subscribe to events
  const subscribeToEvents = (ws, source) => {
    // Calculate event topic hashes
    const sigSessionCreated = "SessionCreated(uint256,address,uint256)";
    const sigSentenceEmitted = "SentenceEmitted(uint256,uint256,string,uint256)";
    const sigAIRequest = "AIRequest(uint256,bytes,uint256)";

    const topicSessionCreated = keccak256(toHex(sigSessionCreated));
    const topicSentenceEmitted = keccak256(toHex(sigSentenceEmitted));
    const topicAIRequest = keccak256(toHex(sigAIRequest));

    // Subscribe to SessionCreated events
    ws.send(
      JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "eth_subscribe",
        params: [
          "logs",
          {
            address: contracts.RobotManager.address,
            topics: [[topicSessionCreated]],
          },
        ],
      })
    );

    // Subscribe to SentenceEmitted events
    ws.send(
      JSON.stringify({
        jsonrpc: "2.0",
        id: 2,
        method: "eth_subscribe",
        params: [
          "logs",
          {
            address: contracts.RobotManager.address,
            topics: [[topicSentenceEmitted]],
          },
        ],
      })
    );

    // Subscribe to AIRequest events
    ws.send(
      JSON.stringify({
        jsonrpc: "2.0",
        id: 3,
        method: "eth_subscribe",
        params: [
          "logs",
          {
            address: contracts.RobotManager.address,
            topics: [[topicAIRequest]],
          },
        ],
      })
    );
  };

  // Helper function to handle WebSocket messages
  const handleWebSocketMessage = (event, source) => {
    try {
      const data = JSON.parse(event.data);

      if (data.method === "eth_subscription" && data.params?.result) {
        const log = data.params.result;

        if (log.topics && log.data) {
          try {
            // Decode event log
            const decoded = decodeEventLog({
              abi: contracts.RobotManager.abi,
              data: log.data,
              topics: log.topics,
            });

            const args = decoded.args || {};

            // Create event object
            const robotEvent = {
              id: `${log.transactionHash}-${log.logIndex}-${source}`,
              eventName: decoded.eventName,
              sessionId: args.sessionId || BigInt(0),
              timestamp: args.timestamp || BigInt(0),
              source: source, // "chain" or "interceptor"
              data: {},
            };

            // Extract event-specific data
            if (decoded.eventName === "SessionCreated") {
              robotEvent.data.robot = args.robot;
            } else if (decoded.eventName === "SentenceEmitted") {
              robotEvent.data.sentenceIndex = args.sentenceIndex;
              robotEvent.data.sentence = args.sentence;
            } else if (decoded.eventName === "AIRequest") {
              robotEvent.data.requestData = args.requestData;
            }

            // Add to appropriate events list
            if (source === "chain") {
              setChainEvents((prev) => [robotEvent, ...prev]);
            } else {
              setInterceptorEvents((prev) => [robotEvent, ...prev]);
            }

            console.log(`🔔 Robot event received from ${source}:`, robotEvent);
          } catch (decodeErr) {
            console.error(`❌ Error decoding event from ${source}:`, decodeErr);
          }
        }
      }
    } catch (err) {
      console.error(`❌ Error handling WebSocket message from ${source}:`, err);
    }
  };

  // WebSocket subscription for CHAIN events (không interceptor)
  useEffect(() => {
    if (!account) {
      // Cleanup when disconnected
      if (wsChainRef.current) {
        wsChainRef.current.close();
        wsChainRef.current = null;
      }
      if (reconnectChainTimeoutRef.current) {
        clearTimeout(reconnectChainTimeoutRef.current);
        reconnectChainTimeoutRef.current = null;
      }
      return;
    }

    const connectChainWebSocket = () => {
      // Close existing connection
      if (wsChainRef.current) {
        wsChainRef.current.close();
        wsChainRef.current = null;
      }

      try {
        const ws = new WebSocket(WSS_RPC);
        wsChainRef.current = ws;

        ws.onopen = () => {
          console.log("✅ Chain WebSocket connected (no interceptor)");
          subscribeToEvents(ws, "chain");
        };

        ws.onmessage = (event) => {
          handleWebSocketMessage(event, "chain");
        };

        ws.onerror = (error) => {
          console.error("❌ Chain WebSocket error:", error);
        };

        ws.onclose = (event) => {
          console.log("❌ Chain WebSocket disconnected");
          wsChainRef.current = null;

          // Auto-reconnect after 500ms
          if (event.code !== 1000) {
            reconnectChainTimeoutRef.current = setTimeout(() => {
              connectChainWebSocket();
            }, 500);
          }
        };
      } catch (err) {
        console.error("❌ Error setting up Chain WebSocket:", err);
        setError(
          err instanceof Error ? err.message : "Failed to setup Chain WebSocket"
        );
      }
    };

    connectChainWebSocket();

    // Cleanup on unmount
    return () => {
      if (wsChainRef.current) {
        wsChainRef.current.close();
        wsChainRef.current = null;
      }
      if (reconnectChainTimeoutRef.current) {
        clearTimeout(reconnectChainTimeoutRef.current);
        reconnectChainTimeoutRef.current = null;
      }
    };
  }, [account]);

  // WebSocket subscription for INTERCEPTOR events (có interceptor)
  useEffect(() => {
    if (!account) {
      // Cleanup when disconnected
      if (wsInterceptorRef.current) {
        wsInterceptorRef.current.close();
        wsInterceptorRef.current = null;
      }
      if (reconnectInterceptorTimeoutRef.current) {
        clearTimeout(reconnectInterceptorTimeoutRef.current);
        reconnectInterceptorTimeoutRef.current = null;
      }
      return;
    }

    const connectInterceptorWebSocket = () => {
      // Close existing connection
      if (wsInterceptorRef.current) {
        wsInterceptorRef.current.close();
        wsInterceptorRef.current = null;
      }

      try {
        const ws = new WebSocket(WSS_RPC_INTERCEPTOR);
        wsInterceptorRef.current = ws;

        ws.onopen = () => {
          console.log("✅ Interceptor WebSocket connected");
          subscribeToEvents(ws, "interceptor");
        };

        ws.onmessage = (event) => {
          handleWebSocketMessage(event, "interceptor");
        };

        ws.onerror = (error) => {
          console.error("❌ Interceptor WebSocket error:", error);
        };

        ws.onclose = (event) => {
          console.log("❌ Interceptor WebSocket disconnected");
          wsInterceptorRef.current = null;

          // Auto-reconnect after 500ms
          if (event.code !== 1000) {
            reconnectInterceptorTimeoutRef.current = setTimeout(() => {
              connectInterceptorWebSocket();
            }, 500);
          }
        };
      } catch (err) {
        console.error("❌ Error setting up Interceptor WebSocket:", err);
        setError(
          err instanceof Error ? err.message : "Failed to setup Interceptor WebSocket"
        );
      }
    };

    connectInterceptorWebSocket();

    // Cleanup on unmount
    return () => {
      if (wsInterceptorRef.current) {
        wsInterceptorRef.current.close();
        wsInterceptorRef.current = null;
      }
      if (reconnectInterceptorTimeoutRef.current) {
        clearTimeout(reconnectInterceptorTimeoutRef.current);
        reconnectInterceptorTimeoutRef.current = null;
      }
    };
  }, [account]);

  return (
    <RobotContext.Provider
      value={{
        account,
        isConnected: !!account,
        connectWallet,
        disconnectWallet,
        createSession,
        emitSentence,
        chainEvents,
        interceptorEvents,
        clearChainEvents,
        clearInterceptorEvents,
        clearAllEvents,
        isLoading,
        error,
      }}
    >
      {children}
    </RobotContext.Provider>
  );
}

export function useRobot() {
  const context = useContext(RobotContext);
  if (!context) {
    throw new Error("useRobot must be used within RobotProvider");
  }
  return context;
}

