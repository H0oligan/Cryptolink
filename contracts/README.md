# CryptoLink Smart Contracts

These Solidity contracts implement CryptoLink's non-custodial collection layer for EVM chains and TRON. **They are intentionally public** — publishing the source is what lets every operator and merchant verify the contracts have no backdoor.

## What's in here

| File | Purpose |
|---|---|
| [`MerchantCollectorV2.sol`](./MerchantCollectorV2.sol) | Per-merchant collector. Holds incoming funds; only the immutable `owner` (set once at clone init) can withdraw. Used as the implementation behind every EIP-1167 clone. |
| [`CryptoLinkCloneFactory.sol`](./CryptoLinkCloneFactory.sol) | EIP-1167 minimal-proxy factory. Deploys cheap clones (~45 bytes) of `MerchantCollectorV2` for each merchant. |
| [`MerchantCollector.sol`](./MerchantCollector.sol) | Original non-proxy variant, kept for reference. Same security model but more expensive to deploy per merchant. |
| [`abi/MerchantCollector.json`](./abi/MerchantCollector.json) | ABI used by the dashboard frontend to talk to deployed clones. |

## What's NOT in here (and never will be)

- **No hardcoded wallet addresses** — not the CryptoLink operator's, not any merchant's.
- **No default deployments** — addresses for any specific deployment of these contracts are deliberately *not* shipped in this repo. Every operator deploys their own factory; every merchant deploys their own clone via the dashboard. Each operator's deployer wallet is theirs alone.
- **No admin/upgrade keys** — the factory has no power over deployed clones. Each clone's `owner` is set at init time and cannot be changed.

## Who supplies the wallet, and where

| Actor | Wallet they connect | Where they connect it |
|---|---|---|
| **Super-admin (operator)** | The wallet that pays gas to deploy the implementation + factory once per chain | **Admin Panel → `/admin/contracts`** (TronLink / MetaMask popup at deploy time) |
| **Merchant** | The wallet that will own incoming funds and call `withdrawAll()` | **Merchant Panel → Wallet Setup** for each chain (TronLink / MetaMask popup deploys the clone with their address as `owner`) |

Neither wallet is stored in this repository. Both are supplied **interactively at deploy time** through a browser wallet extension (TronLink for TRON, MetaMask or any EIP-1193 wallet for EVM). The address is passed as a constructor / `initialize(address _owner)` parameter — that's the only way an address ever enters one of these contracts.

## Compilation

When you re-compile to verify the bytecode against an on-chain deployment:

| Setting | Value |
|---|---|
| Compiler | `tron_v0.8.25+commit.77bd169f` (TRON solc fork) for TRON; standard solc 0.8.25 for EVM |
| Optimizer | enabled, 200 runs |
| `evmVersion` | `paris` (do **not** use Shanghai — bytecode will not match) |
| viaIR | off |

For TRON specifically, you must use the TRON solc fork — standard Ethereum solc emits slightly different bytecode (it's missing TRON-specific d2/d3 opcodes) and verification on Tronscan will fail.

## Verifying on a block explorer

1. Compile the source with the settings above.
2. Submit `MerchantCollectorV2.sol` and `CryptoLinkCloneFactory.sol` to Etherscan (or chain equivalent) / Tronscan.
3. EIP-1167 clones (the per-merchant collectors) are 45-byte proxies — block explorers detect them automatically and show the implementation's interface. **Do not** try to "verify" individual clones; only the implementation needs verification.

## Trust model in one sentence

CryptoLink the platform never has the ability to move a merchant's funds — every contract here either has no fund-handling function at all (the factory) or restricts withdrawals to an `owner` set at deploy time by the merchant themselves (the collector).
