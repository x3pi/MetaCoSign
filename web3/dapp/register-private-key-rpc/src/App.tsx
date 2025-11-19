import "./App.css";

import SharedHeader from "./components/SharedHeader";
import { Sidebar } from "./components/Sidebar";
import { Route, Routes } from "react-router-dom";
import BlsManagerPage from "./pages/BlsManager/BlsManagerPage";
import MetaMaskSigner from "./pages/MetaMaskSigner/MetaMaskSigner";
import HomePage from "./pages/Home/HomePage";
import AccountTypeManagerPage from "./pages/SetAccountType/AccountTypeManagerPage";
import BlsAccountListPage from "./pages/BlsAccountList/BlsAccountListPage";
import { MobileMenuProvider } from "./contexts/MobileMenuContext";
import { ThemeProvider } from "./contexts/ThemeContext";

export interface PageLink {
  path: string;
  label: string;
}

function App() {
  const pageLinks: PageLink[] = [
    { path: "/", label: "Home" },
    { path: "/bls", label: "Publickey BLS" },
    { path: "/account-type", label: "AccountType" },
    { path: "/register-rpc", label: "Register BLS Rpc" },
    { path: "/accounts", label: "Account List" },
  ];

  return (
    <ThemeProvider>
      <MobileMenuProvider>
        <div className="bg-app text-app min-h-screen flex flex-col transition-colors duration-300">
          {/* Header */}
          <SharedHeader pageLinks={pageLinks} />
          
          {/* Main layout */}
          <div className="flex flex-1 relative">
            {/* Sidebar component */}
            <Sidebar pageLinks={pageLinks} />
            
            {/* Main content - responsive margin/padding */}
            <main className="flex-1 w-full md:w-auto px-2 sm:px-4 pt-4 md:pt-0">
              <div className="container mx-auto">
                <Routes>
                  <Route path="/" element={<HomePage />} />
                  <Route path="bls" element={<BlsManagerPage />} />
                  <Route path="account-type" element={<AccountTypeManagerPage />} />
                  <Route path="register-rpc" element={<MetaMaskSigner />} />
                  <Route path="accounts" element={<BlsAccountListPage />} />
                  <Route path="*" element={<h1>Not found</h1>} />
                </Routes>
              </div>
            </main>
          </div>

        </div>
      </MobileMenuProvider>
    </ThemeProvider>
  );
}

export default App;
