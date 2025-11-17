// src/components/SharedHeader.tsx
import React, { useState, useEffect, useRef } from "react";
import { NavLink, Link } from "react-router-dom";
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

const SharedHeader: React.FC<SharedHeaderProps> = ({ pageLinks }) => {
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

  // Style chung cho các nút nav: bo tròn `rounded-full` (M3 Shape)
  const navButtonBaseStyle =
    "px-4 py-2 rounded-full text-sm font-medium transition-colors duration-150 ease-in-out";

  // Style cho nút active: "Tonal Pill" (M3 Secondary Container)
  const activeNavButtonStyle = "bg-teal-600/30 text-teal-300 font-semibold";

  // Style cho nút inactive: Chỉ là text (M3 On-Surface-Variant)
  const inactiveNavButtonStyle =
    "text-neutral-400 hover:bg-white/10 hover:text-neutral-200";

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
              Account
            </Link>
          </div>

          {/* Điều hướng Desktop (M3 Navigation Style) */}
          <div className="hidden lg:flex lg:ml-6">
            <div className="flex space-x-1">
              {" "}
              {/* Giảm khoảng cách */}
              {pageLinks.map((link) => (
                <NavLink
                  key={link.path}
                  to={link.path}
                  className={({ isActive }) =>
                    `${navButtonBaseStyle} ${
                      isActive ? activeNavButtonStyle : inactiveNavButtonStyle
                    }`
                  }
                >
                  {link.label}
                </NavLink>
              ))}
            </div>
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

      {/* === 4. MENU MOBILE (M3 Navigation Drawer Style) === */}
      {isMobileMenuOpen && (
        <>
          {/* Overlay (Dark scrim) */}
          <div
            className="lg:hidden fixed inset-0 top-16 bg-black/40 z-30 transition-opacity duration-200"
            onClick={() => setIsMobileMenuOpen(false)}
          />

          {/* Sidebar Drawer */}
          <div
            ref={menuRef}
            className="lg:hidden fixed top-16 left-0 h-[calc(100vh-4rem)] w-64 bg-neutral-900 border-r border-neutral-800 shadow-2xl z-40 overflow-y-auto flex flex-col"
            id="mobile-menu"
          >
            {/* Navigation Links Section */}
            <div className="flex-1 px-3 py-6 space-y-1">
              {/* Section Header */}
              <div className="px-3 mb-4">
                <h2 className="text-xs font-bold uppercase tracking-wider text-neutral-500">
                  Navigation
                </h2>
              </div>

              {/* Nav Links */}
              {pageLinks.map((link) => (
                <NavLink
                  key={link.path}
                  to={link.path}
                  onClick={() => setIsMobileMenuOpen(false)}
                  className={({ isActive }) => {
                    const baseClass =
                      "flex items-center px-4 py-3 rounded-lg text-sm font-medium transition-all duration-150 ease-in-out relative";
                    if (isActive) {
                      return `${baseClass} bg-teal-600/20 text-teal-300 border-l-4 border-teal-500`;
                    }
                    return `${baseClass} text-neutral-300 hover:bg-neutral-800/50`;
                  }}
                >
                  <div className="w-1 h-1 rounded-full mr-3 bg-teal-400 opacity-0 group-hover:opacity-100 transition-opacity" />
                  {link.label}
                </NavLink>
              ))}
            </div>

            {/* Divider */}
            <div className="border-t border-neutral-800" />

            {/* Account Section */}
            <div className="px-3 py-6 space-y-4">
              {/* Section Header */}
              <h2 className="px-3 text-xs font-bold uppercase tracking-wider text-neutral-500">
                Account
              </h2>

              {/* Account Info Card */}
              {connectedAccount ? (
                <div className="mx-3 bg-linear-to-br from-teal-600/10 to-teal-600/5 rounded-xl p-4 border border-teal-600/20 space-y-3">
                  <div>
                    <p className="text-xs text-neutral-400 font-medium mb-1">
                      Wallet Address
                    </p>
                    <p className="font-mono text-xs text-teal-300 break-all">
                      {`${connectedAccount.substring(
                        0,
                        6
                      )}...${connectedAccount.substring(
                        connectedAccount.length - 4
                      )}`}
                    </p>
                  </div>

                  <div className="bg-neutral-800/30 rounded-lg px-3 py-2 flex items-center justify-between">
                    <span className="text-xs text-neutral-400">Chain ID</span>
                    <span
                      className={`px-2 py-1 rounded-full text-[10px] font-semibold ${
                        currentChainId === chain991.id
                          ? "bg-green-500/20 text-green-300"
                          : "bg-yellow-500/20 text-yellow-300"
                      }`}
                    >
                      {currentChainId ?? "N/A"}
                    </span>
                  </div>
                </div>
              ) : (
                <div className="mx-3 bg-neutral-800/30 rounded-xl p-4 border border-neutral-700/50 text-center py-6">
                  <p className="text-sm text-neutral-400">
                    No wallet connected
                  </p>
                </div>
              )}

              {/* Action Buttons */}
              <div className="mx-3 space-y-2">
                {connectedAccount &&
                  currentChainId !== null &&
                  currentChainId !== chain991.id && (
                    <button
                      onClick={handleSwitchToChain991}
                      className="w-full flex items-center justify-center px-4 py-3 rounded-lg bg-yellow-500/10 hover:bg-yellow-500/20 text-yellow-300 border border-yellow-500/30 text-sm font-medium transition-all duration-150"
                    >
                      🔗 Switch to {chain991.name}
                    </button>
                  )}

                {!connectedAccount ? (
                  <button
                    onClick={() => {
                      connectWallet();
                      setIsMobileMenuOpen(false);
                    }}
                    disabled={isConnecting}
                    className="w-full flex items-center justify-center px-4 py-3 rounded-lg bg-teal-600 hover:bg-teal-700 disabled:bg-teal-600/50 text-white text-sm font-semibold transition-all duration-150 shadow-lg space-x-2"
                  >
                    {isConnecting && <LoadingSpinnerIcon />}
                    <span>
                      {isConnecting ? "Connecting..." : "Connect Wallet"}
                    </span>
                  </button>
                ) : (
                  <button
                    onClick={() => {
                      disconnectWallet();
                      setIsMobileMenuOpen(false);
                    }}
                    className="w-full flex items-center justify-center px-4 py-3 rounded-lg bg-neutral-800 hover:bg-neutral-700 text-neutral-300 text-sm font-medium transition-all duration-150 border border-neutral-700"
                  >
                    🚪 Disconnect
                  </button>
                )}
              </div>
            </div>
          </div>
        </>
      )}

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
