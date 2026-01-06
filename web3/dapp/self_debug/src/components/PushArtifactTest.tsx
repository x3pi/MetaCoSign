import { useState } from 'react';
import { GO_BACKEND_RPC_URL } from '../constants/customChain';

interface PushArtifactParams {
  contract_address: string;
  metadata: string;
  source_code: string;
  abi: string;
  source_map: string;
  storage_layout: string;
}

interface JSONRPCResponse {
  jsonrpc: string;
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
  id: number;
}
const sourceCodeObj = {
    "test.sol": `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;

contract TestDebug {
    uint256 public balance;

    // Lỗi 1: Revert có thông báo (dễ test nhất)
    function testRequire(uint256 _value) public pure {
        require(_value > 10, "Gia tri phai lon hon 10");
    }

    // Lỗi 2: Lỗi tính toán (Overflow/Underflow) 
    // Solidity 0.8.x sẽ tự panic nếu balance = 0 mà trừ đi 1
    function testUnderflow() public {
        balance -= 1; 
    }

    // Lỗi 3: Revert thủ công
    function testRevert() public pure {
        if (true) {
            revert("Loi manual revert tai day");
        }
    }
}`
  };
export default function PushArtifactTest() {
  const [params, setParams] = useState<PushArtifactParams>({
    contract_address: '',
    metadata: '',
    source_code: '',
    abi: '',
    source_map: '',
    storage_layout: '',
  });

  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<JSONRPCResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    const { name, value, type } = e.target;
    setParams((prev) => ({
      ...prev,
      [name]:
        type === 'checkbox'
          ? (e.target as HTMLInputElement).checked
          : value,
    }));
  };

  const loadExampleData = () => {
    setParams({
      contract_address: '0x75e00FF70bd4eFA111D768425db8B9f1781C9939',
      metadata: JSON.stringify({"compiler":{"version":"0.8.30+commit.73712a01"},"language":"Solidity","output":{"abi":[{"inputs":[],"name":"balance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"_value","type":"uint256"}],"name":"testRequire","outputs":[],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"testRevert","outputs":[],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"testUnderflow","outputs":[],"stateMutability":"nonpayable","type":"function"}],"devdoc":{"kind":"dev","methods":{},"version":1},"userdoc":{"kind":"user","methods":{},"version":1}},"settings":{"compilationTarget":{"test.sol":"TestDebug"},"evmVersion":"prague","libraries":{},"metadata":{"bytecodeHash":"ipfs"},"optimizer":{"enabled":true,"runs":200},"remappings":[],"viaIR":true},"sources":{"test.sol":{"keccak256":"0x4bb5136e9b0a7f49cddd237418f5ef7ba01234f5ca936ba80036ede45d352f2f","license":"MIT","urls":["bzz-raw://ec21ddafb118029cde251f6d775434161798f720c4de63b2a3be7f38bcc03f78","dweb:/ipfs/QmccijEUvR9Aq38N2fXNxdyvsHwbE6oj8xnHTRwk3TeQ8d"]}},"version":1} ,null, 2),
      source_code: JSON.stringify(sourceCodeObj, null, 2),
      abi: JSON.stringify([
        {
            "inputs": [],
            "name": "testUnderflow",
            "outputs": [],
            "stateMutability": "nonpayable",
            "type": "function"
        },
        {
            "inputs": [],
            "name": "balance",
            "outputs": [
                {
                    "internalType": "uint256",
                    "name": "",
                    "type": "uint256"
                }
            ],
            "stateMutability": "view",
            "type": "function"
        },
        {
            "inputs": [
                {
                    "internalType": "uint256",
                    "name": "_value",
                    "type": "uint256"
                }
            ],
            "name": "testRequire",
            "outputs": [],
            "stateMutability": "pure",
            "type": "function"
        },
        {
            "inputs": [],
            "name": "testRevert",
            "outputs": [],
            "stateMutability": "pure",
            "type": "function"
        }
    ], null, 2),
      source_map: "58:584:0:-:0;;;;;;;;;;;;;;;;;;;;;;;;;;;588:35;58:584;588:35;;;58:584;;;;;;;;;;;;;-1:-1:-1;;58:584:0;;;;246:2;58:584;;237:11;58:584;;;;;;;-1:-1:-1;;;58:584:0;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;-1:-1:-1;;58:584:0;;;;;;;;;;;;;;;;;;;-1:-1:-1;;58:584:0;;;;;;-1:-1:-1;;;588:35:0;;58:584;;588:35;;58:584;;;;;;;;;;;588:35;;;58:584;;;;;;-1:-1:-1;;58:584:0;;;;;;-1:-1:-1;;58:584:0;;;;;;;;;;;;;;;;;;;;;",
      storage_layout: JSON.stringify({
        "storage": [
            {
                "astId": 3,
                "contract": "test.sol:TestDebug",
                "label": "balance",
                "offset": 0,
                "slot": "0",
                "type": "t_uint256"
            }
        ],
        "types": {
            "t_uint256": {
                "encoding": "inplace",
                "label": "uint256",
                "numberOfBytes": "32"
            }
        }
    }, null, 2),
    });
  };

  const validateJSON = (jsonString: string, fieldName: string): boolean => {
    try {
      JSON.parse(jsonString);
      return true;
    } catch (e) {
      setError(`Invalid JSON in ${fieldName}: ${e instanceof Error ? e.message : 'Unknown error'}`);
      return false;
    }
  };

  const handleSubmitWithValidation = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    
    // Validate JSON fields
    if (!validateJSON(params.metadata, 'metadata')) return;
    if (!validateJSON(params.source_code, 'source_code')) return;
    if (!validateJSON(params.abi, 'abi')) return;
    if (params.storage_layout && !validateJSON(params.storage_layout, 'storage_layout')) return;

    // If validation passes, proceed with submit
    setLoading(true);
    setResponse(null);

    try {
      const requestBody = {
        jsonrpc: '2.0',
        method: 'rpc_pushArtifact',
        params: params,
        id: 1,
      };

      const res = await fetch(GO_BACKEND_RPC_URL, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });

      const data: JSONRPCResponse = await res.json();
      setResponse(data);

      if (data.error) {
        setError(data.error.message);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-6xl mx-auto p-6 space-y-6">
      <h1 className="text-3xl font-bold text-gray-800 mb-6">
        Test RPC Push Artifact
      </h1>

      <div className="mb-4 flex gap-2">
        <button
          type="button"
          onClick={loadExampleData}
          className="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500 text-sm"
        >
          Load Example Data
        </button>
        <button
          type="button"
          onClick={() => {
            setParams({
              contract_address: '',
              metadata: '',
              source_code: '',
              abi: '',
              source_map: '',
              storage_layout: '',
            });
            setResponse(null);
            setError(null);
          }}
          className="px-4 py-2 bg-gray-400 text-white rounded-md hover:bg-gray-500 focus:outline-none focus:ring-2 focus:ring-gray-500 text-sm"
        >
          Clear All
        </button>
      </div>

      <form onSubmit={handleSubmitWithValidation} className="space-y-4">
        {/* Contract Address */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Contract Address *
          </label>
          <input
            type="text"
            name="contract_address"
            value={params.contract_address}
            onChange={handleChange}
            required
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="0x..."
          />
        </div>

        {/* Metadata (JSON) */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Metadata (config.json) *
          </label>
          <textarea
            name="metadata"
            value={params.metadata}
            onChange={handleChange}
            required
            rows={8}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
            placeholder='{"compiler": {...}, "language": "Solidity", ...}'
          />
        </div>

        {/* Source Code (JSON) */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Source Code (JSON) *
          </label>
          <textarea
            name="source_code"
            value={params.source_code}
            onChange={handleChange}
            required
            rows={8}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
            placeholder='{"contracts/MyContract.sol": "pragma solidity...", ...}'
          />
        </div>

        {/* ABI (JSON) */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            ABI (JSON) *
          </label>
          <textarea
            name="abi"
            value={params.abi}
            onChange={handleChange}
            required
            rows={6}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
            placeholder='[{"type": "function", ...}]'
          />
        </div>

        {/* Source Map */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Source Map
          </label>
          <textarea
            name="source_map"
            value={params.source_map}
            onChange={handleChange}
            rows={3}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
            placeholder="Source map string"
          />
        </div>

        {/* Storage Layout */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Storage Layout (JSON)
          </label>
          <textarea
            name="storage_layout"
            value={params.storage_layout}
            onChange={handleChange}
            rows={4}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
            placeholder='{"storage": [...], ...}'
          />
        </div>

        {/* Submit Button */}
        <button
          type="submit"
          disabled={loading}
          className="w-full bg-blue-600 text-white py-3 px-4 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed font-medium"
        >
          {loading ? 'Sending...' : 'Send RPC Request'}
        </button>
      </form>

      {/* Error Display */}
      {error && (
        <div className="mt-6 p-4 bg-red-50 border border-red-200 rounded-md">
          <h3 className="text-lg font-semibold text-red-800 mb-2">Error</h3>
          <p className="text-red-700">{error}</p>
        </div>
      )}

      {/* Response Display */}
      {response && (
        <div className="mt-6">
          <h3 className="text-lg font-semibold text-gray-800 mb-2">
            Response
          </h3>
          <div className="bg-gray-50 border border-gray-200 rounded-md p-4">
            <pre className="text-sm overflow-auto max-h-96">
              {JSON.stringify(response, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

