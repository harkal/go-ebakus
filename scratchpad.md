### Interacting with the Faucet Contract

On the Ebakus testnet there is a Faucet contract at address ```0xd799e3f0f4b3d8429cb3c8c22b7866193ebbe1ca``` that you can use to obtain Ebakus tokens. In order to interact with the contract you will need the following ABI:

```
var abi = JSON.parse("[{\"constant\":true,\"inputs\":[],\"name\":\"getBalance\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"toWhom\",\"type\":\"address\"}],\"name\":\"sendWei\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"getWei\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getSendAmount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"fallback\"}]")
```

Once you have the ABI stored in the ```abi``` javascript variable, you can create an instance of the ```faucet``` object to interact with is using the command below: 

```var faucet = eth.contract(abi).at('0xd799e3f0f4b3d8429cb3c8c22b7866193ebbe1ca')```

Now you can have 1 EBAKUS token send to you for testing purposes using the following contract action:

```faucet.getWei()```

