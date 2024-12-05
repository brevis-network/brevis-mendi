package circuits

import (
	"github.com/brevis-network/brevis-sdk/sdk"
)

var (
	TargetMarketDepth = sdk.ConstUint248("0x1028C1D959B06200000000000") // 80016521857016597127997947904,  sqrt(1 + .02) * 2^96
)

var (
	FixPoint96 = sdk.ConstUint248("0x1000000000000000000000000")
	U248X32    = sdk.ConstUint248(1 << 32)
	U248X48    = sdk.ConstUint248(1 << 48)
	U248X128   = sdk.ConstUint248("0x100000000000000000000000000000000")
	U248X224   = sdk.ConstUint248("0x100000000000000000000000000000000000000000000000000000000")
	U248X244   = sdk.ConstUint248("0x10000000000000000000000000000000000000000000000000000000000000")
)

type MendiDepthCircuit struct {
	BlockNumber   sdk.Uint32
	PoolAddress   sdk.Uint248 // Holders' addresses
	SqrtPriceX96  sdk.Uint248
	CurrentTick   sdk.Uint248
	TickSpacing   sdk.Uint248
	Liquidity     sdk.Uint248
	UseToken0     sdk.Uint248
	NextTicks     []sdk.Uint248
	LiquidityNets []sdk.Uint248
	LiquidityABS  []sdk.Uint248
}

var _ sdk.AppCircuit = &MendiDepthCircuit{}

func (c *MendiDepthCircuit) Allocate() (maxReceipts, maxStorage, maxTransactions int) {
	// Our app is only ever going to use one storage data at a time so
	// we can simply limit the max number of data for storage to 1 and
	// 0 for all others
	return 0, 32, 0
}

func (c *MendiDepthCircuit) Define(api *sdk.CircuitAPI, in sdk.DataInput) error {
	api.AssertInputsAreUnique()

	storages := sdk.NewDataStream(api, in.StorageSlots)
	sdk.AssertEach(storages, func(s sdk.StorageSlot) sdk.Uint248 {
		return sdk.Uint248(api.Uint32.IsEqual(s.BlockNum, c.BlockNumber))
	})

	slot0 := in.StorageSlots.Raw[0]
	values := api.Bytes32.ToBinary(slot0.Value)
	priceX96Bits := api.Uint248.ToBinary(c.SqrtPriceX96, 160)
	for i, v := range priceX96Bits {
		api.Uint248.AssertIsEqual(v, values[i+96])
	}

	sqrtPriceX96TgtUp := c.getSqrtPriceX96Tgt(api, sdk.ConstUint248(1), c.SqrtPriceX96, TargetMarketDepth)
	sqrtPriceX96TgtDown := c.getSqrtPriceX96Tgt(api, sdk.ConstUint248(0), c.SqrtPriceX96, TargetMarketDepth)

	amount := sdk.ConstUint248(0)

	liquidityCurrent := c.Liquidity
	for i, nextTick := range c.NextTicks {
		sqrtPriceX96Next := c.getSqrtRatioAtTick(api, nextTick)
		api.Uint248.AssertIsLessOrEqual(sqrtPriceX96Next, sqrtPriceX96TgtUp)
		api.Uint248.AssertIsLessOrEqual(sqrtPriceX96TgtDown, sqrtPriceX96Next)
		amountToNextPrice := c.getAmountToNextPrice(api, c.UseToken0, c.SqrtPriceX96, sqrtPriceX96Next, liquidityCurrent)
		amount = api.Uint248.Add(amount, amountToNextPrice)
		liquidityCurrent = api.Uint248.Select(c.LiquidityABS[i], api.Uint248.Add(liquidityCurrent, c.LiquidityNets[i]), api.Uint248.Sub(liquidityCurrent, c.LiquidityNets[i]))
	}

	api.OutputUint(64, api.ToUint248(c.BlockNumber))
	api.OutputAddress(c.PoolAddress)
	api.OutputUint(248, amount)
	api.OutputBool(c.UseToken0)
	return nil
}

func (c *MendiDepthCircuit) getSqrtPriceX96Tgt(
	api *sdk.CircuitAPI,
	upper sdk.Uint248, // 0 or 1
	sqrtPriceX96 sdk.Uint248,
	sqrtDepthX96 sdk.Uint248,
) sdk.Uint248 {
	priceMulDepthDiv96, _ := api.Uint248.Div(api.Uint248.Mul(sqrtPriceX96, sqrtDepthX96), FixPoint96)
	api.Uint248.IsGreaterThan(sqrtPriceX96, priceMulDepthDiv96)

	priceMul96DivDepth, _ := api.Uint248.Div(api.Uint248.Mul(sqrtPriceX96, FixPoint96), sqrtDepthX96)
	return api.Uint248.Select(upper, priceMulDepthDiv96, priceMul96DivDepth)
}

func (c *MendiDepthCircuit) getAmountToNextPrice(
	api *sdk.CircuitAPI,
	useAmount0,
	sqrtPriceX96Current,
	sqrtPriceX96Next,
	liquidityCurrent sdk.Uint248,
) sdk.Uint248 {
	amount0Delta := c.getAmount0Delta(
		api,
		sqrtPriceX96Current,
		sqrtPriceX96Next,
		liquidityCurrent,
	)
	amount1Delta := c.getAmount1Delta(
		api,
		sqrtPriceX96Current,
		sqrtPriceX96Next,
		liquidityCurrent,
	)

	return api.Uint248.Select(
		api.Uint248.IsEqual(liquidityCurrent, sdk.ConstUint248(0)),
		sdk.ConstUint248(0),
		api.Uint248.Select(
			useAmount0,
			amount0Delta,
			amount1Delta,
		),
	)
}

func (c *MendiDepthCircuit) getAmount0Delta(
	api *sdk.CircuitAPI,
	sqrtRatioAX96, //
	sqrtRatioBX96, //
	liquidity sdk.Uint248,
) sdk.Uint248 {
	lower := api.Uint248.Select(api.Uint248.IsLessThan(sqrtRatioAX96, sqrtRatioBX96), sqrtRatioAX96, sqrtRatioBX96)
	upper := api.Uint248.Select(api.Uint248.IsLessThan(sqrtRatioBX96, sqrtRatioAX96), sqrtRatioAX96, sqrtRatioBX96)

	numerator1 := api.Uint248.Mul(liquidity, FixPoint96) // liquidity << 96
	numerator2 := api.Uint248.Sub(upper, lower)

	api.Uint248.AssertIsEqual(api.Uint248.IsGreaterThan(lower, sdk.ConstUint248(0)), sdk.ConstUint248(1))

	mulDiv12U := c.fullMathMulDiv(api, numerator1, numerator2, upper)
	nonRoundUpResult, _ := api.Uint248.Div(mulDiv12U, lower)

	return nonRoundUpResult
}

func (c *MendiDepthCircuit) getAmount1Delta(
	api *sdk.CircuitAPI,
	sqrtRatioAX96, //
	sqrtRatioBX96, //
	liquidity sdk.Uint248,
) sdk.Uint248 {
	lower := api.Uint248.Select(api.Uint248.IsLessThan(sqrtRatioAX96, sqrtRatioBX96), sqrtRatioAX96, sqrtRatioBX96)
	upper := api.Uint248.Select(api.Uint248.IsLessThan(sqrtRatioBX96, sqrtRatioAX96), sqrtRatioAX96, sqrtRatioBX96)

	return c.fullMathMulDiv(api, liquidity, api.Uint248.Sub(upper, lower), FixPoint96)
}

func (c *MendiDepthCircuit) fullMathMulDiv(api *sdk.CircuitAPI, a, b, denominator sdk.Uint248) sdk.Uint248 {
	result, _ := api.Uint248.Div(api.Uint248.Mul(a, b), denominator)
	return result
}

func (c *MendiDepthCircuit) fullMathMulDivRoundingUp(api *sdk.CircuitAPI, a, b, denominator sdk.Uint248) sdk.Uint248 {
	result, r := api.Uint248.Div(api.Uint248.Mul(a, b), denominator)
	result = api.Uint248.Select(api.Uint248.IsEqual(r, sdk.ConstUint248(0)), result, api.Uint248.Add(result, sdk.ConstUint248(1)))
	return result
}

func (c *MendiDepthCircuit) unsafeMathRoundingUp(api *sdk.CircuitAPI, a, b sdk.Uint248) sdk.Uint248 {
	result, r := api.Uint248.Div(a, b)
	result = api.Uint248.Select(api.Uint248.IsEqual(r, sdk.ConstUint248(0)), result, api.Uint248.Add(result, sdk.ConstUint248(1)))
	return result
}

func (c *MendiDepthCircuit) mulDiv128(api *sdk.CircuitAPI, a, b sdk.Uint248) sdk.Uint248 {
	q, _ := api.Uint248.Div(api.Uint248.Mul(a, U248X48), U248X128)
	q, _ = api.Uint248.Div(api.Uint248.Mul(q, b), U248X48)
	return q
}

func (c *MendiDepthCircuit) getSqrtRatioAtTick(
	api *sdk.CircuitAPI,
	tick sdk.Uint248,
) sdk.Uint248 {
	sign, absTick := c.getAbsTick(api, tick)
	tickBits := api.Uint248.ToBinary(absTick, 20)
	ratio := sdk.ConstUint248(0)

	for i, bit := range tickBits {
		if i == 0 {
			ratio = api.Uint248.Select(bit, sdk.ConstUint248("0xfffcb933bd6fad37aa2d162d1a594001"), sdk.ConstUint248("0x100000000000000000000000000000000"))
		} else if i == 1 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xfff97272373d413259a46990580e213a"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 2 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xfff2e50f5f656932ef12357cf3c7fdcc"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 3 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xffe5caca7e10e4e61c3624eaa0941cd0"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 4 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xffcb9843d60f6159c9db58835c926644"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 5 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xff973b41fa98c081472e6896dfb254c0"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 6 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xff2ea16466c96a3843ec78b326b52861"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 7 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xfe5dee046a99a2a811c461f1969c3053"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 8 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xfcbe86c7900a88aedcffc83b479aa3a4"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 9 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xf987a7253ac413176f2b074cf7815e54"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 10 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xf3392b0822b70005940c7a398e4b70f3"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 11 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xe7159475a2c29b7443b29c7fa6e889d9"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 12 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xd097f3bdfd2022b8845ad8f792aa5825"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 13 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0xa9f746462d870fdf8a65dc1f90e061e5"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 14 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x70d869a156d2a1b890bb3df62baf32f7"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 15 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x31be135f97d08fd981231505542fcfa6"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 16 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x9aa508b5b7a84e1c677de54f3e99bc9"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 17 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x5d6af8dedb81196699c329225ee604"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 18 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x2216e584f5fa1ea926041bedfe98"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		} else if i == 19 {
			tmpRatio := c.mulDiv128(api, ratio, sdk.ConstUint248("0x48a170391f7dc42444e8fa2"))
			ratio = api.Uint248.Select(bit, tmpRatio, ratio)
		}
	}

	// If sign is positive, ratio = uint256.max / ratio
	// In circuit, only U248 is support. Simulate uint256.max as 2**244 * 2**12
	q, _ := api.Uint248.Div(U248X244, ratio)
	maxDivRatio := api.Uint248.Mul(q, sdk.ConstUint248(1<<12))
	ratio = api.Uint248.Select(sign, maxDivRatio, ratio)

	q, r := api.Uint248.Div(ratio, U248X32)
	q = api.Uint248.Select(api.Uint248.IsEqual(r, sdk.ConstUint248(0)), q, api.Uint248.Add(q, sdk.ConstUint248(1)))
	return q
}

func (c *MendiDepthCircuit) getAbsTick(api *sdk.CircuitAPI, tick sdk.Uint248) (sign, abs sdk.Uint248) {
	return sdk.ConstUint248(1), tick
}
