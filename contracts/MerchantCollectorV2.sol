// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title MerchantCollectorV2
 * @notice Proxy-compatible version of MerchantCollector for EIP-1167 clone pattern.
 *         Uses initialize() instead of constructor so clones can set their own owner.
 *         Each clone has its own storage — funds are completely isolated per merchant.
 *         CryptoLink has NO admin function or any access to funds.
 */
contract MerchantCollectorV2 {
    address public owner;
    bool private _initialized;

    event Received(address indexed from, uint256 amount);
    event WithdrewNative(address indexed to, uint256 amount);
    event WithdrewToken(address indexed token, address indexed to, uint256 amount);

    /**
     * @notice Initialize the clone with the merchant's wallet as owner.
     *         Can only be called once (enforced by _initialized flag).
     *         Called automatically by the CloneFactory during deployment.
     */
    function initialize(address _owner) external {
        require(!_initialized, "already init");
        require(_owner != address(0), "zero owner");
        owner = _owner;
        _initialized = true;
    }

    receive() external payable {
        emit Received(msg.sender, msg.value);
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    function withdrawNative() external onlyOwner {
        uint256 bal = address(this).balance;
        require(bal > 0, "no native balance");
        (bool ok, ) = owner.call{value: bal}("");
        require(ok, "native transfer failed");
        emit WithdrewNative(owner, bal);
    }

    function withdrawToken(address token) external onlyOwner {
        require(token != address(0), "zero token");
        uint256 bal = _tokenBalance(token);
        require(bal > 0, "no token balance");
        _safeTransfer(token, owner, bal);
        emit WithdrewToken(token, owner, bal);
    }

    function withdrawAll(address[] calldata tokens) external onlyOwner {
        uint256 native = address(this).balance;
        if (native > 0) {
            (bool ok, ) = owner.call{value: native}("");
            require(ok, "native transfer failed");
            emit WithdrewNative(owner, native);
        }
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

    function _tokenBalance(address token) internal view returns (uint256) {
        (bool success, bytes memory data) = token.staticcall(
            abi.encodeWithSignature("balanceOf(address)", address(this))
        );
        if (!success || data.length < 32) return 0;
        return abi.decode(data, (uint256));
    }

    function _safeTransfer(address token, address to, uint256 amount) internal {
        (bool success, ) = token.call(
            abi.encodeWithSignature("transfer(address,uint256)", to, amount)
        );
        require(success, "token transfer failed");
    }
}
