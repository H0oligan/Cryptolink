import path from "path";
import {defineConfig} from "vite";
import react from "@vitejs/plugin-react";
import svgr from "vite-plugin-svgr";
import dynamicImport from "vite-plugin-dynamic-import";
import basicSsl from "@vitejs/plugin-basic-ssl";

// CryptoLink public release tag (x.y: 2.0, 2.1, … with x.0 for major updates).
// Single source of truth for the payment SPA — surfaces in the footer.
const CRYPTOLINK_VERSION = "2.3";

// https://vitejs.dev/config/
export default defineConfig({
    base: process.env.VITE_ROOTPATH || "/p/",
    resolve: {
        alias: {
            src: path.resolve(__dirname, "/src")
        }
    },
    define: {
        __CRYPTOLINK_VERSION__: JSON.stringify(CRYPTOLINK_VERSION)
    },
    // @ts-ignore
    plugins: [basicSsl(), svgr({include: "**/*.svg", svgrOptions: {exportType: "named"}}), dynamicImport(), react()]
});
