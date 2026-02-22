// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./MerchantCollector.sol";

/**
 * @title CollectorFactory
 * @notice Deploys MerchantCollector contracts using CREATE2 for deterministic addresses.
 *         Deploy one CollectorFactory per EVM chain. Store its address in CryptoLink config.
 *
 * Address prediction (pure, can be done off-chain):
 *   keccak256(0xff ++ factory ++ salt ++ keccak256(initcode))[12:]
 *
 * Salt convention: keccak256(abi.encodePacked(merchantOwnerAddress, merchantUUID))
 */
contract CollectorFactory {
    event Deployed(address indexed merchantOwner, bytes32 indexed salt, address collector);

    /**
     * @notice Deploy a MerchantCollector for a merchant.
     * @param merchantOwner The merchant's wallet address (becomes immutable owner of collector).
     * @param salt          Deterministic salt (use keccak256 of merchant UUID bytes).
     */
    function deploy(address merchantOwner, bytes32 salt) external returns (address collector) {
        bytes memory initcode = _initcode(merchantOwner);
        assembly {
            collector := create2(0, add(initcode, 32), mload(initcode), salt)
        }
        require(collector != address(0), "deploy failed");
        emit Deployed(merchantOwner, salt, collector);
    }

    /**
     * @notice Predict the address of a collector without deploying.
     *         This is a pure computation — no on-chain call needed.
     */
    function predictAddress(address merchantOwner, bytes32 salt) external view returns (address) {
        bytes32 hash = keccak256(
            abi.encodePacked(
                bytes1(0xff),
                address(this),
                salt,
                keccak256(_initcode(merchantOwner))
            )
        );
        return address(uint160(uint256(hash)));
    }

    function _initcode(address merchantOwner) internal pure returns (bytes memory) {
        return abi.encodePacked(
            type(MerchantCollector).creationCode,
            abi.encode(merchantOwner)
        );
    }
}
