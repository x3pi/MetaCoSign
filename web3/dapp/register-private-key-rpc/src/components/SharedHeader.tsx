// src/components/SharedHeader.tsx
import React, { useState, useEffect, useRef } from "react";
import { Link } from "react-router-dom";
import { useWallet } from "../contexts/WalletContext";
import { chain991 } from "~/constants/customChain";
import type { PageLink } from "../App";
import { ThemeToggle } from "./ThemeToggle";

const LoadingSpinnerIcon = () => (
  <svg
    className="animate-spin h-5 w-5 text-white"
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    viewBox="0 0 24 24"
  >
    <circle
      className="opacity-25"
      cx="12"
      cy="12"
      r="10"
      stroke="currentColor"
      strokeWidth="4"
    ></circle>
    <path
      className="opacity-75"
      fill="currentColor"
      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
    ></path>
  </svg>
);

const CloseIcon = () => (
  <svg
    className="h-4 w-4"
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    aria-hidden="true"
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      d="M6 18L18 6M6 6l12 12"
    />
  </svg>
);

interface SharedHeaderProps {
  pageLinks: PageLink[];
}

const SharedHeader: React.FC<SharedHeaderProps> = () => {
  const {
    connectedAccount,
    isConnecting,
    currentChainId,
    connectWallet,
    disconnectWallet,
    switchNetwork,
    error: walletError,
    status: walletStatus,
    clearError: clearWalletError,
    setStatusMessage: setWalletStatusMessage,
  } = useWallet();

  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const handleSwitchToChain991 = () => {
    switchNetwork(chain991.id);
    setIsMobileMenuOpen(false);
  };

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        menuRef.current &&
        !menuRef.current.contains(event.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsMobileMenuOpen(false);
      }
    };
    if (isMobileMenuOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isMobileMenuOpen]);

  // === 1. ĐỊNH NGHĨA STYLE M3 BẰNG TAILWIND ===

  // Style chung cho các nút/pills bên phải: bo tròn `rounded-full`
  const walletPillBaseStyle =
    "px-4 py-2 rounded-full text-xs font-medium shadow-sm transition-all duration-150 ease-in-out focus:outline-none focus:ring-2 focus:ring-opacity-75 flex items-center justify-center";

  // Nút Connect: "Filled Button" (M3 Primary)
  const connectWalletButtonStyle =
    "bg-teal-600 hover:bg-teal-700 text-white ring-teal-500";

  // Nút Disconnect: "Tonal Button" (M3 Surface-Variant)
  const disconnectWalletButtonStyle =
    "bg-neutral-800 hover:bg-neutral-700 text-neutral-300 ring-neutral-600";

  // Nút Switch Network (M3 Error/Warning Color)
  const switchNetworkButtonStyle =
    "bg-yellow-500 hover:bg-yellow-600 text-neutral-900 ring-yellow-400";

  return (
    // === 2. HEADER: M3 "Top App Bar" (Surface + Elevation) ===
    // Bỏ border-b, dùng shadow-lg để tạo độ cao
    <header className="bg-app shadow-lg sticky top-0 z-50 transition-colors duration-300">
      <div className="container mx-auto px-2 sm:px-4">
        <div className="relative flex items-center justify-between h-16">
          {/* Logo (M3 Primary Color) */}
          <div className="shrink-0">
            <Link
              to="/"
              className="text-xl font-bold text-primary cursor-pointer hover:opacity-80 transition-opacity"
              onClick={() => setIsMobileMenuOpen(false)}
            >
              Account Manager
            </Link>
          </div>

          {/* === 3. CỤM VÍ PHIÊN BẢN DESKTOP (M3 Shape: rounded-full) === */}
          <div className="hidden lg:flex lg:ml-auto lg:items-center lg:space-x-2">
            <ThemeToggle />

            {connectedAccount ? (
              <>
                {/* Pill cho Địa chỉ và Chain ID (M3 Tonal) */}
                <div className="bg-neutral-800 rounded-full px-4 py-2 text-xs flex items-center space-x-2">
                  <span className="text-neutral-300">
                    {`${connectedAccount.substring(
                      0,
                      6
                    )}...${connectedAccount.substring(
                      connectedAccount.length - 4
                    )}`}
                  </span>
                  <span
                    className={`px-1.5 py-0.5 rounded-full text-white text-[10px] ${
                      currentChainId === chain991.id
                        ? "bg-green-500"
                        : "bg-yellow-500 text-neutral-900"
                    }`}
                  >
                    ID: {currentChainId ?? "N/A"}
                  </span>
                </div>

                {currentChainId !== null && currentChainId !== chain991.id && (
                  <button
                    onClick={handleSwitchToChain991}
                    className={`${walletPillBaseStyle} ${switchNetworkButtonStyle} py-2`}
                  >
                    Switch
                  </button>
                )}

                <button
                  onClick={() => disconnectWallet()}
                  className={`${walletPillBaseStyle} ${disconnectWalletButtonStyle}`}
                >
                  Disconnect
                </button>
              </>
            ) : (
              // Nút Connect (M3 Filled Button)
              <button
                onClick={() => connectWallet()}
                disabled={isConnecting}
                className={`${walletPillBaseStyle} ${connectWalletButtonStyle}`}
              >
                {isConnecting && <LoadingSpinnerIcon />}
                <span className={isConnecting ? "ml-1.5" : ""}>
                  Connect Wallet
                </span>
              </button>
            )}
          </div>

          {/* Nút Hamburger cho Mobile (M3 Icon Button) */}
          <div className="flex items-center ml-2 lg:hidden gap-2">
            <ThemeToggle />

            <button
              ref={buttonRef}
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
              type="button"
              // Style tròn, Tonal
              className="bg-neutral-800 hover:bg-neutral-700 text-neutral-300 rounded-full w-10 h-10 flex items-center justify-center focus:outline-none focus:ring-2 focus:ring-neutral-600"
              aria-controls="mobile-menu"
              aria-expanded={isMobileMenuOpen}
            >
              <span className="sr-only">Open main menu</span>
              {isMobileMenuOpen ? (
                <svg
                  className="block h-5 w-5"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              ) : (
                <svg
                  className="block h-5 w-5"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M4 6h16M4 12h16M4 18h16"
                  />
                </svg>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* === 5. THANH STATUS (M3 "Snackbar" Component) === */}
      {/* Giao diện này vốn đã rất M3, giữ nguyên */}
      {(walletStatus || walletError) && (
        <div className="container mx-auto px-2 sm:px-4 pb-1">
          {walletStatus && (
            <div className="relative mt-1 flex items-center justify-between text-xs p-2.5 bg-sky-800/90 border border-sky-700/90 text-sky-200 rounded-md shadow-lg">
              <span>{walletStatus}</span>
              <button
                onClick={() => setWalletStatusMessage(null)}
                className="ml-3 shrink-0 bg-black/20 hover:bg-black/40 rounded-full p-1 -mr-1 -my-1"
              >
                <span className="sr-only">Close</span>
                <CloseIcon />
              </button>
            </div>
          )}
          {walletError && (
            <div className="relative mt-1 flex items-center justify-between text-xs p-2.5 bg-red-800/90 border border-red-700/90 text-red-200 rounded-md shadow-lg">
              <span>{walletError}</span>
              <button
                onClick={clearWalletError}
                className="ml-3 shrink-0 bg-black/20 hover:bg-black/40 rounded-full p-1 -mr-1 -my-1"
              >
                <span className="sr-only">Close</span>
                <CloseIcon />
              </button>
            </div>
          )}
        </div>
      )}
    </header>
  );
};

export default SharedHeader;
