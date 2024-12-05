// SPDX-License-Identifier: MIT
pragma solidity ^0.8.18;

import "@openzeppelin/contracts/access/Ownable.sol";

import "./lib/BrevisAppZkOnly.sol";

// Only accept ZK-attested results.
contract MendiDepth is BrevisAppZkOnly, Ownable {
    event DepthAttested(uint64 blockNum, address pool, uint248 amount, bool useAmount0);

    bytes32 public vkHash;

    constructor(address _brevisRequest) BrevisAppZkOnly(_brevisRequest) Ownable(msg.sender) {}

    // BrevisQuery contract will call our callback once Brevis backend submits the proof.
    // This method is called with once the proof is verified.
    function handleProofResult(bytes32 _vkHash, bytes calldata _circuitOutput) internal override {
        // We need to check if the verifying key that Brevis used to verify the proof
        // generated by our circuit is indeed our designated verifying key. This proves
        // that the _circuitOutput is authentic
        require(vkHash == _vkHash, "invalid vk");
        (uint64 blockNum, address pool, uint248 amount, bool useAmount0) = decodeOutput(_circuitOutput);
        emit DepthAttested(blockNum, pool, amount, useAmount0);
    }

    function decodeOutput(bytes calldata o) internal pure returns (uint64 blockNum, address pool, uint248 amount, bool useAmount0) {
        blockNum = uint64(bytes8(o[0:8]));
        pool  = address(bytes20(o[8:28]));
        amount  = uint248(bytes31(o[28:59]));
        useAmount0 = uint8(bytes1(o[59:60])) == 1;
    }

    function setVkHash(bytes32 _vkHash) external onlyOwner {
        vkHash = _vkHash;
    }

    function mockDecode(bytes calldata o) external pure returns (uint64 blockNum, address pool, uint248 amount, bool useAmount0) {
       return decodeOutput(o);
    }
}
