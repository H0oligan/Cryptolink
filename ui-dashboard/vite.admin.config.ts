import path from "path";
import {defineConfig, Plugin} from "vite";
import react from "@vitejs/plugin-react";
import dynamicImport from "vite-plugin-dynamic-import";
import basicSsl from "@vitejs/plugin-basic-ssl";
import svgr from "vite-plugin-svgr";

// Inject global polyfill as inline script instead of using define (which
// breaks axios by renaming its "global" export property key to "globalThis").
function globalPolyfill(): Plugin {
    return {
        name: "global-polyfill",
        transformIndexHtml(html) {
            return html.replace("<head>", '<head><script>window.global=globalThis;</script>');
        }
    };
}

// CryptoLink public release tag (single-decimal scheme: 1.0, 2.0, …).
// Single source of truth for the admin SPA — kept in sync with vite.config.ts.
const CRYPTOLINK_VERSION = "1.0";

// Admin panel SPA — built with base /admin/ and deployed to public_html/admin/
export default defineConfig({
    base: "/admin/",
    build: {
        outDir: "dist-admin",
        sourcemap: false,
        minify: "esbuild",
        rollupOptions: {
            input: "admin.html"
        }
    },
    resolve: {
        alias: {
            src: path.resolve(__dirname, "/src"),
            buffer: "buffer/"
        }
    },
    define: {
        __CRYPTOLINK_VERSION__: JSON.stringify(CRYPTOLINK_VERSION)
    },
    optimizeDeps: {
        esbuildOptions: {
            define: {
                global: "globalThis"
            }
        }
    },
    plugins: [basicSsl(), svgr(), dynamicImport(), react(), globalPolyfill()]
});
