// src/components/SharedHeader.tsx
import React from "react";
import { Link } from "react-router-dom";
import { useWallet } from "../contexts/WalletContext";
import { useMobileMenu } from "../contexts/MobileMenuContext";
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

// Mobile menu button component
const MobileMenuButton: React.FC = () => {
  const { isMobileMenuOpen, toggleMobileMenu } = useMobileMenu();
  
  return (
    <button
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        console.log('Mobile menu button clicked, current state:', isMobileMenuOpen);
        toggleMobileMenu();
      }}
      className="md:hidden w-10 h-10 bg-primary hover:bg-primary-hover rounded-lg flex items-center justify-center text-white transition-all duration-150 shadow-md"
      aria-label="Toggle menu"
    >
      <div className="w-5 h-5 flex flex-col justify-center items-center">
        <span className={`bg-white block transition-all duration-300 ease-out h-0.5 w-4 rounded-sm ${isMobileMenuOpen ? 'rotate-45 translate-y-1' : '-translate-y-0.5'}`}></span>
        <span className={`bg-white block transition-all duration-300 ease-out h-0.5 w-4 rounded-sm my-0.5 ${isMobileMenuOpen ? 'opacity-0' : 'opacity-100'}`}></span>
        <span className={`bg-white block transition-all duration-300 ease-out h-0.5 w-4 rounded-sm ${isMobileMenuOpen ? '-rotate-45 -translate-y-1' : 'translate-y-0.5'}`}></span>
      </div>
    </button>
  );
};

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

  const handleSwitchToChain991 = () => {
    switchNetwork(chain991.id);
  };

  // === 1. ĐỊNH NGHĨA STYLE M3 BẰNG TAILWIND ===

  // Style chung cho các nút/pills bên phải: bo tròn `rounded-full`
  const walletPillBaseStyle =
    "px-4 py-2 rounded-full text-xs font-medium shadow-sm transition-all duration-150 ease-in-out focus:outline-none focus:ring-2 focus:ring-opacity-75 flex items-center justify-center";

  // Nút Connect: "Filled Button" (M3 Primary)
  const connectWalletButtonStyle =
    "bg-primary hover:bg-primary-hover text-white ring-primary";

  // Nút Disconnect: "Tonal Button" (M3 Surface-Variant)
  const disconnectWalletButtonStyle =
    "bg-app-secondary hover:bg-app-tertiary text-app-secondary ring-border";

  // Nút Switch Network (M3 Error/Warning Color)
  const switchNetworkButtonStyle =
    "bg-warning hover:bg-warning text-white ring-warning";

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
                <div className="bg-app-secondary rounded-full px-4 py-2 text-xs flex items-center space-x-2">
                  <span className="text-app-secondary">
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
                        ? "bg-success"
                        : "bg-warning text-white"
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

          {/* Mobile menu button and theme toggle */}
          <div className="flex items-center ml-2 lg:hidden gap-2">
            <ThemeToggle />
            {/* Mobile menu button from context */}
            <MobileMenuButton />
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
