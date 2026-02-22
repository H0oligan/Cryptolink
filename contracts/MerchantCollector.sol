// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title MerchantCollector
 * @notice Non-custodial payment collector for CryptoLink merchants.
 *         Deployed once per merchant per EVM chain.
 *         Only the immutable `owner` (merchant's MetaMask address) can withdraw.
 *         CryptoLink has NO admin function or any access to funds.
 */
contract MerchantCollector {
    address public immutable owner;

    event Received(address indexed from, uint256 amount);
    event WithdrewNative(address indexed to, uint256 amount);
    event WithdrewToken(address indexed token, address indexed to, uint256 amount);

    constructor(address _owner) {
        require(_owner != address(0), "zero owner");
        owner = _owner;
    }

    receive() external payable {
        emit Received(msg.sender, msg.value);
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    /**
     * @notice Withdraw all native coin (ETH/MATIC/BNB/AVAX) to owner.
     */
    function withdrawNative() external onlyOwner {
        uint256 bal = address(this).balance;
        require(bal > 0, "no native balance");
        (bool ok, ) = owner.call{value: bal}("");
        require(ok, "native transfer failed");
        emit WithdrewNative(owner, bal);
    }

    /**
     * @notice Withdraw a single ERC-20 token to owner.
     * @param token ERC-20 contract address (e.g. USDT, USDC)
     */
    function withdrawToken(address token) external onlyOwner {
        require(token != address(0), "zero token");
        uint256 bal = _tokenBalance(token);
        require(bal > 0, "no token balance");
        _safeTransfer(token, owner, bal);
        emit WithdrewToken(token, owner, bal);
    }

    /**
     * @notice Withdraw all native coin + multiple ERC-20 tokens in one tx.
     * @param tokens Array of ERC-20 contract addresses to sweep.
     */
    function withdrawAll(address[] calldata tokens) external onlyOwner {
        // Native
        uint256 native = address(this).balance;
        if (native > 0) {
            (bool ok, ) = owner.call{value: native}("");
            require(ok, "native transfer failed");
            emit WithdrewNative(owner, native);
        }
        // Tokens
        for (uint256 i = 0; i < tokens.length; i++) {
            address token = tokens[i];
            if (token == address(0)) continue;
            uint256 bal = _tokenBalance(token);
            if (bal > 0) {
                _safeTransfer(token, owner, bal);
                emit WithdrewToken(token, owner, bal);
            }
        }
    }

    // ── Internals ──────────────────────────────────────────────────────────

    function _tokenBalance(address token) internal view returns (uint256) {
        (bool success, bytes memory data) = token.staticcall(
            abi.encodeWithSignature("balanceOf(address)", address(this))
        );
        if (!success || data.length < 32) return 0;
        return abi.decode(data, (uint256));
    }

    function _safeTransfer(address token, address to, uint256 amount) internal {
        (bool success, bytes memory data) = token.call(
            abi.encodeWithSignature("transfer(address,uint256)", to, amount)
        );
        require(success && (data.length == 0 || abi.decode(data, (bool))), "token transfer failed");
    }
}
