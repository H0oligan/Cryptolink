import js from "@eslint/js";
import tsPlugin from "@typescript-eslint/eslint-plugin";
import prettierRecommended from "eslint-plugin-prettier/recommended";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";

export default [
    {ignores: ["build/**", "public/**", "node_modules/**", "vite.config.ts"]},
    js.configs.recommended,
    ...tsPlugin.configs["flat/recommended-type-checked"],
    prettierRecommended,
    {
        files: ["src/**/*.{ts,tsx}"],
        plugins: {react, "react-hooks": reactHooks},
        languageOptions: {parserOptions: {project: ["./tsconfig.json"]}},
        rules: {
            "no-await-in-loop": "error",
            "no-template-curly-in-string": "error",
            "no-else-return": "error",
            "no-new-wrappers": "error",
            "no-useless-catch": "error",
            "lines-between-class-members": [
                "error",
                "always",
                {
                    exceptAfterSingleLine: true
                }
            ],
            "no-lonely-if": "error",
            "no-multi-assign": "error",
            "no-unneeded-ternary": "error",
            "object-shorthand": ["error", "properties"],
            "prefer-object-spread": "error",
            "no-duplicate-imports": "error",
            "@typescript-eslint/member-ordering": [
                "error",
                {
                    default: [
                        "static-field",
                        "private-instance-field",
                        "public-instance-field",
                        "instance-field",
                        "constructor",
                        "public-instance-method"
                    ]
                }
            ],
            "@typescript-eslint/no-explicit-any": "error",
            "@typescript-eslint/no-inferrable-types": [
                "error",
                {
                    ignoreParameters: false,
                    ignoreProperties: false
                }
            ],
            "@typescript-eslint/consistent-type-assertions": [
                "error",
                {
                    assertionStyle: "as",
                    objectLiteralTypeAssertions: "allow-as-parameter"
                }
            ],
            "@typescript-eslint/triple-slash-reference": [
                "error",
                {
                    types: "prefer-import"
                }
            ],
            "@typescript-eslint/no-unused-vars": "error",
            "@typescript-eslint/no-use-before-define": [
                "error",
                {
                    functions: false
                }
            ],
            "@typescript-eslint/no-useless-constructor": "error",
            "@typescript-eslint/no-unused-expressions": "error",
            "@typescript-eslint/prefer-function-type": "error",
            "line-comment-position": "off",
            "no-unused-expressions": "off",
            "require-atomic-updates": "off",
            "no-prototype-builtins": "off",
            "no-useless-constructor": "off",
            "no-use-before-define": "off",
            "capitalized-comments": "off",
            "@typescript-eslint/require-await": "off",
            "@typescript-eslint/explicit-member-accessibility": "off",
            "@typescript-eslint/explicit-function-return-type": "off",
            camelcase: "off",
            "@typescript-eslint/no-non-null-assertion": "off",
            "@typescript-eslint/no-misused-promises": "off",
            "@typescript-eslint/no-empty-function": "off",
            "@typescript-eslint/unbound-method": "off",
            "@typescript-eslint/prefer-regexp-exec": "off",
            "no-dupe-class-members": "off",
            "@typescript-eslint/naming-convention": [
                "error",
                {
                    selector: "interface",
                    format: ["PascalCase"],
                    custom: {
                        regex: "^I[A-Z]",
                        match: false
                    }
                }
            ],
            "@typescript-eslint/no-floating-promises": "off",
            "@typescript-eslint/no-implied-eval": "off",
            "@typescript-eslint/no-unsafe-assignment": "off",
            "@typescript-eslint/no-unsafe-call": "off",
            "@typescript-eslint/no-unsafe-member-access": "off",
            "@typescript-eslint/no-unsafe-return": "off",
            "@typescript-eslint/restrict-plus-operands": "off",
            "@typescript-eslint/restrict-template-expressions": "off",
            "@typescript-eslint/explicit-module-boundary-types": "off",
            "@typescript-eslint/prefer-as-const": "off",
            "@typescript-eslint/no-non-null-asserted-optional-chain": "off"
        }
    }
];
