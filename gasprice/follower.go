package gasprice

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/0xPolygonHermez/zkevm-node/encoding"
	"github.com/0xPolygonHermez/zkevm-node/log"
)

// FollowerGasPrice struct.
type FollowerGasPrice struct {
	cfg  Config
	pool poolInterface
	ctx  context.Context
	eth  ethermanInterface
}

// newFollowerGasPriceSuggester inits l2 follower gas price suggester which is based on the l1 gas price.
func newFollowerGasPriceSuggester(ctx context.Context, cfg Config, pool poolInterface, ethMan ethermanInterface) *FollowerGasPrice {
	gps := &FollowerGasPrice{
		cfg:  cfg,
		pool: pool,
		ctx:  ctx,
		eth:  ethMan,
	}
	gps.UpdateGasPriceAvg()
	return gps
}

// UpdateGasPriceAvg updates the gas price.
func (f *FollowerGasPrice) UpdateGasPriceAvg() {
	ctx := context.Background()
	// Get L1 gasprice
	l1GasPrice := f.eth.GetL1GasPrice(f.ctx)
	btcEthRatio, _ := f.eth.GetTokenRatio()
	log.Debug("ChainLink btcEthRatio value: ", btcEthRatio)
	if btcEthRatio == 0 {
		btcEthRatio = f.cfg.DefaultRatio
	}
	if big.NewInt(0).Cmp(l1GasPrice) == 0 {
		log.Warn("gas price 0 received. Skipping update...")
		return
	}

	// Apply factor to calculate l2 gasPrice
	factor := big.NewFloat(0).SetFloat64(f.cfg.Factor)
	ratio := big.NewFloat(0).SetFloat64(btcEthRatio)
	res := new(big.Float).Mul(factor, big.NewFloat(0).SetInt(l1GasPrice))

	res = new(big.Float).Quo(res, ratio)

	// Store l2 gasPrice calculated
	result := new(big.Int)
	res.Int(result)
	minGasPrice := big.NewInt(0).SetUint64(f.cfg.DefaultGasPriceWei)
	if minGasPrice.Cmp(result) == 1 { // minGasPrice > result
		log.Warn("setting DefaultGasPriceWei for L2")
		result = minGasPrice
	}
	maxGasPrice := new(big.Int).SetUint64(f.cfg.MaxGasPriceWei)
	if f.cfg.MaxGasPriceWei > 0 && result.Cmp(maxGasPrice) == 1 { // result > maxGasPrice
		log.Warn("setting MaxGasPriceWei for L2")
		result = maxGasPrice
	}
	var truncateValue *big.Int
	log.Debug("Full L2 gas price value: ", result, ". Length: ", len(result.String()))
	numLength := len(result.String())
	if numLength > 3 { //nolint:gomnd
		aux := "%0" + strconv.Itoa(numLength-3) + "d" //nolint:gomnd
		var ok bool
		value := result.String()[:3] + fmt.Sprintf(aux, 0)
		truncateValue, ok = new(big.Int).SetString(value, encoding.Base10)
		if !ok {
			log.Error("error converting: ", truncateValue)
		}
	} else {
		truncateValue = result
	}

	log.Debug("Setting gas prices, l2 : ", truncateValue.Uint64(), " l1 :", l1GasPrice.Uint64())
	if truncateValue != nil {
		err := f.pool.SetGasPrices(ctx, truncateValue.Uint64(), l1GasPrice.Uint64())
		if err != nil {
			log.Errorf("failed to update gas price in poolDB, err: %v", err)
		}
	} else {
		log.Error("nil value detected. Skipping...")
	}
}
