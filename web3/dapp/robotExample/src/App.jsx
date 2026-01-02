import { useState, useEffect } from "react";
import { useRobot } from "./contexts/RobotContext";
import { privateKey } from "./constants/customeChain";

function App() {
  const {
    account,
    isConnected,
    connectWallet,
    disconnectWallet,
    dispatch,
    getDataByTxhash,
    chainEvents,
    interceptorEvents,
    clearChainEvents,
    clearInterceptorEvents,
    clearAllEvents,
    isLoading,
    error,
  } = useRobot();

  // Form states
  const [privateKeyInput, setPrivateKeyInput] = useState("");
  const [sessionId, setSessionId] = useState("");
  const [actionId, setActionId] = useState("");
  const [data, setData] = useState("");
  const [activeTab, setActiveTab] = useState("chain"); // "chain" or "interceptor"
  
  // Get data by txHash states
  const [txHashInput, setTxHashInput] = useState("");
  const [txData, setTxData] = useState(null);
  const [isLoadingTxData, setIsLoadingTxData] = useState(false);
  
  // Spam test states
  const [isSpamming, setIsSpamming] = useState(false);
  const [spamProgress, setSpamProgress] = useState({ sent: 0, success: 0, failed: 0, errors: [] });

  // Auto-connect with private key from constants on mount
  useEffect(() => {
    if (!isConnected && privateKey) {
      console.log("Auto-connecting with private key from constants...");
      connectWallet(privateKey);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only run once on mount

  const handleConnect = () => {
    const keyToUse = privateKeyInput.trim() || privateKey;
    if (!keyToUse) {
      alert("Please enter a private key");
      return;
    }
    connectWallet(keyToUse);
  };

  const handleDispatch = async (e) => {
    e.preventDefault();
    if (!sessionId || !actionId || !data) {
      alert("Please fill all fields");
      return;
    }

    try {
      console.log(`Dispatching sessionId=${sessionId}, actionId=${actionId}, data=${data}`);
      const hash = await dispatch(sessionId, actionId, data);
      console.log(`✅ Dispatch successful! Transaction hash: ${hash}`);
      // Don't reset form to allow spam testing
    } catch (err) {
      alert(`Error: ${err.message}`);
    }
  };

  const handleGetDataByTxhash = async (e) => {
    e.preventDefault();
    if (!txHashInput || txHashInput.trim() === "") {
      alert("Please enter a transaction hash");
      return;
    }

    setIsLoadingTxData(true);
    setTxData(null);
    try {
      const data = await getDataByTxhash(txHashInput.trim());
      setTxData(data);
      console.log("✅ Data retrieved:", data);
    } catch (err) {
      alert(`Error: ${err.message}`);
      setTxData(null);
    } finally {
      setIsLoadingTxData(false);
    }
  };

  // Spam 1000 dispatch commands để test nonce
  const handleSpamDispatch = async () => {
    // Validate input
    if (!sessionId || sessionId.trim() === "") {
      alert("Please fill Session ID field first");
      return;
    }
    if (!actionId || actionId.trim() === "") {
      alert("Please fill Action ID field first");
      return;
    }
    if (!data || data.trim() === "") {
      alert("Please fill Data field first");
      return;
    }

    if (isSpamming) {
      alert("Spam test is already running!");
      return;
    }

    setIsSpamming(true);
    setSpamProgress({ sent: 0, success: 0, failed: 0, errors: [] });

    const total = 1000;
    const batchSize = 10; // Gửi 10 lệnh cùng lúc để tăng tốc
    let successCount = 0;
    let failedCount = 0;
    const allErrors = [];

    try {
      for (let i = 0; i < total; i += batchSize) {
        const batch = [];
        const batchEnd = Math.min(i + batchSize, total);

        // Tạo batch promises
        for (let j = i; j < batchEnd; j++) {
          try {
            // Xử lý data: nếu là hex string thì giữ nguyên, nếu là text thì append index
            let dataWithIndex;
            if (data.startsWith("0x")) {
              // Nếu là hex string, không append text vào (giữ nguyên hoặc có thể append hex index nếu cần)
              // Để đơn giản, giữ nguyên data khi spam với hex string
              dataWithIndex = data;
            } else {
              // Nếu là text, append index vào text
              dataWithIndex = `${data} [${j}]`;
            }
            
            batch.push(
              dispatch(sessionId, actionId, dataWithIndex)
                .then((hash) => {
                  successCount++;
                  setSpamProgress((prev) => ({
                    ...prev,
                    sent: prev.sent + 1,
                    success: successCount,
                  }));
                  console.log(`✅ Success at index ${j}, hash: ${hash}`);
                })
                .catch((err) => {
                  const errorMsg = err?.message || String(err) || "Unknown error";
                  failedCount++;
                  const errorObj = { index: j, error: errorMsg };
                  allErrors.push(errorObj);
                  
                  setSpamProgress((prev) => ({
                    ...prev,
                    sent: prev.sent + 1,
                    failed: failedCount,
                    errors: [...prev.errors.slice(-9), errorObj], // Giữ lại 10 lỗi gần nhất
                  }));
                  console.error(`❌ Error at index ${j}:`, err);
                })
            );
          } catch (validationErr) {
            const errorMsg = `Validation error: ${validationErr?.message || String(validationErr)}`;
            failedCount++;
            const errorObj = { index: j, error: errorMsg };
            allErrors.push(errorObj);
            
            setSpamProgress((prev) => ({
              ...prev,
              sent: prev.sent + 1,
              failed: failedCount,
              errors: [...prev.errors.slice(-9), errorObj],
            }));
            console.error(`❌ Validation error at index ${j}:`, validationErr);
          }
        }

        // Đợi batch hoàn thành
        await Promise.allSettled(batch);

        // Delay nhỏ giữa các batch để tránh quá tải
        if (batchEnd < total) {
          await new Promise((resolve) => setTimeout(resolve, 50));
        }
      }

      // Final summary
      const finalProgress = {
        sent: total,
        success: successCount,
        failed: failedCount,
        errors: allErrors.slice(-10), // Lấy 10 lỗi cuối cùng
      };

      console.log("✅ Spam test completed!", finalProgress);
      setSpamProgress(finalProgress);
    } catch (err) {
      console.error("❌ Spam test error:", err);
      alert(`Spam test error: ${err.message}`);
    } finally {
      setIsSpamming(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-100 p-8">
      <div className="max-w-6xl mx-auto">
        <h1 className="text-4xl font-bold text-center mb-8 text-gray-800">
          Robot Contract Demo
        </h1>

        {/* Wallet Connection Section */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-6">
          <h2 className="text-2xl font-semibold mb-4">Wallet Connection</h2>
          {!isConnected ? (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Private Key (Optional - will use default from constants if empty)
                </label>
                <input
                  type="password"
                  value={privateKeyInput}
                  onChange={(e) => setPrivateKeyInput(e.target.value)}
                  placeholder={
                    privateKey
                      ? `Default: ${privateKey.slice(0, 10)}... (from constants)`
                      : "Enter your private key (0x... or without 0x)"
                  }
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <button
                onClick={handleConnect}
                className="w-full bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 transition-colors"
              >
                Connect Wallet
              </button>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="bg-green-50 border border-green-200 rounded-md p-4">
                <p className="text-sm text-gray-600">Connected Account:</p>
                <p className="font-mono text-sm text-green-800 break-all">
                  {account}
                </p>
              </div>
              <button
                onClick={disconnectWallet}
                className="w-full bg-red-600 text-white py-2 px-4 rounded-md hover:bg-red-700 transition-colors"
              >
                Disconnect Wallet
              </button>
            </div>
          )}
        </div>

        {/* Error Display */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-6">
            <p className="text-red-800 text-sm">{error}</p>
          </div>
        )}

        {/* Dispatch Section */}
        {isConnected && (
          <div className="bg-white rounded-lg shadow-md p-6 mb-6">
            <h2 className="text-2xl font-semibold mb-4">Dispatch</h2>
            <form onSubmit={handleDispatch} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Session ID (bytes32 - hex string)
                </label>
                <input
                  type="text"
                  value={sessionId}
                  onChange={(e) => setSessionId(e.target.value)}
                  placeholder="e.g., 0x1234... or hex string"
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Action ID (bytes32 - hex string)
                </label>
                <input
                  type="text"
                  value={actionId}
                  onChange={(e) => setActionId(e.target.value)}
                  placeholder="e.g., 0x5678... or hex string"
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Data (bytes - hex string or text)
                </label>
                <textarea
                  value={data}
                  onChange={(e) => setData(e.target.value)}
                  placeholder="Enter data (text or hex string starting with 0x)"
                  rows={3}
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={isLoading || isSpamming}
                  className="flex-1 bg-purple-600 text-white py-2 px-4 rounded-md hover:bg-purple-700 transition-colors disabled:bg-gray-400 disabled:cursor-not-allowed"
                >
                  {isLoading ? "Processing..." : "Dispatch"}
                </button>
                <button
                  type="button"
                  onClick={handleSpamDispatch}
                  disabled={isLoading || isSpamming || !sessionId || !actionId || !data}
                  className="flex-1 bg-orange-600 text-white py-2 px-4 rounded-md hover:bg-orange-700 transition-colors disabled:bg-gray-400 disabled:cursor-not-allowed"
                >
                  {isSpamming ? "Spamming..." : "🚀 Spam 1000x"}
                </button>
              </div>
            </form>

            {/* Spam Progress Display */}
            {isSpamming && (
              <div className="mt-4 p-4 bg-orange-50 border border-orange-200 rounded-md">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-sm font-semibold text-orange-800">
                    Spam Test Progress
                  </h3>
                  <span className="text-xs text-orange-600">
                    {spamProgress.sent} / 1000
                  </span>
                </div>
                <div className="w-full bg-orange-200 rounded-full h-2 mb-2">
                  <div
                    className="bg-orange-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${(spamProgress.sent / 1000) * 100}%` }}
                  ></div>
                </div>
                <div className="flex gap-4 text-xs">
                  <span className="text-green-600">
                    ✅ Success: {spamProgress.success}
                  </span>
                  <span className="text-red-600">
                    ❌ Failed: {spamProgress.failed}
                  </span>
                </div>
                {spamProgress.errors.length > 0 && (
                  <div className="mt-2 max-h-32 overflow-y-auto">
                    <p className="text-xs font-semibold text-red-600 mb-1">
                      Recent Errors (showing last 5):
                    </p>
                    {spamProgress.errors.slice(-5).map((err, idx) => (
                      <div
                        key={idx}
                        className="text-xs text-red-700 bg-red-50 p-1 mb-1 rounded"
                      >
                        <span className="font-mono">#{err.index}:</span>{" "}
                        {err.error}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Get Data By TxHash Section */}
        {isConnected && (
          <div className="bg-white rounded-lg shadow-md p-6 mb-6">
            <h2 className="text-2xl font-semibold mb-4">Get Data By TxHash</h2>
            <form onSubmit={handleGetDataByTxhash} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Transaction Hash
                </label>
                <input
                  type="text"
                  value={txHashInput}
                  onChange={(e) => setTxHashInput(e.target.value)}
                  placeholder="0x..."
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <button
                type="submit"
                disabled={isLoadingTxData}
                className="w-full bg-green-600 text-white py-2 px-4 rounded-md hover:bg-green-700 transition-colors disabled:bg-gray-400 disabled:cursor-not-allowed"
              >
                {isLoadingTxData ? "Loading..." : "Get Data"}
              </button>
            </form>

            {/* Display Transaction Data */}
            {txData && (
              <div className="mt-4 p-4 bg-green-50 border border-green-200 rounded-md">
                <h3 className="text-lg font-semibold text-green-800 mb-2">
                  Transaction Data
                </h3>
                <div className="space-y-2 text-sm">
                  <div>
                    <span className="font-semibold">TxHash:</span>{" "}
                    <span className="font-mono">{txData.txHash}</span>
                  </div>
                  <div>
                    <span className="font-semibold">From:</span>{" "}
                    <span className="font-mono">{txData.fromAddress}</span>
                  </div>
                  <div>
                    <span className="font-semibold">To:</span>{" "}
                    <span className="font-mono">{txData.toAddress}</span>
                  </div>
                  <div>
                    <span className="font-semibold">Created At:</span>{" "}
                    {new Date(txData.createdAt * 1000).toLocaleString()}
                  </div>
                  <div>
                    <span className="font-semibold">Raw Tx Hex:</span>{" "}
                    <span className="font-mono text-xs break-all">
                      {txData.rawTxHex}
                    </span>
                  </div>
                  {txData.events && txData.events.length > 0 && (
                    <div>
                      <span className="font-semibold">Events:</span>
                      <div className="mt-2 space-y-2">
                        {txData.events.map((event, idx) => (
                          <div
                            key={idx}
                            className="bg-white p-2 rounded border border-green-200"
                          >
                            <div className="text-xs">
                              <span className="font-semibold">Data:</span>{" "}
                              <span className="font-mono break-all">
                                {event.data}
                              </span>
                            </div>
                            <div className="text-xs mt-1">
                              <span className="font-semibold">Created At:</span>{" "}
                              {new Date(event.createdAt * 1000).toLocaleString()}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Events Display Section with Tabs */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-2xl font-semibold">Events</h2>
            <div className="flex gap-2">
              {(chainEvents.length > 0 || interceptorEvents.length > 0) && (
                <button
                  onClick={clearAllEvents}
                  className="text-sm text-red-600 hover:text-red-800"
                >
                  Clear All
                </button>
              )}
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200 mb-4">
            <button
              onClick={() => setActiveTab("chain")}
              className={`px-4 py-2 font-medium text-sm transition-colors ${
                activeTab === "chain"
                  ? "border-b-2 border-blue-600 text-blue-600"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              Chain Events ({chainEvents.length})
            </button>
            <button
              onClick={() => setActiveTab("interceptor")}
              className={`px-4 py-2 font-medium text-sm transition-colors ${
                activeTab === "interceptor"
                  ? "border-b-2 border-purple-600 text-purple-600"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              Interceptor Events ({interceptorEvents.length})
            </button>
          </div>

          {/* Tab Content */}
          {activeTab === "chain" ? (
            <div>
              <div className="flex justify-between items-center mb-2">
                <p className="text-sm text-gray-600">
                  Events from chain (no interceptor) - Forward trực tiếp lên chain
                </p>
                {chainEvents.length > 0 && (
                  <button
                    onClick={clearChainEvents}
                    className="text-xs text-red-600 hover:text-red-800"
                  >
                    Clear Chain Events
                  </button>
                )}
              </div>
              {chainEvents.length === 0 ? (
                <p className="text-gray-500 text-center py-8">
                  No chain events yet. Events from chain will appear here.
                </p>
              ) : (
                <div className="space-y-3 max-h-96 overflow-y-auto">
                  {chainEvents.map((event) => (
                    <EventCard key={event.id} event={event} source="chain" />
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div>
              <div className="flex justify-between items-center mb-2">
                <p className="text-sm text-gray-600">
                  Events from interceptor - Chặn lại và trả về RPC
                </p>
                {interceptorEvents.length > 0 && (
                  <button
                    onClick={clearInterceptorEvents}
                    className="text-xs text-red-600 hover:text-red-800"
                  >
                    Clear Interceptor Events
                  </button>
                )}
              </div>
              {interceptorEvents.length === 0 ? (
                <p className="text-gray-500 text-center py-8">
                  No interceptor events yet. Events from interceptor will appear here.
                </p>
              ) : (
                <div className="space-y-3 max-h-96 overflow-y-auto">
                  {interceptorEvents.map((event) => (
                    <EventCard key={event.id} event={event} source="interceptor" />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// EventCard component để hiển thị event
function EventCard({ event, source }) {
  const formatTimestamp = (timestamp) => {
    if (!timestamp) return "N/A";
    return new Date(Number(timestamp) * 1000).toLocaleString();
  };

  const formatBytes32 = (bytes32) => {
    if (!bytes32) return "N/A";
    const str = typeof bytes32 === "string" ? bytes32 : bytes32.toString();
    return str.length > 20 ? `${str.slice(0, 10)}...${str.slice(-8)}` : str;
  };

  const formatBytes = (bytes) => {
    if (!bytes) return "N/A";
    const str = typeof bytes === "string" ? bytes : bytes.toString();
    if (str.startsWith("0x")) {
      // Try to decode as UTF-8 if possible
      try {
        const hex = str.slice(2);
        // Convert hex string to Uint8Array, then to UTF-8 string
        const bytesArray = new Uint8Array(
          hex.match(/.{1,2}/g)?.map((byte) => parseInt(byte, 16)) || []
        );
        const decoder = new TextDecoder("utf-8", { fatal: false });
        const decoded = decoder.decode(bytesArray);
        // Check if it's valid UTF-8 (no null bytes in the middle and not empty)
        if (decoded && !decoded.includes("\x00") && decoded.length > 0) {
          return decoded;
        }
      } catch {
        // Not valid UTF-8, show hex
      }
    }
    return str.length > 50 ? `${str.slice(0, 25)}...${str.slice(-25)}` : str;
  };

  return (
    <div
      className={`border rounded-md p-4 hover:bg-gray-50 transition-colors ${
        source === "chain"
          ? "border-blue-200 bg-blue-50"
          : "border-purple-200 bg-purple-50"
      }`}
    >
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          <span
            className={`px-2 py-1 rounded text-xs font-semibold ${
              event.eventName === "EmitSentence"
                ? "bg-green-100 text-green-800"
                : "bg-purple-100 text-purple-800"
            }`}
          >
            {event.eventName}
          </span>
          <span
            className={`px-2 py-1 rounded text-xs font-semibold ${
              source === "chain"
                ? "bg-blue-200 text-blue-800"
                : "bg-purple-200 text-purple-800"
            }`}
          >
            {source === "chain" ? "🔗 Chain" : "🛡️ Interceptor"}
          </span>
        </div>
        <span className="text-xs text-gray-500">
          {formatTimestamp(event.timestamp)}
        </span>
      </div>
      <div className="space-y-1 text-sm">
        <p>
          <span className="font-medium">Session ID:</span>{" "}
          <span className="font-mono text-xs">{formatBytes32(event.sessionId)}</span>
        </p>
        <p>
          <span className="font-medium">Action ID:</span>{" "}
          <span className="font-mono text-xs">{formatBytes32(event.actionId)}</span>
        </p>
        {event.operator && (
          <p>
            <span className="font-medium">Operator:</span>{" "}
            <span className="font-mono text-xs">{event.operator}</span>
          </p>
        )}
        {event.data && (
          <p>
            <span className="font-medium">Data:</span>{" "}
            <span className="font-mono text-xs break-all">
              {formatBytes(event.data)}
            </span>
          </p>
        )}
      </div>
    </div>
  );
}

export default App;
