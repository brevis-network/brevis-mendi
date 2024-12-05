import { Fixture } from 'ethereum-waffle';
import { ethers } from 'hardhat';
import { MendiDepth, MendiDepth__factory } from '../typechain';
import { loadFixture } from '@nomicfoundation/hardhat-toolbox/network-helpers';

import {  Wallet } from 'ethers';
import { expect } from 'chai';


describe('Mendi reward Test', async () => {
  let contract: MendiDepth;

  const zeroAddr = '0x0000000000000000000000000000000000000000';

  beforeEach(async () => {
    const res = await loadFixture(deployLib);
    contract = res.reward as MendiDepth;
  });

  async function deployLib() {
    const factory = new MendiDepth__factory();
    const [admin] = await ethers.getSigners();
    const reward = await factory.connect(admin).deploy(zeroAddr);

    return { reward };
  }

  it('should decode parameters', async () => {
    const res = await contract.mockDecode("0x000000000145893c88e6a0c2ddd26feeb64f039a2c41296fcb3f56400000000000000000000000000000000000000000000000000000368586533b01")
    console.log(res)
  })
});
