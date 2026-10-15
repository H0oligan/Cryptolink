import path from "path";
import {defineConfig} from "vitest/config";

export default defineConfig({
    resolve: {alias: {src: path.resolve(__dirname, "src")}},
    test: {environment: "node", include: ["src/**/__tests__/**/*.test.ts"]}
});
