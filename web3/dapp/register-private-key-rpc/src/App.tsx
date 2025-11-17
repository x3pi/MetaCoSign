import "./App.css";

import SharedHeader from "./components/SharedHeader";
import { Route, Routes } from "react-router-dom";
import BlsManagerPage from "./pages/BlsManager/BlsManagerPage";
import MetaMaskSigner from "./pages/MetaMaskSigner/MetaMaskSigner";
import HomePage from "./pages/Home/HomePage";
import AccountTypeManagerPage from "./pages/SetAccountType/AccountTypeManagerPage";

export interface PageLink {
  path: string;
  label: string;
}

function App() {
  const pageLinks: PageLink[] = [
    { path: "/", label: "Publickey BLS" }, //
    { path: "/account-type", label: "AccountType" },
    { path: "/register-rpc", label: "Register BLS Rpc" }, //
  ];

  return (
    <div className="bg-app text-app min-h-screen flex flex-col transition-colors duration-300">
      <SharedHeader pageLinks={pageLinks} />
      <main className="grow container mx-auto px-2 sm:px-4">
        {/* 7. Sử dụng Routes và Route để định nghĩa các trang */}
        <Routes>
          <Route path="" element={<HomePage />} />
          <Route path="bls" element={<BlsManagerPage />} />
          <Route path="account-type" element={<AccountTypeManagerPage />} />
          <Route path="register-rpc" element={<MetaMaskSigner />} />
          <Route path="*" element={<h1>Not found</h1>} />
        </Routes>
      </main>

      <footer className="text-center py-6 text-xs text-app-muted bg-app-secondary mt-auto transition-colors duration-300">
        <p>An account management application.</p>
      </footer>
    </div>
  );
}

export default App;
