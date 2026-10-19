// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import { MockFlareTeeManager } from "fcc-extension-scaffold/test/MockFlareTeeManager.sol";
import { IFlareContractRegistry } from "flare-periphery/src/coston2/IFlareContractRegistry.sol";
import { Test } from "forge-std/src/Test.sol";
import { Deploy } from "../script/Deploy.s.sol";
import { PostRegister } from "../script/PostRegister.s.sol";
import { InstructionSender } from "../src/InstructionSender.sol";

contract InstructionSenderTest is Test {
    MockFlareTeeManager internal manager;
    InstructionSender internal sender;
    uint256 internal fee;
    address internal caller = makeAddr("caller");

    function setUp() public {
        manager = new MockFlareTeeManager();
        sender = new InstructionSender(address(manager));
        manager.setInstructionsSender(address(sender));
        sender.setExtensionId(manager.EXTENSION_ID());
        fee = manager.FEE_PER_TEE();
        vm.deal(caller, 1 ether);
    }

    // The contract builds exactly the op pair and payload the Go side handles: the conformance fixture's.
    function test_MatchesTheFixture() public {
        string memory fixture = vm.readFile("../conformance/my-command.json");
        bytes memory payload = vm.parseJsonBytes(fixture, ".message");
        (address from, bytes memory message) = abi.decode(payload, (address, bytes));

        uint256 price = sender.quoteFee(sender.OP_TYPE_MY_EXTENSION(), sender.OP_COMMAND_MY_COMMAND());
        vm.deal(from, price);
        vm.prank(from);
        sender.myCommand{ value: price }(message);

        (bytes32 opType, bytes32 opCommand,,,) = manager.sent();
        assertEq(opType, opName(vm.parseJsonString(fixture, ".opType")));
        assertEq(opCommand, opName(vm.parseJsonString(fixture, ".opCommand")));
        assertEq(manager.sentMessage(), payload);
    }

    function test_MyCommandParams() public {
        vm.prank(caller);
        sender.myCommand{ value: fee }("hello");

        (bytes32 opType, bytes32 opCommand,, uint64 threshold, address claimBack) = manager.sent();
        assertEq(opType, sender.OP_TYPE_MY_EXTENSION());
        assertEq(opCommand, sender.OP_COMMAND_MY_COMMAND());
        assertEq(manager.sentTeeIds()[0], manager.TEE());
        assertEq(manager.sentCosigners().length, 0);
        assertEq(threshold, 0);
        assertEq(claimBack, caller);
        assertEq(manager.sentMessage(), abi.encode(caller, bytes("hello")));
    }

    // Both env variables live in this one test: tests run in parallel and share the environment.
    function test_DeployScript() public {
        Deploy d = new Deploy();
        string memory out = "deployments/test-deploy.json";
        vm.setEnv("DEPLOY_OUTPUT", out);

        // FlareTeeManager from the registry, whatever the shell exports
        vm.setEnv("FLARE_TEE_MANAGER", vm.toString(address(0)));
        vm.mockCall(
            address(d.REGISTRY()),
            abi.encodeCall(IFlareContractRegistry.getContractAddressByName, ("FlareTeeManager")),
            abi.encode(address(manager))
        );
        d.run();
        InstructionSender deployed = InstructionSender(vm.parseJsonAddress(vm.readFile(out), ".instructionSender"));
        assertEq(deployed.flareTeeManager(), address(manager));

        // flareTeeManager overrides it
        MockFlareTeeManager other = new MockFlareTeeManager();
        vm.setEnv("FLARE_TEE_MANAGER", vm.toString(address(other)));
        d.run();
        deployed = InstructionSender(vm.parseJsonAddress(vm.readFile(out), ".instructionSender"));
        assertEq(deployed.flareTeeManager(), address(other));
        vm.removeFile(out);
    }

    function test_PostRegisterScript() public {
        // the script broadcasts as the deployer, which owns the contract
        vm.prank(DEFAULT_SENDER);
        InstructionSender fresh = new InstructionSender(address(manager));
        manager.setInstructionsSender(address(fresh));
        vm.setEnv("INSTRUCTION_SENDER", vm.toString(address(fresh)));
        vm.setEnv("EXTENSION_ID", vm.toString(manager.EXTENSION_ID()));

        PostRegister p = new PostRegister();
        p.run();
        assertEq(fresh.extensionId(), manager.EXTENSION_ID());
        p.run(); // re-running with the same id changes nothing
    }
}

/// @dev An op name as Go and Solidity pad it: UTF-8, right-padded to 32 bytes.
function opName(string memory _name) pure returns (bytes32) {
    // casting is safe: conformance refuses fixtures with names over 32 bytes
    // forge-lint: disable-next-line(unsafe-typecast)
    return bytes32(bytes(_name));
}
