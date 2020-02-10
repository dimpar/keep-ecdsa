import {
  getETHBalancesFromList,
  getERC20BalancesFromList,
  addToBalances
} from './helpers/listBalanceUtils'

import { mineBlocks } from "./helpers/mineBlocks";

const { expectRevert } = require('openzeppelin-test-helpers');

const ECDSAKeep = artifacts.require('./ECDSAKeep.sol')
const TestToken = artifacts.require('./TestToken.sol')
const KeepBonding = artifacts.require('./KeepBonding.sol')
const TestEtherReceiver = artifacts.require('./TestEtherReceiver.sol')

const truffleAssert = require('truffle-assertions')

const BN = web3.utils.BN

const chai = require('chai')
chai.use(require('bn-chai')(BN))
const expect = chai.expect

contract('ECDSAKeep', (accounts) => {
  const owner = accounts[1]
  const members = [accounts[2], accounts[3]]
  const honestThreshold = 1

  let keepBonding;

  before(async () => {
    keepBonding = await KeepBonding.new()
  })

  describe('#constructor', async () => {
    it('succeeds', async () => {
      let keep = await ECDSAKeep.new(
        owner,
        members,
        honestThreshold,
        keepBonding.address
      )

      assert(web3.utils.isAddress(keep.address), 'invalid keep address')
    })
  })

  describe('#sign', async () => {
    const digest = '0xca071ca92644f1f2c4ae1bf71b6032e5eff4f78f3aa632b27cbc5f84104a32da'
    let keep

    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
    })

    it('emits event', async () => {
      const digest = '0xbb0b57005f01018b19c278c55273a60118ffdd3e5790ccc8a48cad03907fa521'

      let res = await keep.sign(digest, { from: owner })
      truffleAssert.eventEmitted(res, 'SignatureRequested', (ev) => {
        return ev.digest == digest
      })
    })

    it('cannot be called by non-owner', async () => {
      try {
        await keep.sign(digest)
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, 'Ownable: caller is not the owner.')
      }
    })

    it('cannot be called by non-owner member', async () => {
      try {
        await keep.sign(digest, { from: members[0] })
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, 'Ownable: caller is not the owner.')
      }
    })

    it('cannot be requested if already in progress', async () => {
      await keep.sign(digest, { from: owner })
      try {
        await keep.sign('0x02', { from: owner })
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, 'Signer is busy')
      }
    })

    it('can be requested again after timeout passed', async () => {
      await keep.sign(digest, { from: owner })

      const signingTimeout = await keep.signingTimeout.call()
      mineBlocks(signingTimeout)

      await keep.sign(digest, { from: owner })
    })
  })

  describe('public key', () => {
    const expectedPublicKey = '0xa899b9539de2a6345dc2ebd14010fe6bcd5d38db9ed75cef4afc6fc68a4c45a4901970bbff307e69048b4d6edf960a6dd7bc5ba9b1cf1b4e0a1e319f68e0741a'

    let keep

    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address);
    })

    it('get public key before it is set', async () => {
      let publicKey = await keep.getPublicKey.call()

      assert.equal(publicKey, undefined, 'public key should not be set')
    })

    it('set public key and get it', async () => {
      await keep.setPublicKey(expectedPublicKey, { from: members[0] })

      let publicKey = await keep.getPublicKey.call()

      assert.equal(
        publicKey,
        expectedPublicKey,
        'incorrect public key'
      )
    })

    describe('setPublicKey', async () => {
      it('emits an event', async () => {
        let res = await keep.setPublicKey(expectedPublicKey, { from: members[0] })

        truffleAssert.eventEmitted(res, 'PublicKeyPublished', (ev) => {
          return ev.publicKey == expectedPublicKey
        })
      })

      it('cannot be called by non-member', async () => {
        try {
          await keep.setPublicKey(expectedPublicKey)
          assert(false, 'Test call did not error as expected')
        } catch (e) {
          assert.include(e.message, 'Caller is not the keep member')
        }
      })

      it('cannot be called by non-member owner', async () => {
        try {
          await keep.setPublicKey(expectedPublicKey, { from: owner })
          assert(false, 'Test call did not error as expected')
        } catch (e) {
          assert.include(e.message, 'Caller is not the keep member')
        }
      })
    })
  })

  describe('checkBondAmount', () =>  {
    const value0 = new BN(30)
    const value1 = new BN(70)

    it('should return bond amount', async () => {
      let keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
      let referenceID = web3.utils.toBN(web3.utils.padLeft(keep.address, 32))
      
      await keepBonding.deposit(members[0], { value: value0 })
      await keepBonding.deposit(members[1], { value: value1 })
      await keepBonding.createBond(members[0], keep.address, referenceID, value0)
      await keepBonding.createBond(members[1], keep.address, referenceID, value1)

      let actual = await keep.checkBondAmount.call()
      let expected = value0.add(value1);

      expect(actual).to.eq.BN(expected, "incorrect bond amount.");
    })  
  })

  describe('seizeSignerBonds', () =>  {
    const value0 = new BN(30)
    const value1 = new BN(70)

    it('should seize signer bond', async () => {
      let keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
      let referenceID = web3.utils.toBN(web3.utils.padLeft(keep.address, 32))
      
      await keepBonding.deposit(members[0], { value: value0 })
      await keepBonding.deposit(members[1], { value: value1 })
      await keepBonding.createBond(members[0], keep.address, referenceID, value0)
      await keepBonding.createBond(members[1], keep.address, referenceID, value1)

      let bondsBeforeSeizure = await keep.checkBondAmount()
      let expected = value0.add(value1);
      expect(bondsBeforeSeizure).to.eq.BN(expected, "incorrect bond amount before seizure.");
      
      let gasPrice = await web3.eth.getGasPrice()

      let ownerBalanceBefore = await web3.eth.getBalance(owner);
      let txHash = await keep.seizeSignerBonds({from: owner})

      let seizedSignerBondsFee = new BN(txHash.receipt.gasUsed).mul(new BN(gasPrice))
      let ownerBalanceDiff = new BN(await web3.eth.getBalance(owner))
          .add(seizedSignerBondsFee).sub(new BN(ownerBalanceBefore));

      expect(ownerBalanceDiff).to.eq.BN(value0.add(value1), "incorrect owner balance.");
      
      let bondsAfterSeizure = await keep.checkBondAmount()
      expect(bondsAfterSeizure).to.eq.BN(0, "should zero all the bonds.");
    })  
  })

  describe('submitSignatureFraud', () =>  {
    const digest1 = '0x14a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b'
    const digest2 = '0x54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b'
    const digest3 = '0x24a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b'
    const signatureV = 27
    const signatureR = '0x9b32c3623b6a16e87b4d3a56cd67c666c9897751e24a51518136185403b1cba2'
    const signatureS = '0x90838891021e1c7d0d1336613f24ecab703dee5ff1b6c8881bccc2c011606a35'
    const publicKey = '0x657282135ed640b0f5a280874c7e7ade110b5c3db362e0552e6b7fff2cc8459328850039b734db7629c31567d7fc5677536b7fc504e967dc11f3f2289d3d4051'
    let keep
    
    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
      const signingTimeout = await keep.signingTimeout.call()
      
      await keep.setPublicKey(publicKey, { from: members[0] })
      
      await keep.sign(digest1, { from: owner })
      mineBlocks(signingTimeout)
      await keep.sign(digest3, { from: owner })
    })
    
    it('should return true when signature is valid but was not requested', async () => {
      let res = await keep.submitSignatureFraud.call(
        signatureV,
        signatureR,
        signatureS,
        digest2, 
        '0x000'
      )
        
      assert.isTrue(res, 'Signature is fraudulent because is valid but was not requested.')
    })

    it('should return an error when signature is invalid and was requested', async () => {
      const badSignatureR = '0x1112c3623b6a16e87b4d3a56cd67c666c9897751e24a51518136185403b1cba2'
      await expectRevert(
        keep.submitSignatureFraud.call(
          signatureV,
          badSignatureR,
          signatureS,
          digest1, 
          '0x000'
        ),
        'Signature is not fraudulent'
      )
    })
        
    it('should return an error when signature is invalid and was not requested', async () => {
      const badDigest = '0x11a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b'
      const badSignatureR = '0x1112c3623b6a16e87b4d3a56cd67c666c9897751e24a51518136185403b1cba2'
      await expectRevert(
        keep.submitSignatureFraud.call(
          signatureV,
          badSignatureR,
          signatureS,
          badDigest, 
          '0x000'
        ),
        'Signature is not fraudulent'
      )
    })

    it('should return an error when signature is valid and was requested', async () => {
      await expectRevert(
        keep.submitSignatureFraud.call(
          signatureV,
          signatureR,
          signatureS,
          digest1, 
          '0x000'
        ),
        'Signature is not fraudulent'
      )
    })
  })

  describe('submitSignature', () => {
    const digest = '0x54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b'
    const publicKey = '0x657282135ed640b0f5a280874c7e7ade110b5c3db362e0552e6b7fff2cc8459328850039b734db7629c31567d7fc5677536b7fc504e967dc11f3f2289d3d4051'
    const signatureR = '0x9b32c3623b6a16e87b4d3a56cd67c666c9897751e24a51518136185403b1cba2'
    const signatureS = '0x90838891021e1c7d0d1336613f24ecab703dee5ff1b6c8881bccc2c011606a35'
    const signatureRecoveryID = 0

    let keep

    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)

      await keep.setPublicKey(publicKey, { from: members[0] })
      await keep.sign(digest, { from: owner })
    })

    it('emits an event', async () => {
      let res = await keep.submitSignature(
        signatureR,
        signatureS,
        signatureRecoveryID,
        { from: members[0] }
      )

      truffleAssert.eventEmitted(res, 'SignatureSubmitted', (ev) => {
        return ev.digest == digest
          && ev.r == signatureR
          && ev.s == signatureS
          && ev.recoveryID == signatureRecoveryID
      })
    })

    it('clears signing lock after submission', async () => {
      await keep.submitSignature(
        signatureR,
        signatureS,
        signatureRecoveryID,
        { from: members[0] }
      )

      await keep.sign(digest, { from: owner })
    })

    it('cannot be submitted if signing was not requested', async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)

      await keep.setPublicKey(publicKey, { from: members[0] })

      try {
        await keep.submitSignature(
          signatureR,
          signatureS,
          signatureRecoveryID,
          { from: members[0] }
        )
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, "Not awaiting a signature")
      }
    })

    it('accepts signature after timeout', async () => {
      const signingTimeout = await keep.signingTimeout.call()
      mineBlocks(signingTimeout)

      await keep.submitSignature(
        signatureR,
        signatureS,
        signatureRecoveryID,
        { from: members[0] }
      )
    })

    describe('validates signature', async () => {
      it('rejects recovery ID out of allowed range', async () => {
        try {
          await keep.submitSignature(
            signatureR,
            signatureS,
            4,
            { from: members[0] }
          )
          assert(false, 'Test call did not error as expected')
        } catch (e) {
          assert.include(e.message, "Recovery ID must be one of {0, 1, 2, 3}")
        }
      })

      it('rejects invalid signature', async () => {
        try {
          await keep.submitSignature(
            signatureR,
            signatureS,
            1,
            { from: members[0] }
          )
          assert(false, 'Test call did not error as expected')
        } catch (e) {
          assert.include(e.message, "Invalid signature")
        }
      })
    })

    it('cannot be called by non-member', async () => {
      try {
        await keep.submitSignature(
          signatureR,
          signatureS,
          signatureRecoveryID
        )
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, 'Caller is not the keep member')
      }
    })

    it('cannot be called by non-member owner', async () => {
      try {
        await keep.submitSignature(
          signatureR,
          signatureS,
          signatureRecoveryID,
          { from: owner }
        )
        assert(false, 'Test call did not error as expected')
      } catch (e) {
        assert.include(e.message, 'Caller is not the keep member')
      }
    })
  })

  describe('#distributeETHToMembers', async () => {
    const ethValue = 100000
    let keep
    let etherReceiver

    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
      etherReceiver = await TestEtherReceiver.new()
    })

    it('correctly distributes ETH', async () => {
      const initialBalances = await getETHBalancesFromList(members)

      await keep.distributeETHToMembers({ value: ethValue })

      const newBalances = await getETHBalancesFromList(members)
      const check = addToBalances(initialBalances, ethValue / members.length)

      assert.equal(newBalances.toString(), check.toString())
    })

    it('correctly handles unused remainder', async () => {
      const expectedRemainder = 1
      const valueWithRemainder = members.length + expectedRemainder
      const initialKeepBalance = await web3.eth.getBalance(keep.address)

      await keep.distributeETHToMembers({ value: valueWithRemainder })

      const finalKeepBalance = await web3.eth.getBalance(keep.address)
      const keepBalanceCheck = finalKeepBalance - initialKeepBalance

      assert.equal(keepBalanceCheck, new BN(expectedRemainder))
    })

    it('reverts with zero value', async () => {
      await expectRevert(
        keep.distributeETHToMembers(),
        'dividend value must be non-zero'
      )
    })

    it('reverts with zero dividend', async () => {
      const msgValue = members.length - 1
      await expectRevert(
        keep.distributeETHToMembers({ value: msgValue }),
        'dividend value must be non-zero'
      )
    })

    it('does not revert in case of transfer failure', async () => {
      const member1 = accounts[2]
      const member2 = etherReceiver.address // a receiver which we expect to reject the transfer
      const member3 = accounts[3]

      const members = [member1, member2, member3]

      const singleValue = new BN(await etherReceiver.invalidValue.call())
      const msgValue = singleValue.mul(new BN(members.length))

      const expectedBalances = [
        new BN(await web3.eth.getBalance(member1)).add(singleValue),
        new BN(await web3.eth.getBalance(member2)),
        new BN(await web3.eth.getBalance(member3)).add(singleValue),
      ]

      const keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)

      await keep.distributeETHToMembers({ value: msgValue })

      // Check balances of all keep members' accounts.
      const newBalances = await getETHBalancesFromList(members)
      assert.deepEqual(newBalances, expectedBalances)

      // Check that value which failed transfer remained in the keep contract.
      assert.equal(await web3.eth.getBalance(keep.address), new BN(singleValue))

    })
  })

  describe('#distributeERC20ToMembers', async () => {
    const erc20Value = 1000000
    let keep
    let token

    beforeEach(async () => {
      keep = await ECDSAKeep.new(owner, members, honestThreshold, keepBonding.address)
      token = await TestToken.new()
    })

    it('correctly distributes ERC20', async () => {
      const initialBalances = await getERC20BalancesFromList(members, token)
      await token.mint(accounts[0], erc20Value)
      await token.approve(keep.address, erc20Value)
      await keep.distributeERC20ToMembers(token.address, erc20Value)

      const newBalances = await getERC20BalancesFromList(members, token)
      const check = addToBalances(initialBalances, erc20Value / members.length)

      assert.equal(newBalances.toString(), check.toString())
    })

    it('correctly handles unused remainder', async () => {
      const valueWithRemainder = members.length + 1
      const initialKeepBalance = await token.balanceOf(keep.address)

      await token.mint(accounts[0], valueWithRemainder)
      await token.approve(keep.address, valueWithRemainder)
      await keep.distributeERC20ToMembers(token.address, valueWithRemainder)

      const finalKeepBalance = await token.balanceOf(keep.address)
      const expectedRemainder = 0
      const keepBalanceCheck = initialKeepBalance - finalKeepBalance

      assert.equal(keepBalanceCheck, expectedRemainder)
    })

    it('fails with insufficient approval', async () => {
      await expectRevert(
        keep.distributeERC20ToMembers(token.address, erc20Value),
        "SafeMath: subtraction overflow"
      )
    })

    it('fails with zero value', async () => {
      await token.mint(accounts[0], erc20Value)
      await expectRevert(
        keep.distributeERC20ToMembers(token.address, 0),
        "dividend value must be non-zero"
      )
    })

    it('reverts with zero dividend', async () => {
      const value = members.length - 1
      await token.mint(accounts[0], value)
      await token.approve(keep.address, erc20Value)
      await expectRevert(
        keep.distributeERC20ToMembers(token.address, value),
        'dividend value must be non-zero'
      )
    })
  })
})
