import "./App.css";

import SharedHeader from "./components/SharedHeader";
import { Sidebar } from "./components/Sidebar";
import { Route, Routes } from "react-router-dom";
import BlsManagerPage from "./pages/BlsManager/BlsManagerPage";
import MetaMaskSigner from "./pages/MetaMaskSigner/MetaMaskSigner";
import HomePage from "./pages/Home/HomePage";
import AccountTypeManagerPage from "./pages/SetAccountType/AccountTypeManagerPage";
import BlsAccountListPage from "./pages/BlsAccountList/BlsAccountListPage";

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
    <div className="bg-app text-app min-h-screen flex flex-col transition-colors duration-300">
      <SharedHeader pageLinks={pageLinks} />
      
      <div className="flex flex-1">
        <Sidebar pageLinks={pageLinks} />
        
        <main className="flex-1 container mx-auto px-2 sm:px-4">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="bls" element={<BlsManagerPage />} />
            <Route path="account-type" element={<AccountTypeManagerPage />} />
            <Route path="register-rpc" element={<MetaMaskSigner />} />
            <Route path="accounts" element={<BlsAccountListPage />} />
            <Route path="*" element={<h1>Not found</h1>} />
          </Routes>
        </main>
      </div>

      <footer className="text-center py-6 text-xs text-app-muted bg-app-secondary mt-auto transition-colors duration-300">
        <p>An account management application.</p>
      </footer>
    </div>
  );
}

export default App;
