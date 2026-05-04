/// <reference types="vite/client" />

// Injected by vite at build time from vite.config.ts / vite.admin.config.ts.
// Single source of truth for the public CryptoLink release tag (single-decimal
// scheme: 1.0, 2.0, …). Surfaces in the dashboard sidebar logo.
declare const __CRYPTOLINK_VERSION__: string;
