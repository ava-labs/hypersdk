// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.27;

contract ContractFactory {
    event ContractDeployed(address indexed newContract);
    
    function deployContract() public returns (address) {
        TestContract newContract = new TestContract();
        emit ContractDeployed(address(newContract));
        return address(newContract);
    }
}

contract TestContract {
    uint256 public value;
    address public deployedContract;
    
    event ValueChanged(uint256 newValue);
    event TransferReceived(address indexed from, uint256 amount);
    
    function setValue(uint256 _value) public {
        value = _value;
        emit ValueChanged(_value);
    }
    
    function getValue() public view returns (uint256) {
        return value;
    }
    
    function transferToAddress(address payable _to) public payable {
        _to.transfer(msg.value);
    }
    

    function transferThroughContract(address payable _finalDestination) public payable {
        emit TransferReceived(msg.sender, msg.value);
        _finalDestination.transfer(msg.value);
    }
    
    function delegateCallTest(address _contract, bytes memory _data) public returns (bytes memory) {
        (bool success, bytes memory result) = _contract.delegatecall(_data);
        require(success, "Delegate call failed");
        return result;
    }
    
    receive() external payable {
        emit TransferReceived(msg.sender, msg.value);
    }
}