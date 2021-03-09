const fs = require("fs")
const toml = require("toml")
const tomlify = require("tomlify-j0.4")
const Web3 = require("web3")
const HDWalletProvider = require("@truffle/hdwallet-provider")
const Kit = require("@celo/contractkit")

const hostChain = process.env.HOST_CHAIN || "ethereum"

// Returns a web3 instance basing on the host chain type.
const web3Provider = {
  ethereum: {
    getWeb3: () => {
      // We override transactionConfirmationBlocks and transactionBlockTimeout
      // because they're 25 and 50 blocks respectively at default. The result
      // of this on small private testnets is long wait times for scripts
      // to execute.
      const web3Options = {
        defaultBlock: "latest",
        defaultGas: 4712388,
        transactionBlockTimeout: 25,
        transactionConfirmationBlocks: 3,
        transactionPollingTimeout: 480,
      }

      const contractOwnerProvider = new HDWalletProvider(
          process.env.CONTRACT_OWNER_ETH_ACCOUNT_PRIVATE_KEY,
          ethRPCUrl
      )

      return new Web3(contractOwnerProvider, null, web3Options)
    }
  },

  celo: {
    getWeb3: () => {
      // In case the Celo chain is used, the web3 object could not use the
      // HD wallet provider as it conflicts with the Celo contract kit.
      // The web3 object should be initialized just with the RPC URL.
      return new Web3(ethRPCUrl)
    }
  }
}

// ETH host info
const ethRPCUrl = process.env.ETH_RPC_URL
const ethWSUrl = process.env.ETH_WS_URL
const ethNetworkId = process.env.ETH_NETWORK_ID

// Contract owner info
const contractOwnerAddress = process.env.CONTRACT_OWNER_ETH_ACCOUNT_ADDRESS
const authorizer = contractOwnerAddress
const purse = contractOwnerAddress

const operatorKeyFile = process.env.KEEP_TECDSA_ETH_KEYFILE_PATH

// LibP2P network info
const libp2pPeers = [process.env.KEEP_TECDSA_PEERS]
const libp2pPort = Number(process.env.KEEP_TECDSA_PORT)
const libp2pAnnouncedAddresses = [process.env.KEEP_TECDSA_ANNOUNCED_ADDRESSES]

const web3 = web3Provider[hostChain].getWeb3()

/*
Each <contract.json> file is sourced directly from the InitContainer.  Files are generated by
Truffle during contract migration and copied to the InitContainer image via Circle.
*/
const bondedECDSAKeepFactory = getWeb3Contract("BondedECDSAKeepFactory")
const keepBondingContract = getWeb3Contract("KeepBonding")
const tokenStakingContract = getWeb3Contract("TokenStaking")
const keepTokenContract = getWeb3Contract("KeepToken")
const tbtcSystemContract = getWeb3Contract("TBTCSystem")

// Addresses of the external contracts (e.g. TBTCSystem) which should be set for
// the InitContainer execution. Addresses should be separated with spaces.
const sanctionedApplications = process.env.SANCTIONED_APPLICATIONS.split(" ")

// Returns a web3 contract object based on a truffle contract artifact JSON file.
function getWeb3Contract(contractName) {
  const filePath = `/tmp/${contractName}.json`
  const parsed = JSON.parse(fs.readFileSync(filePath))
  const abi = parsed.abi
  const address = parsed.networks[ethNetworkId].address
  return new web3.eth.Contract(abi, address)
}

async function provisionKeepTecdsa() {
  try {
    console.log(`###########  Provisioning keep-ecdsa on ${hostChain} host chain! ###########`)

    console.log(
      `\n<<<<<<<<<<<< Read operator address from key file >>>>>>>>>>>>`
    )
    const operatorAddress = readAddressFromKeyFile(operatorKeyFile)

    console.log(
      `\n<<<<<<<<<<<< Funding Operator Account ${operatorAddress} >>>>>>>>>>>>`
    )
    await fundOperator(operatorAddress, purse, "10")

    console.log(
      `\n<<<<<<<<<<<< Staking Operator Account ${operatorAddress} >>>>>>>>>>>>`
    )
    await stakeOperator(operatorAddress, contractOwnerAddress, authorizer)

    console.log(
      `\n<<<<<<<<<<<< Authorizing Operator Contract ${bondedECDSAKeepFactory.options.address} >>>>>>>>>>>>`
    )
    await authorizeOperatorContract(
      operatorAddress,
      bondedECDSAKeepFactory.options.address,
      authorizer
    )

    console.log(
      `\n<<<<<<<<<<<< Deposit to KeepBondingContract ${keepBondingContract.options.address} >>>>>>>>>>>>`
    )
    await depositUnbondedValue(operatorAddress, purse, "10")

    for (let i = 0; i < sanctionedApplications.length; i++) {
      const sanctionedApplicationAddress = sanctionedApplications[i]

      console.log(
        `\n<<<<<<<<<<<< Check Sortition Pool for Sanctioned Application: ${sanctionedApplicationAddress} >>>>>>>>>>>>`
      )
      const sortitionPoolContractAddress = await getSortitionPool(
        sanctionedApplicationAddress
      )

      const ADDRESS_ZERO = "0x0000000000000000000000000000000000000000"
      if (
        !sortitionPoolContractAddress ||
        sortitionPoolContractAddress === ADDRESS_ZERO
      ) {
        console.error(
          `missing sortition pool for application: [${applicationAddress}]`
        )
        continue
      }

      console.log(
        `\n<<<<<<<<<<<< Authorizing Sortition Pool Contract ${sortitionPoolContractAddress} >>>>>>>>>>>>`
      )
      await authorizeSortitionPoolContract(
        operatorAddress,
        sortitionPoolContractAddress,
        authorizer
      )
    }

    console.log("\n<<<<<<<<<<<< Creating keep-ecdsa Config File >>>>>>>>>>>>")
    await createKeepTecdsaConfig()

    console.log("\n########### keep-ecdsa Provisioning Complete! ###########")
    process.exit()
  } catch (error) {
    console.error(error.message)
    throw error
  }
}

function readAddressFromKeyFile(keyFilePath) {
  const keyFile = JSON.parse(fs.readFileSync(keyFilePath, "utf8"))

  return web3.utils.toHex(keyFile.address)
}

async function fundOperator(operatorAddress, purse, requiredEtherBalance) {
  const requiredBalance = web3.utils.toBN(
    web3.utils.toWei(requiredEtherBalance, "ether")
  )

  const currentBalance = web3.utils.toBN(
    await web3.eth.getBalance(operatorAddress)
  )
  if (currentBalance.gte(requiredBalance)) {
    console.log(
      `Operator address is already funded, current balance: ${web3.utils.fromWei(
        currentBalance
      )}`
    )
    return
  }

  const transferAmount = requiredBalance.sub(currentBalance)

  console.log(
    `Funding account ${operatorAddress} with ${web3.utils.fromWei(
      transferAmount
    )} ether from purse ${purse}`
  )
  await transactor[hostChain].sendTransaction({
    from: purse,
    to: operatorAddress,
    value: transferAmount,
  })
  console.log(`Account ${operatorAddress} funded!`)
}

async function depositUnbondedValue(operatorAddress, purse, etherToDeposit) {
  const requiredBalance = web3.utils.toBN(
    web3.utils.toWei(etherToDeposit, "ether")
  )

  const currentBalance = web3.utils.toBN(
    await keepBondingContract.methods.unbondedValue(operatorAddress).call()
  )

  if (currentBalance.gte(requiredBalance)) {
    console.log(
      `Operator has required unbonded value, current balance: ${web3.utils.fromWei(
        currentBalance
      )}`
    )
    return
  }

  const transferAmount = requiredBalance.sub(currentBalance)

  const txObject = await keepBondingContract.methods.deposit(operatorAddress)

  await transactor[hostChain].sendTransactionObject(txObject, {
    value: transferAmount,
    from: purse,
  })

  console.log(
    `deposited ${web3.utils.fromWei(
      transferAmount
    )} ETH bonding value for operatorAddress ${operatorAddress}`
  )
}

async function isStaked(operatorAddress) {
  console.log("Checking if operator address is staked:")
  const stakedAmount = await tokenStakingContract.methods
    .balanceOf(operatorAddress)
    .call()
  return stakedAmount != 0
}

async function stakeOperator(
  operatorAddress,
  contractOwnerAddress,
  authorizer
) {
  const staked = await isStaked(operatorAddress)

  /*
  We need to stake only in cases where an operator account is not already staked.  If the account
  is staked, or the client type is relay-requester we need to exit staking, albeit for different
  reasons.  In the case where the account is already staked, additional staking will fail.
  Clients of type relay-requester don't need to be staked to submit a request, they're acting more
  as a consumer of the network, rather than an operator.
  */
  if (staked === true) {
    console.log("Operator account already staked, exiting!")
    return
  } else {
    console.log(
      `Staking 2000000 KEEP tokens on operator account ${operatorAddress}`
    )
  }

  const delegation = Buffer.concat([
    Buffer.from(web3.utils.hexToBytes(contractOwnerAddress)),
    Buffer.from(web3.utils.hexToBytes(operatorAddress)),
    Buffer.from(web3.utils.hexToBytes(authorizer)),
  ])

  const txObject = await keepTokenContract.methods
    .approveAndCall(
      tokenStakingContract.options.address,
      formatAmount(2000000, 18),
      delegation
    )

  await transactor[hostChain].sendTransactionObject(txObject, {
    from: contractOwnerAddress,
  })

  console.log(`Staked!`)
}

async function authorizeOperatorContract(
  operatorAddress,
  operatorContractAddress,
  authorizer
) {
  console.log(
    `Authorizing Operator Contract ${operatorContractAddress} for operator account ${operatorAddress}`
  )

  if (
    await tokenStakingContract.methods
      .isAuthorizedForOperator(operatorAddress, operatorContractAddress)
      .call()
  ) {
    console.log("Already authorized!")
    return
  }

  const txObject = await tokenStakingContract.methods
    .authorizeOperatorContract(operatorAddress, operatorContractAddress)

  await transactor[hostChain].sendTransactionObject(txObject, {
    from: authorizer
  })

  console.log(`Authorized!`)
}

async function authorizeSortitionPoolContract(
  operatorAddress,
  sortitionPoolContractAddress,
  authorizer
) {
  console.log(
    `Authorizing Sortition Pool Contract ${sortitionPoolContractAddress} for operator account ${operatorAddress}`
  )

  if (
    await keepBondingContract.methods
      .hasSecondaryAuthorization(operatorAddress, sortitionPoolContractAddress)
      .call()
  ) {
    console.log("Already authorized!")
    return
  }

  const txObject = await keepBondingContract.methods
    .authorizeSortitionPoolContract(
      operatorAddress,
      sortitionPoolContractAddress
    )

  await transactor[hostChain].sendTransactionObject(txObject, {
    from: authorizer
  })

  console.log(`Authorized!`)
}

async function getSortitionPool(applicationAddress) {
  const sortitionPoolContractAddress = await bondedECDSAKeepFactory.methods
    .getSortitionPool(applicationAddress)
    .call()

  console.log(
    `sortition pool contract address: ${sortitionPoolContractAddress}`
  )
  return sortitionPoolContractAddress
}

async function createKeepTecdsaConfig() {
  const parsedConfigFile = toml.parse(
    fs.readFileSync("/tmp/keep-ecdsa-config-template.toml", "utf8")
  )

  parsedConfigFile[hostChain].URL = ethWSUrl
  parsedConfigFile[hostChain].URLRPC = ethRPCUrl

  parsedConfigFile[hostChain].account.KeyFile = operatorKeyFile

  parsedConfigFile[hostChain].ContractAddresses.BondedECDSAKeepFactory =
      bondedECDSAKeepFactory.options.address

  parsedConfigFile[hostChain].ContractAddresses.TBTCSystem =
      tbtcSystemContract.options.address

  parsedConfigFile.LibP2P.Peers = libp2pPeers
  parsedConfigFile.LibP2P.Port = libp2pPort
  parsedConfigFile.LibP2P.AnnouncedAddresses = libp2pAnnouncedAddresses

  parsedConfigFile.Storage.DataDir = process.env.KEEP_DATA_DIR

  /*
  tomlify.toToml() writes our Seed/Port values as a float.  The added precision renders our config
  file unreadable by the keep-client as it interprets 3919.0 as a string when it expects an int.
  Here we format the default rendering to write the config file with Seed/Port values as needed.
  */
  const formattedConfigFile = tomlify.toToml(parsedConfigFile, {
    space: 2,
    replace: (key, value) => {
      return key == "Port" ? value.toFixed(0) : false
    },
  })

  fs.writeFileSync(
    "/mnt/keep-ecdsa/config/keep-ecdsa-config.toml",
    formattedConfigFile
  )
  console.log(
    "keep-ecdsa config written to /mnt/keep-ecdsa/config/keep-ecdsa-config.toml"
  )
}

/*
\heimdall aliens numbers.  Really though, the approveAndCall function expects numbers
in a particular format, this function facilitates that.
*/
function formatAmount(amount, decimals) {
  return web3.utils.toHex(
    web3.utils
      .toBN(amount)
      .mul(web3.utils.toBN(10).pow(web3.utils.toBN(decimals)))
  )
}

// Transactor sends transactions with respect of chain-specific requirements.
const transactor = {
  ethereum: {
    sendTransaction: async (txConfig) => {
      const tx = await web3.eth.sendTransaction(txConfig)
      console.log(`transaction ${tx.transactionHash} has been sent`)
    },

    sendTransactionObject: async (txObject, params) => {
      const tx = await txObject.send(params)
      console.log(`transaction ${tx.transactionHash} has been sent`)
    }
  },

  celo: {
    sendTransaction: async (txConfig) => {
      const tx = await getCeloKit().sendTransaction(txConfig)
      console.log(`transaction ${await tx.getHash()} has been sent`)
    },

    sendTransactionObject: async (txObject, params) => {
      const tx = await getCeloKit().sendTransactionObject(txObject, params)
      console.log(`transaction ${await tx.getHash()} has been sent`)
    }
  }
}

// Returns the Celo contract kit instance.
function getCeloKit() {
  const celoKit = Kit.newKitFromWeb3(web3)
  celoKit.addAccount(process.env.CONTRACT_OWNER_ETH_ACCOUNT_PRIVATE_KEY)
  return celoKit
}

provisionKeepTecdsa().catch((error) => {
  console.error(error)
  process.exit(1)
})
