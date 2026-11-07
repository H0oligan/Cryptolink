import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import {describe, expect, it} from "vitest";

// #root-portal is an always-present, full-screen fixed layer that sits above the
// checkout. If it takes pointer events, the page looks fine but nothing can be clicked.
describe("root-portal overlay", () => {
    it("lets clicks through to the checkout", () => {
        const html = readFileSync(resolve(__dirname, "../../index.html"), "utf8");
        const portal = html.match(/<div id="root-portal" class="([^"]*)"/);

        expect(portal?.[1].split(/\s+/)).toContain("pointer-events-none");
    });

    it("keeps the error alert itself interactive", () => {
        const source = readFileSync(resolve(__dirname, "../components/ErrorAlert.tsx"), "utf8");

        expect(source).toContain("pointer-events-auto");
    });
});
