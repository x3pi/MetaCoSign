import { useState, useEffect, useCallback } from "react";
import { encodeFunctionData, type Hex, hexToString } from "viem";
import { useWallet } from "~/contexts/WalletContext";
import { PageContainer } from "~/components/PageContainer";
import { PageCard } from "~/components/PageCard";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import { Badge } from "~/components/ui/badge";
import { Alert } from "~/components/ui/alert";
import { contracts } from "~/constants/contracts";
import { chain991 } from "~/constants/customChain";
import LoadingSpinnerIcon from "~/components/LoadingSpinnerIcon";

interface BlsAccount {
  address: Uint8Array;
  blsPublicKey: Uint8Array;
  registeredAt: bigint;
  isConfirmed: boolean;
  confirmedAt?: bigint;
  registerTxHash: Uint8Array;
  confirmTxHash?: Uint8Array;
}

const BLS_PUBLIC_KEY =
  "0x86d5de6f7c9c13cc0d959a553cc0e4853ba5faae45a28da9bddc8ef8e104eb5d3dece8dfaa24f11b4243ec27537e3184" as Hex;

function BlsAccountListPage() {
  const { walletClient, publicClient } = useWallet();

  const [filter, setFilter] = useState<"confirmed" | "unconfirmed">(
    "unconfirmed"
  );
  const [accounts, setAccounts] = useState<BlsAccount[]>([]);
  const [page, setPage] = useState(0);
  const [pageSize] = useState(20);
  const [totalPages, setTotalPages] = useState(0);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string>("");
  const [confirmingAddress, setConfirmingAddress] = useState<string | null>(
    null
  );

  // Load accounts callback
  const loadAccounts = useCallback(async () => {
    if (!publicClient) return;

    setIsLoading(true);
    setError("");

    try {
      const timestamp = Math.floor(Date.now() / 1000);

      // TODO: Sign with actual BLS private key
      // For now, use a dummy signature (96 bytes)
      const signature = ("0x" + "00".repeat(96)) as Hex;
      // Call getAllAccount via eth_call
      const data = encodeFunctionData({
        abi: contracts.AccountManager.abi,
        functionName: "getAllAccount",
        args: [
          signature,
          BLS_PUBLIC_KEY,
          BigInt(timestamp),
          BigInt(page),
          BigInt(pageSize),
          filter === "confirmed",
        ],
      });

      // Use eth_call to read data
      const result = await publicClient.call({
        to: contracts.AccountManager.address,
        data: data,
      });

      if (result.data) {
        // The backend will return JSON in the result data
        // Decode hex to string then parse JSON
        try {
          const jsonStr = hexToString(result.data);
          const response: {
            accounts: BlsAccount[];
            total: number;
            page: number;
            pageSize: number;
            totalPage: number;
          } = JSON.parse(jsonStr);

          setAccounts(response.accounts);
          setTotal(response.total);
          setTotalPages(response.totalPage);
        } catch (parseErr) {
          console.error("Failed to parse response:", parseErr);
          setError("Failed to parse accounts data");
        }
      } else {
        setAccounts([]);
        setTotal(0);
        setTotalPages(0);
      }
    } catch (err) {
      console.error("Error loading accounts:", err);
      setError(err instanceof Error ? err.message : "Failed to load accounts");
    } finally {
      setIsLoading(false);
    }
  }, [publicClient, page, pageSize, filter]);

  // Load accounts when filter or page changes
  useEffect(() => {
    loadAccounts();
  }, [loadAccounts]);

  const handleConfirmAccount = async (accountAddress: Hex) => {
    if (!walletClient || !publicClient) {
      setError("Wallet not connected");
      return;
    }
    setConfirmingAddress(accountAddress);
    setError("");

    try {
      // Encode confirmAccount function call
      const data = encodeFunctionData({
        abi: contracts.AccountManager.abi,
        functionName: "confirmAccount",
        args: [accountAddress],
      });

      const account = walletClient.account;
      if (!account) {
        throw new Error("No account connected");
      }

      // Get nonce
      const nonce = await publicClient.getTransactionCount({
        address: account.address,
        blockTag: "pending",
      });

      // Estimate gas
      const gasLimit = await publicClient.estimateGas({
        account: account.address,
        to: contracts.AccountManager.address,
        data: data,
        value: 0n,
      });

      // Get gas price
      const gasPrice = await publicClient.getGasPrice();

      // Send transaction
      const txHash = await walletClient.sendTransaction({
        account: account.address,
        to: contracts.AccountManager.address,
        data: data,
        value: 0n,
        nonce: nonce,
        gas: gasLimit,
        gasPrice: gasPrice,
        chain: chain991,
      });

      console.log("Confirm transaction sent:", txHash);

      // Wait for transaction
      const receipt = await publicClient.waitForTransactionReceipt({
        hash: txHash,
      });

      if (receipt.status === "success") {
        // Reload accounts
        await loadAccounts();
        setError("");
        alert(`Account ${accountAddress} confirmed successfully!`);
      } else {
        throw new Error("Transaction failed");
      }
    } catch (err) {
      console.error("Error confirming account:", err);
      setError(err instanceof Error ? err.message : "Failed to confirm account");
    } finally {
      setConfirmingAddress(null);
    }
  };

  const formatDate = (timestamp: bigint) => {
    return new Date(Number(timestamp) * 1000).toLocaleString();
  };

  const formatAddress = (addressBytes: Uint8Array) => {
    const hex = Array.from(addressBytes)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    const address = "0x" + hex;
    return `${address.slice(0, 6)}...${address.slice(-4)}`;
  };

  const formatHash = (hashBytes: Uint8Array) => {
    const hex = Array.from(hashBytes)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    return "0x" + hex;
  };

  const bytesToAddress = (bytes: Uint8Array): Hex => {
    const hex = Array.from(bytes)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    return ("0x" + hex) as Hex;
  };

  return (
    <PageContainer>
      <PageCard
        title="BLS Account Management"
        description={`Manage accounts registered with BLS Public Key: ${BLS_PUBLIC_KEY.slice(
          0,
          20
        )}...`}
      >
        {/* Filter Buttons */}
        <div className="flex gap-4 mb-6">
          <Button
            onClick={() => {
              setFilter("unconfirmed");
              setPage(0);
            }}
            variant={filter === "unconfirmed" ? "default" : "outline"}
            className="flex-1"
          >
            Unconfirmed ({filter === "unconfirmed" ? total : "?"})
          </Button>
          <Button
            onClick={() => {
              setFilter("confirmed");
              setPage(0);
            }}
            variant={filter === "confirmed" ? "default" : "outline"}
            className="flex-1"
          >
            Confirmed ({filter === "confirmed" ? total : "?"})
          </Button>
        </div>

        {/* Error Alert */}
        {error && (
          <Alert variant="destructive" className="mb-4">
            {error}
          </Alert>
        )}

        {/* Loading State */}
        {isLoading && (
          <div className="flex justify-center items-center py-12">
            <LoadingSpinnerIcon />
            <span className="ml-2">Loading accounts...</span>
          </div>
        )}

        {/* Accounts List */}
        {!isLoading && accounts.length > 0 && (
          <div className="space-y-4">
            {accounts.map((account) => (
              <div
                key={formatHash(account.address)}
                className="border border-border rounded-lg p-4 hover:bg-card-hover transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 space-y-2">
                    <div className="flex items-center gap-2">
                      <Label className="font-mono text-sm">
                        {formatHash(account.address)}
                      </Label>
                      <Badge
                        variant={account.isConfirmed ? "success" : "warning"}
                      >
                        {account.isConfirmed ? "Confirmed" : "Pending"}
                      </Badge>
                    </div>

                    <div className="text-xs text-app-muted space-y-1">
                      <div>
                        <span className="font-semibold">Registered:</span>{" "}
                        {formatDate(account.registeredAt)}
                      </div>
                      {account.isConfirmed && account.confirmedAt && (
                        <div>
                          <span className="font-semibold">Confirmed:</span>{" "}
                          {formatDate(account.confirmedAt)}
                        </div>
                      )}
                      <div>
                        <span className="font-semibold">Register TX:</span>{" "}
                        <span className="text-primary">
                          {formatAddress(account.registerTxHash)}
                        </span>
                      </div>
                      {account.confirmTxHash && (
                        <div>
                          <span className="font-semibold">Confirm TX:</span>{" "}
                          <span className="text-primary">
                            {formatAddress(account.confirmTxHash)}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Confirm Button (only for unconfirmed) */}
                  {!account.isConfirmed && (
                    <Button
                      onClick={() =>
                        handleConfirmAccount(bytesToAddress(account.address))
                      }
                      disabled={
                        confirmingAddress ===
                        bytesToAddress(account.address)
                      }
                      size="sm"
                      className="ml-4"
                    >
                      {confirmingAddress ===
                      bytesToAddress(account.address) ? (
                        <>
                          <LoadingSpinnerIcon />
                          <span className="ml-2">Confirming...</span>
                        </>
                      ) : (
                        "Confirm"
                      )}
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Empty State */}
        {!isLoading && accounts.length === 0 && (
          <div className="text-center py-12 text-app-muted">
            <p>No {filter} accounts found.</p>
            <p className="text-xs mt-2">
              Try registering a BLS public key first.
            </p>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between mt-6 pt-6 border-t border-border">
            <Button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0 || isLoading}
              variant="outline"
            >
              Previous
            </Button>

            <span className="text-sm text-app-muted">
              Page {page + 1} of {totalPages} ({total} total accounts)
            </span>

            <Button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1 || isLoading}
              variant="outline"
            >
              Next
            </Button>
          </div>
        )}

        {/* Refresh Button */}
        <div className="mt-6">
          <Button
            onClick={loadAccounts}
            disabled={isLoading}
            variant="outline"
            className="w-full"
          >
            {isLoading ? "Loading..." : "Refresh"}
          </Button>
        </div>
      </PageCard>
    </PageContainer>
  );
}

export default BlsAccountListPage;
