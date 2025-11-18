// src/components/Sidebar.tsx
import React, { useState } from 'react';
import { NavLink } from 'react-router-dom';
import type { PageLink } from '../App';

interface SidebarProps {
  pageLinks: PageLink[];
}

export const Sidebar: React.FC<SidebarProps> = ({ pageLinks }) => {
  const [isCollapsed, setIsCollapsed] = useState(false);

  return (
    <>
      {/* Sidebar */}
      <aside
        className={`fixed left-0 top-16 h-[calc(100vh-4rem)] bg-app-secondary border-r border-app-border shadow-xl z-40 transition-all duration-300 flex flex-col ${
          isCollapsed ? 'w-16' : 'w-64'
        }`}
      >
        {/* Toggle Button */}
        <button
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="absolute -right-3 top-6 w-6 h-6 bg-teal-600 hover:bg-teal-700 rounded-full flex items-center justify-center text-white shadow-lg transition-all duration-150 z-50"
          title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          <span className="text-xs">{isCollapsed ? '→' : '←'}</span>
        </button>

        {/* Navigation Links */}
        <nav className="flex-1 px-3 py-6 space-y-1 overflow-y-auto">
          {!isCollapsed && (
            <div className="px-3 mb-4">
              <h2 className="text-xs font-bold uppercase tracking-wider text-app-muted">
                Navigation
              </h2>
            </div>
          )}

          {pageLinks.map((link) => (
            <NavLink
              key={link.path}
              to={link.path}
              title={isCollapsed ? link.label : undefined}
              className={({ isActive }) => {
                const baseClass = `flex items-center px-4 py-3 rounded-lg text-sm font-medium transition-all duration-150 ease-in-out ${
                  isCollapsed ? 'justify-center' : ''
                }`;
                if (isActive) {
                  return `${baseClass} bg-teal-600/20 text-teal-300 border-l-4 border-teal-500`;
                }
                return `${baseClass} text-app hover:bg-neutral-800/50`;
              }}
            >
              {({ isActive }) => (
                <>
                  {!isCollapsed && (
                    <>
                      {isActive && (
                        <div className="w-1 h-1 rounded-full mr-3 bg-teal-400" />
                      )}
                      {link.label}
                    </>
                  )}
                  {isCollapsed && (
                    <span className="text-lg">
                      {link.path === '/' && '🏠'}
                      {link.path === '/bls' && '🔐'}
                      {link.path === '/account-type' && '⚙️'}
                      {link.path === '/register-rpc' && '📝'}
                    </span>
                  )}
                </>
              )}
            </NavLink>
          ))}
        </nav>

        {/* Divider */}
        <div className="border-t border-app-border" />

        {/* Selected Account Info */}
    
      </aside>

      {/* Spacer để content không bị sidebar che */}
      <div
        className={`transition-all duration-300 ${
          isCollapsed ? 'w-16' : 'w-64'
        }`}
      />
    </>
  );
};
