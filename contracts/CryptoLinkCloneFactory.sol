// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title CryptoLinkCloneFactory
 * @notice Creates EIP-1167 minimal proxy clones of MerchantCollectorV2.
 *         Deployed once per blockchain by CryptoLink admin.
 *         Each clone costs ~1/50th of a full contract deployment.
 *
 *         The factory has NO admin powers over deployed clones.
 *         Each clone's owner is set at creation and cannot be changed.
 */
contract CryptoLinkCloneFactory {
    address public immutable implementation;

    event CloneCreated(address indexed owner, address clone);

    constructor(address _implementation) {
        require(_implementation != address(0), "zero impl");
        implementation = _implementation;
    }

    /**
     * @notice Deploy a new MerchantCollector clone for a merchant.
     * @param merchantOwner The merchant's wallet address (becomes owner of the clone).
     * @return clone The address of the newly created clone contract.
     */
    function deploy(address merchantOwner) external returns (address clone) {
        require(merchantOwner != address(0), "zero owner");
        clone = _createClone(implementation);

        // Initialize the clone with the merchant's address as owner.
        // This can only be called once — the clone rejects subsequent initialize() calls.
        (bool ok, ) = clone.call(
            abi.encodeWithSignature("initialize(address)", merchantOwner)
        );
        require(ok, "init failed");

        emit CloneCreated(merchantOwner, clone);
    }

    /**
     * @dev Creates an EIP-1167 minimal proxy clone.
     *      Runtime code (45 bytes):
     *        363d3d373d3d3d363d73{impl}5af43d82803e903d91602b57fd5bf3
     */
    function _createClone(address impl) internal returns (address result) {
        bytes20 implBytes = bytes20(impl);
        assembly {
            let ptr := mload(0x40)
            mstore(ptr, 0x3d602d80600a3d3981f3363d3d373d3d3d363d73000000000000000000000000)
            mstore(add(ptr, 0x14), implBytes)
            mstore(add(ptr, 0x28), 0x5af43d82803e903d91602b57fd5bf30000000000000000000000000000000000)
            result := create(0, ptr, 0x37)
        }
        require(result != address(0), "clone failed");
    }
}
