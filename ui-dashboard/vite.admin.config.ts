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
    optimizeDeps: {
        esbuildOptions: {
            define: {
                global: "globalThis"
            }
        }
    },
    plugins: [basicSsl(), svgr(), dynamicImport(), react(), globalPolyfill()]
});
