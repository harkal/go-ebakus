# Deploying your Ebakus Contracts on Ebakus

## 1. Compile your contract

```
solcjs --abi game.sol
solcjs --bin game.sol
```

## 2. Display the compiled code on the console

```
more game_sol_game.abi
more game_sol_game.bin
```

## 3. We continue on our ebakus node

```$ ebakus --testnet --verbosity=2 console```

You should see this:

```
Welcome to the Ebakus JavaScript console!

instance: Ebakus/v1.8.20-unstable/darwin-amd64/go1.11.2
coinbase: 0x32f14386cea573bba82282b6f449ee77030a96e2
at block: 89471 (Mon, 24 Dec 2018 00:00:38 EET)
 datadir: /Users/harkal/ebakus2
 modules: admin:1.0 debug:1.0 dpos:1.0 eth:1.0 miner:1.0 net:1.0 personal:1.0 rpc:1.0 txpool:1.0 web3:1.0

>
```

## 4. Unlock you account

```
> personal.unlockAccount(eth.coinbase)
Unlock account 0x....
Passphrase: [ENTER PASSPHRASE]
```

## 5. Setup abi and bytecode

```
> var myContract = eth.contract([CONTENTS OF ABI FILE])
> var bytecode = '0x[CONTENTS OF BIN FILE]'
```

Replace [CONTENTS OF ABI FILE] and [CONTENTS OF BIN FILE] with the outputs of the more commands that should still be open in another terminal window.

## 6. Deploy

```
> var game = myContract.new({from:eth.coinbase, data:bytecode, gas: 2000000})
```

## 7. Interact with contract

```
> game.playMove('forward')
```