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

// https://vitejs.dev/config/
export default defineConfig({
    base: process.env.VITE_ROOTPATH || "/dashboard/",
    build: {
        sourcemap: false,
        minify: "esbuild"
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
