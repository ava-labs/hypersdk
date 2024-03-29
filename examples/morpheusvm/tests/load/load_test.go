// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pebble"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/workers"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	trpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
)

const genesisBalance uint64 = hconsts.MaxUint64

var (
	logFactory logging.Factory
	log        logging.Logger
)

func init() {
	logFactory = logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})
	l, err := logFactory.Make("main")
	if err != nil {
		panic(err)
	}
	log = l
}

type instance struct {
	chainID            ids.ID
	nodeID             ids.NodeID
	vm                 *vm.VM
	toEngine           chan common.Message
	JSONRPCServer      *httptest.Server
	TokenJSONRPCServer *httptest.Server
	cli                *rpc.JSONRPCClient // clients for embedded VMs
	tcli               *trpc.JSONRPCClient
	dbDir              string
	parse              []float64
	verify             []float64
	accept             []float64
}

type account struct {
	priv    ed25519.PrivateKey
	factory *auth.ED25519Factory
	rsender codec.Address
	sender  string
}

var (
	dist        string
	vms         int
	accts       int
	txs         int
	trace       bool
	maxFee      uint64
	acceptDepth int
	verifyAuth  bool

	deterministicKeys bool

	senders []*account
	blks    []*chain.StatelessBlock

	// root account used to facilitate all other transfers
	root *account

	// when used with embedded VMs
	genesisBytes []byte
	instances    []*instance
	numWorkers   int

	gen *genesis.Genesis

	z *rand.Zipf // only populated if zipf dist

	txGen    time.Duration
	blockGen time.Duration
)

func init() {
	flag.StringVar(
		&dist,
		"dist",
		"uniform",
		"account usage distribution",
	)
	flag.IntVar(
		&vms,
		"vms",
		5,
		"number of VMs to create",
	)
	flag.IntVar(
		&accts,
		"accts",
		1000,
		"number of accounts to create",
	)
	flag.IntVar(
		&txs,
		"txs",
		1000,
		"number of txs to create",
	)
	flag.BoolVar(
		&trace,
		"trace",
		false,
		"trace function calls",
	)
	flag.Uint64Var(
		&maxFee,
		"max-fee",
		1000,
		"max fee per tx",
	)
	flag.IntVar(
		&acceptDepth,
		"accept-depth",
		1,
		"depth to run block accept",
	)
	flag.BoolVar(
		&verifyAuth,
		"verify-auth",
		true,
		"verify auth over RPC and in block verification",
	)
	flag.BoolVar(
		&deterministicKeys,
		"deterministic-keys",
		false,
		"generate deterministic keys",
	)
}

func TestLoad(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "indexvm load test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	gomega.Ω(dist).Should(gomega.BeElementOf([]string{"uniform", "zipf"}))
	gomega.Ω(vms).Should(gomega.BeNumerically(">", 1))

	var err error
	priv, err := ed25519.GeneratePrivateKey()
	gomega.Ω(err).Should(gomega.BeNil())
	rsender := auth.NewED25519Address(priv.PublicKey())
	sender := codec.MustAddressBech32(consts.HRP, rsender)
	root = &account{priv, auth.NewED25519Factory(priv), rsender, sender}
	log.Debug(
		"generated root key",
		zap.String("addr", sender),
		zap.String("pk", hex.EncodeToString(priv[:])),
	)

	// create embedded VMs
	instances = make([]*instance, vms)
	gen = genesis.Default()
	gen.MinUnitPrice = fees.Dimensions{1, 1, 1, 1, 1}
	// target must be set less than max, otherwise we will iterate through all txs in mempool
	gen.WindowTargetUnits = fees.Dimensions{hconsts.NetworkSizeLimit - 10*units.KiB, hconsts.MaxUint64, hconsts.MaxUint64, hconsts.MaxUint64, hconsts.MaxUint64} // disable unit price increase
	// leave room for block header
	gen.MaxBlockUnits = fees.Dimensions{hconsts.NetworkSizeLimit - units.KiB, hconsts.MaxUint64, hconsts.MaxUint64, hconsts.MaxUint64, hconsts.MaxUint64}
	gen.MinBlockGap = 0                                        // don't require time between blocks
	gen.ValidityWindow = 1_000 * hconsts.MillisecondsPerSecond // txs shouldn't expire
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: sender,
			Balance: genesisBalance,
		},
	}
	genesisBytes, err = json.Marshal(gen)
	gomega.Ω(err).Should(gomega.BeNil())

	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	app := &appSender{}
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})
	// TODO: add main logger we can view data from later
	for i := range instances {
		nodeID := ids.GenerateTestNodeID()
		sk, err := bls.NewSecretKey()
		gomega.Ω(err).Should(gomega.BeNil())
		l, err := logFactory.Make(nodeID.String())
		gomega.Ω(err).Should(gomega.BeNil())
		dname, err := os.MkdirTemp("", fmt.Sprintf("%s-chainData", nodeID.String()))
		gomega.Ω(err).Should(gomega.BeNil())
		snowCtx := &snow.Context{
			NetworkID:    networkID,
			SubnetID:     subnetID,
			ChainID:      chainID,
			NodeID:       nodeID,
			Log:          l,
			ChainDataDir: dname,
			Metrics:      metrics.NewOptionalGatherer(),
			PublicKey:    bls.PublicFromSecretKey(sk),
		}

		dname, err = os.MkdirTemp("", fmt.Sprintf("%s-root", nodeID.String()))
		gomega.Ω(err).Should(gomega.BeNil())
		db, _, err := pebble.New(dname, pebble.NewDefaultConfig())
		gomega.Ω(err).Should(gomega.BeNil())
		numWorkers = runtime.NumCPU() // only run one at a time

		c := controller.New()
		toEngine := make(chan common.Message, 1)
		var tracePrefix string
		if trace {
			switch i {
			case 0:
				tracePrefix = `"traceEnabled":true, "traceSampleRate":1, "traceAgent":"builder", `
			case 1:
				tracePrefix = `"traceEnabled":true, "traceSampleRate":1, "traceAgent":"verifier", `
			}
		}
		mempoolSize := max(txs, accts*2) // TODO: handle this with a const eg, num EVM txs
		err = c.Initialize(
			context.TODO(),
			snowCtx,
			db,
			genesisBytes,
			nil,
			[]byte(
				fmt.Sprintf(
					`{%s"authVerificationCores":%d, "rootGenerationCores":%d, "transactionExecutionCores":%d, "mempoolSize":%d, "mempoolSponsorSize":%d, "verifyAuth":%t, "testMode":true}`,
					tracePrefix,
					numWorkers/3,
					numWorkers/3,
					numWorkers/3,
					mempoolSize,
					mempoolSize,
					verifyAuth,
				),
			),
			toEngine,
			nil,
			app,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		var hd map[string]http.Handler
		hd, err = c.CreateHandlers(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		jsonRPCServer := httptest.NewServer(hd[rpc.JSONRPCEndpoint])
		tjsonRPCServer := httptest.NewServer(hd[trpc.JSONRPCEndpoint])
		instances[i] = &instance{
			chainID:            snowCtx.ChainID,
			nodeID:             snowCtx.NodeID,
			vm:                 c,
			toEngine:           toEngine,
			JSONRPCServer:      jsonRPCServer,
			TokenJSONRPCServer: tjsonRPCServer,
			cli:                rpc.NewJSONRPCClient(jsonRPCServer.URL),
			tcli:               trpc.NewJSONRPCClient(tjsonRPCServer.URL, snowCtx.NetworkID, snowCtx.ChainID),
			dbDir:              dname,
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		c.ForceReady()
	}

	// Verify genesis allocates loaded correctly (do here otherwise test may
	// check during and it will be inaccurate)
	for _, inst := range instances {
		cli := inst.tcli
		g, err := cli.Genesis(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())

		for _, alloc := range g.CustomAllocation {
			bal, err := cli.Balance(context.Background(), alloc.Address)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(bal).Should(gomega.Equal(alloc.Balance))
		}
	}

	app.instances = instances
	color.Blue("created %d VMs", vms)
})

var _ = ginkgo.AfterSuite(func() {
	for _, instance := range instances {
		instance.JSONRPCServer.Close()
		instance.TokenJSONRPCServer.Close()
		err := instance.vm.Shutdown(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
	}

	// Print out stats
	log.Info("-----------")
	log.Info("stats:")
	blocks := len(blks)
	log.Info("workers", zap.Int("count", numWorkers))
	log.Info(
		"tx generation",
		zap.Int("accts", accts),
		zap.Int("txs", txs),
		zap.Duration("t", txGen),
	)
	log.Info(
		"block generation",
		zap.Duration("t", blockGen),
		zap.Int64("avg(ms)", blockGen.Milliseconds()/int64(blocks)),
		zap.Float64("tps", float64(txs)/blockGen.Seconds()),
	)
	for i, instance := range instances[1:] {
		// Get size of db dir after shutdown
		dbSize, err := dirSize(instance.dbDir)
		gomega.Ω(err).Should(gomega.BeNil())

		// Compute analysis
		parse1, parse2, parseDur := getHalfAverages(instance.parse)
		verify1, verify2, verifyDur := getHalfAverages(instance.verify)
		accept1, accept2, acceptDur := getHalfAverages(instance.accept)
		t := parseDur + verifyDur + acceptDur
		fb := float64(blocks)
		log.Info("block verification",
			zap.Int("instance", i+1),
			zap.Duration("t", time.Duration(t)),
			zap.Float64("parse(ms/b)", parseDur/fb*1000),
			zap.Float64("parse1(ms/b)", parse1*1000),
			zap.Float64("parse2(ms/b)", parse2*1000),
			zap.Float64("verify(ms/b)", verifyDur/fb*1000),
			zap.Float64("verify1(ms/b)", verify1*1000),
			zap.Float64("verify2(ms/b)", verify2*1000),
			zap.Float64("accept(ms/b)", acceptDur/fb*1000),
			zap.Float64("accept1(ms/b)", accept1*1000),
			zap.Float64("accept2(ms/b)", accept2*1000),
			zap.Float64("tps", float64(txs)/t),
			zap.Float64("disk size (MB)", dbSize),
		)
	}
})

type SeededRandReader struct {
	rand.Source
}

func NewSeededRandReader(seed int64) *SeededRandReader {
	return &SeededRandReader{
		Source: rand.NewSource(seed),
	}
}

func (s *SeededRandReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(s.Source.Int63() & 0xff)
	}
	return len(p), nil
}

var _ = ginkgo.Describe("load tests vm", func() {
	ginkgo.It("distributes funds", func() {
		ginkgo.By("create accounts", func() {
			senders = make([]*account, accts)
			reader := NewSeededRandReader(0) // deterministic key generation
			for i := 0; i < accts; i++ {
				var (
					tpriv ed25519.PrivateKey
					err   error
				)
				if deterministicKeys {
					tpriv, err = ed25519.GeneratePrivateKeyWithReader(reader)
				} else {
					tpriv, err = ed25519.GeneratePrivateKey()
				}
				gomega.Ω(err).Should(gomega.BeNil())
				trsender := auth.NewED25519Address(tpriv.PublicKey())
				tsender := codec.MustAddressBech32(consts.HRP, trsender)
				senders[i] = &account{tpriv, auth.NewED25519Factory(tpriv), trsender, tsender}
			}
		})

		ginkgo.By("load accounts", func() {
			// sending 1 tx to each account
			remainder := uint64(accts)*maxFee + uint64(100_000_000)
			// leave some left over for root
			fundSplit := (genesisBalance - remainder) / uint64(accts)
			gomega.Ω(fundSplit).Should(gomega.Not(gomega.BeZero()))
			log.Info("funding accounts", zap.Uint64("split", fundSplit), zap.Uint64("remainder", remainder))
			issuer := newSimpleTxIssuer()
			for _, acct := range senders {
				action := &actions.Transfer{
					Value: fundSplit,
					To:    acct.rsender,
				}
				issuer.issueTx(action, root.factory)
			}
			issuer.produceAndAcceptBlock(true)
		})

		var (
			abis     = make(map[string]*parsedABI)
			deployed = make(map[string]ethCommon.Address)
		)

		ginkgo.By("deploy contracts", func() {
			ctx := context.TODO()
			evmTxBuilder := &evmTxBuilder{
				actor: root.rsender,
				bcli:  instances[0].tcli,
			}

			// parse all the contracts
			for name, fn := range map[string]string{
				"TokenA":  "../../contracts/artifacts/contracts/TokenA.sol/TokenA.json",
				"TokenB":  "../../contracts/artifacts/contracts/TokenB.sol/TokenB.json",
				"WETH9":   "../../contracts/node_modules/@uniswap/v2-periphery/build/WETH9.json",
				"Factory": "../../contracts/node_modules/@uniswap/v2-core/build/UniswapV2Factory.json",
				"Router":  "../../contracts/node_modules/@uniswap/v2-periphery/build/UniswapV2Router02.json",
			} {
				abi, err := NewABI(fn)
				gomega.Ω(err).Should(gomega.BeNil())
				abis[name] = abi
			}

			type step struct {
				abi    string
				args   func() []interface{}
				method string
				to     func() *ethCommon.Address
			}

			owner := actions.ToEVMAddress(root.rsender)
			approvedAmount := big.NewInt(1_000_000_000_000_000_000)
			minLiquidity := big.NewInt(1)
			maxLiquidty := big.NewInt(1_000_000)
			deadline := big.NewInt(time.Now().Unix() + 1000)

			steps := []step{
				{
					abi: "TokenA",
				},
				{
					abi: "TokenB",
				},
				{
					abi: "WETH9",
				},
				{
					abi: "Factory",
					args: func() []interface{} {
						return []interface{}{owner}
					},
				},
				{
					abi: "Router",
					args: func() []interface{} {
						return []interface{}{deployed["Factory"], deployed["WETH9"]}
					},
				},
				{
					abi:    "Factory",
					method: "createPair",
					args: func() []interface{} {
						return []interface{}{deployed["TokenA"], deployed["TokenB"]}
					},
				},
				{
					abi:    "TokenA",
					method: "approve",
					args: func() []interface{} {
						return []interface{}{deployed["Router"], approvedAmount}
					},
				},
				{
					abi:    "TokenB",
					method: "approve",
					args: func() []interface{} {
						return []interface{}{deployed["Router"], approvedAmount}
					},
				},
				{
					abi:    "Router",
					method: "addLiquidity",
					args: func() []interface{} {
						return []interface{}{
							deployed["TokenA"],
							deployed["TokenB"],
							maxLiquidty,
							maxLiquidty,
							minLiquidity,
							minLiquidity,
							owner,
							deadline,
						}
					},
				},
			}

			issuer := newSimpleTxIssuer()
			for _, step := range steps {
				isDeploy := step.method == ""
				var args []interface{}
				if step.args != nil {
					args = step.args()
				}
				calldata, err := abis[step.abi].calldata(step.method, args...)
				gomega.Ω(err).Should(gomega.BeNil())

				var to *ethCommon.Address
				if step.to != nil {
					to = step.to()
				} else if !isDeploy {
					addr := deployed[step.abi]
					to = &addr
				}
				action, err := evmTxBuilder.evmCall(ctx, &Args{
					To:        to,
					Data:      calldata,
					FillNonce: true, // TODO: change to isDeploy
				})
				gomega.Ω(err).Should(gomega.BeNil())

				if isDeploy {
					deployed[step.abi] = crypto.CreateAddress(owner, action.Nonce)
				}

				issuer.issueTx(action, root.factory)
				issuer.produceAndAcceptBlock(true)
			}
		})

		ginkgo.By("send tokens to accounts", func() {
			ctx := context.TODO()
			evmTxBuilder := &evmTxBuilder{
				actor: root.rsender,
				bcli:  instances[0].tcli,
			}

			amount := big.NewInt(1_000)
			tokenA := deployed["TokenA"]
			tokenB := deployed["TokenB"]

			issuer := newSimpleTxIssuer()
			for _, acct := range senders {
				evmAddr := actions.ToEVMAddress(acct.rsender)
				calldataA, err := abis["TokenA"].calldata("transfer", evmAddr, amount)
				gomega.Ω(err).Should(gomega.BeNil())
				actionA, err := evmTxBuilder.evmCall(ctx, &Args{
					To:   &tokenA,
					Data: calldataA,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				issuer.issueTx(actionA, root.factory)

				calldataB, err := abis["TokenB"].calldata("transfer", evmAddr, amount)
				gomega.Ω(err).Should(gomega.BeNil())
				actionB, err := evmTxBuilder.evmCall(ctx, &Args{
					To:   &tokenB,
					Data: calldataB,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				issuer.issueTx(actionB, root.factory)
			}
			issuer.produceAndAcceptBlock(true)
		})

		ginkgo.By("approve tokens", func() {
			ctx := context.TODO()
			issuer := newSimpleTxIssuer()
			tokenA := deployed["TokenA"]
			tokenB := deployed["TokenB"]
			amount := big.NewInt(10_000_000)

			for _, acct := range senders {
				evmTxBuilder := &evmTxBuilder{
					actor: acct.rsender,
					bcli:  instances[0].tcli,
				}

				calldataApproveA, err := abis["TokenA"].calldata("approve", deployed["Router"], amount)
				gomega.Ω(err).Should(gomega.BeNil())
				actionAllowanceA, err := evmTxBuilder.evmCall(ctx, &Args{
					To:   &tokenA,
					Data: calldataApproveA,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				issuer.issueTx(actionAllowanceA, acct.factory)

				calldataApproveB, err := abis["TokenB"].calldata("approve", deployed["Router"], amount)
				gomega.Ω(err).Should(gomega.BeNil())
				actionAllowanceB, err := evmTxBuilder.evmCall(ctx, &Args{
					To:   &tokenB,
					Data: calldataApproveB,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				issuer.issueTx(actionAllowanceB, acct.factory)
			}
			issuer.produceAndAcceptBlock(true)
		})

		ginkgo.By("swap tokens", func() {
			ctx := context.TODO()
			router := deployed["Router"]
			routeAB := []ethCommon.Address{deployed["TokenA"], deployed["TokenB"]}
			routeBA := []ethCommon.Address{deployed["TokenB"], deployed["TokenA"]}

			amountOut := func(evmTxBuilder *evmTxBuilder, amountIn *big.Int, route []ethCommon.Address, slippageNum, slippageDen int) *big.Int {
				calldata, err := abis["Router"].calldata("getAmountsOut", amountIn, route)
				gomega.Ω(err).Should(gomega.BeNil())
				trace, _, err := evmTxBuilder.evmTraceCall(ctx, &Args{
					To:   &router,
					Data: calldata,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				var amountsOut []*big.Int
				err = abis["Router"].unpack("getAmountsOut", trace.Output, &amountsOut)
				gomega.Ω(err).Should(gomega.BeNil())
				expected := amountsOut[len(amountsOut)-1]
				expected = expected.Mul(expected, big.NewInt(int64(slippageNum)))
				expected = expected.Div(expected, big.NewInt(int64(slippageDen)))
				return expected
			}

			balanceOf := func(evmTxBuilder *evmTxBuilder, name string, token ethCommon.Address) *big.Int {
				owner := actions.ToEVMAddress(evmTxBuilder.actor)
				calldata, err := abis[name].calldata("balanceOf", owner)
				gomega.Ω(err).Should(gomega.BeNil())
				trace, _, err := evmTxBuilder.evmTraceCall(ctx, &Args{
					To:   &token,
					Data: calldata,
				})
				gomega.Ω(err).Should(gomega.BeNil())
				var balance *big.Int
				err = abis[name].unpack("balanceOf", trace.Output, &balance)
				gomega.Ω(err).Should(gomega.BeNil())
				return balance
			}

			txBuilders := make([]*evmTxBuilder, len(senders))
			for i, acct := range senders {
				txBuilders[i] = &evmTxBuilder{
					actor: acct.rsender,
					bcli:  instances[0].tcli,
				}
			}

			issuer := newSimpleTxIssuer()
			totalSuccess, totalTxs := 0, 0
			for round := 0; round < 100; round++ {
				amountIn := big.NewInt(200)
				// this avoids duplicate txs
				amountIn = amountIn.Sub(amountIn, big.NewInt(int64(round)))

				var minAmountOut *big.Int
				for i, acct := range senders {
					evmAddr := actions.ToEVMAddress(acct.rsender)

					var route []ethCommon.Address
					if (round+i)%2 == 0 {
						route = routeAB
					} else {
						route = routeBA
					}
					amountOut := amountOut(txBuilders[i], amountIn, route, 50, 100)
					if minAmountOut == nil || amountOut.Cmp(minAmountOut) < 0 {
						minAmountOut = amountOut
					}
					deadline := big.NewInt(time.Now().Unix() + 1000)
					calldata, err := abis["Router"].calldata(
						"swapExactTokensForTokens",
						amountIn,
						amountOut,
						route,
						evmAddr,
						deadline,
					)
					gomega.Ω(err).Should(gomega.BeNil())
					var action *actions.EvmCall
					for attempt := 0; attempt < 300; attempt++ {
						action, err = txBuilders[i].evmCall(ctx, &Args{
							To:   &router,
							Data: calldata,
						})
						if err != nil {
							balanceOfA := balanceOf(txBuilders[i], "TokenA", deployed["TokenA"])
							balanceOfB := balanceOf(txBuilders[i], "TokenB", deployed["TokenB"])
							log.Error("failed to create swap tx",
								zap.Int("try", attempt),
								zap.Error(err),
								zap.Int("round", round),
								zap.Int("account", i),
								zap.Stringer("amountIn", amountIn),
								zap.Stringer("amountOut", amountOut),
								zap.Stringer("balanceA", balanceOfA),
								zap.Stringer("balanceB", balanceOfB),
							)
							continue
						}
						break
					}
					gomega.Ω(err).Should(gomega.BeNil())
					issuer.issueTx(action, acct.factory)
				}
				// some trades may not succeed, so we don't require all txs to be in a block
				success, txs := issuer.produceAndAcceptBlock(false)

				// check balances
				totalA := big.NewInt(0)
				totalB := big.NewInt(0)
				totalNative := uint64(0)
				for i := range senders {
					native, err := instances[0].tcli.Balance(context.Background(), senders[i].sender)
					gomega.Ω(err).Should(gomega.BeNil())
					balanceA := balanceOf(txBuilders[i], "TokenA", deployed["TokenA"])
					balanceB := balanceOf(txBuilders[i], "TokenB", deployed["TokenB"])
					totalA = totalA.Add(totalA, balanceA)
					totalB = totalB.Add(totalB, balanceB)
					totalNative += native
				}
				ratio := float64(minAmountOut.Uint64()) / float64(amountIn.Uint64())
				log.Info(
					"swap round totals",
					zap.Int("round", round),
					zap.Int("success", success),
					zap.Int("txs", txs),
					zap.Stringer("TokenA", totalA),
					zap.Stringer("TokenB", totalB),
					zap.Stringer("minAmountOut", minAmountOut),
					zap.Float64("ratio", ratio),
					zap.Uint64("native", totalNative),
				)
				totalSuccess += success
				totalTxs += txs
			}
			log.Info("test summary",
				zap.Int("totalSuccess", totalSuccess),
				zap.Int("totalTxs", totalTxs),
			)
		})
	})

	ginkgo.It("creates blocks", func() {
		l := sync.Mutex{}
		allTxs := map[ids.ID]struct{}{}
		ginkgo.By("generate txs", func() {
			start := time.Now()
			w := workers.NewParallel(numWorkers, 10) // parallelize generation to speed things up
			j, err := w.NewJob(512)
			gomega.Ω(err).Should(gomega.BeNil())
			for i := 0; i < txs; i++ {
				j.Go(func() error {
					// TODO make this way more efficient
					var txID ids.ID
					for {
						// It is ok if a transfer is to self
						randSender := getAccount()
						randRecipient := getAccount()
						var terr error
						txID, terr = issueSimpleTx(
							instances[0],
							randRecipient.rsender,
							1,
							randSender.factory,
						)
						if terr == nil {
							break
						}
					}
					l.Lock()
					allTxs[txID] = struct{}{}
					if len(allTxs)%10_000 == 0 {
						log.Debug("generating txs", zap.Int("remaining", txs-len(allTxs)))
					}
					l.Unlock()
					return nil
				})
			}
			j.Done(nil)
			gomega.Ω(j.Wait()).Should(gomega.BeNil())
			txGen = time.Since(start)
		})

		ginkgo.By("producing blks", func() {
			start := time.Now()
			acceptCalls := []func(){}
			for {
				blk, accept := produceBlock(instances[0])
				if blk == nil {
					break
				}
				acceptCalls = append(acceptCalls, accept)
				log.Debug("block produced", zap.Uint64("height", blk.Hght), zap.Int("txs", len(blk.Txs)))
				for _, tx := range blk.Txs {
					delete(allTxs, tx.ID())
				}
				blks = append(blks, blk)

				// Accept blocks at some [acceptDepth]
				acceptIndex := len(acceptCalls) - 1 - acceptDepth
				if acceptIndex < 0 {
					continue
				}
				acceptCalls[acceptIndex]()
			}

			// Accept remaining blocks
			for i := len(acceptCalls) - acceptDepth; i < len(acceptCalls); i++ {
				acceptCalls[i]()
			}

			// Ensure all transactions included in a block
			gomega.Ω(len(allTxs)).To(gomega.BeZero())
			blockGen = time.Since(start)
		})
	})

	ginkgo.It("verifies blocks", func() {
		for i, instance := range instances[1:] {
			log.Warn("sleeping 2s before starting verification", zap.Int("instance", i+1))
			time.Sleep(2 * time.Second)

			acceptCalls := []func(){}
			ginkgo.By(fmt.Sprintf("sync instance %d", i+1), func() {
				for _, blk := range blks {
					acceptCalls = append(acceptCalls, addBlock(i+1, instance, blk))

					// Accept blocks at some [acceptDepth]
					acceptIndex := len(acceptCalls) - 1 - acceptDepth
					if acceptIndex < 0 {
						continue
					}
					acceptCalls[acceptIndex]()
				}

				// Accept remaining blocks
				for i := len(acceptCalls) - acceptDepth; i < len(acceptCalls); i++ {
					acceptCalls[i]()
				}
			})
		}
	})
})

type simpleTxIssuer struct {
	instances []*instance
	pending   map[ids.ID]struct{}
}

func newSimpleTxIssuer() *simpleTxIssuer {
	return &simpleTxIssuer{
		instances: instances,
		pending:   make(map[ids.ID]struct{}),
	}
}

func (s *simpleTxIssuer) issueTx(action chain.Action, signer chain.AuthFactory) {
	id, err := issueTx(s.instances[0], action, signer)
	gomega.Ω(err).Should(gomega.BeNil())
	s.pending[id] = struct{}{}
}

func (s *simpleTxIssuer) produceAndAcceptBlock(mustSucceed bool) (int, int) {
	instances := s.instances
	success, txs := 0, 0
	for {
		blk, accept := produceBlock(instances[0])
		if blk == nil {
			break
		}
		accept()

		txs += len(blk.Results())
		for _, result := range blk.Results() {
			if result.Success {
				success++
			}
			if !mustSucceed {
				continue
			}
			gomega.Ω(result.Success).Should(gomega.BeTrue())
		}
		log.Info("block produced",
			zap.Uint64("height", blk.Hght),
			zap.Int("txs", len(blk.Txs)),
			zap.Int("success", success),
		)
		for _, tx := range blk.Txs {
			delete(s.pending, tx.ID())
		}
		for i, instance := range instances[1:] {
			accept := addBlock(i+1, instance, blk)
			accept()
		}
	}
	gomega.Ω(len(s.pending)).To(gomega.BeZero())
	return success, txs
}

func issueSimpleTx(
	i *instance,
	to codec.Address,
	amount uint64,
	factory chain.AuthFactory,
) (ids.ID, error) {
	action := &actions.Transfer{
		To:    to,
		Value: amount,
	}
	return issueTx(i, action, factory)
}

func issueTx(
	i *instance,
	action chain.Action,
	factory chain.AuthFactory,
) (ids.ID, error) {
	tx := chain.NewTx(
		&chain.Base{
			Timestamp: hutils.UnixRMilli(-1, 100*hconsts.MillisecondsPerSecond),
			ChainID:   i.chainID,
			MaxFee:    maxFee,
		},
		nil,
		action,
	)
	tx, err := tx.Sign(factory, consts.ActionRegistry, consts.AuthRegistry)
	gomega.Ω(err).To(gomega.BeNil())
	_, err = i.cli.SubmitTx(context.TODO(), tx.Bytes())
	return tx.ID(), err
}

func produceBlock(i *instance) (*chain.StatelessBlock, func()) {
	ctx := context.TODO()

	blk, err := i.vm.BuildBlock(ctx)
	if errors.Is(err, chain.ErrNoTxs) {
		log.Debug("no more blocks to produce")
		return nil, nil
	}
	gomega.Ω(err).To(gomega.BeNil())
	gomega.Ω(blk).To(gomega.Not(gomega.BeNil()))

	gomega.Ω(blk.Verify(ctx)).To(gomega.BeNil())
	gomega.Ω(blk.Status()).To(gomega.Equal(choices.Processing))

	err = i.vm.SetPreference(ctx, blk.ID())
	gomega.Ω(err).To(gomega.BeNil())

	return blk.(*chain.StatelessBlock), func() {
		gomega.Ω(blk.Accept(ctx)).To(gomega.BeNil())
		gomega.Ω(blk.Status()).To(gomega.Equal(choices.Accepted))

		lastAccepted, err := i.vm.LastAccepted(ctx)
		gomega.Ω(err).To(gomega.BeNil())
		gomega.Ω(lastAccepted).To(gomega.Equal(blk.ID()))
	}
}

func addBlock(num int, i *instance, blk *chain.StatelessBlock) func() {
	ctx := context.TODO()
	start := time.Now()
	tblk, err := i.vm.ParseBlock(ctx, blk.Bytes())
	i.parse = append(i.parse, time.Since(start).Seconds())
	gomega.Ω(err).Should(gomega.BeNil())
	start = time.Now()
	gomega.Ω(tblk.Verify(ctx)).Should(gomega.BeNil(), "block failed verification (instance %d)", num)
	i.verify = append(i.verify, time.Since(start).Seconds())
	return func() {
		start = time.Now()
		gomega.Ω(tblk.Accept(ctx)).Should(gomega.BeNil())
		i.accept = append(i.accept, time.Since(start).Seconds())
	}
}

var _ common.AppSender = &appSender{}

type appSender struct {
	next      int
	instances []*instance
}

func (app *appSender) SendAppGossip(ctx context.Context, appGossipBytes []byte, _ int, _ int, _ int) error {
	n := len(app.instances)
	sender := app.instances[app.next].nodeID
	app.next++
	app.next %= n
	return app.instances[app.next].vm.AppGossip(ctx, sender, appGossipBytes)
}

func (*appSender) SendAppRequest(context.Context, set.Set[ids.NodeID], uint32, []byte) error {
	return nil
}

func (*appSender) SendAppResponse(context.Context, ids.NodeID, uint32, []byte) error {
	return nil
}

func (*appSender) SendAppError(context.Context, ids.NodeID, uint32, int32, string) error {
	return nil
}

func (*appSender) SendAppGossipSpecific(context.Context, set.Set[ids.NodeID], []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppRequest(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppError(context.Context, ids.ID, uint32, int32, string) error {
	return nil
}

func getAccount() *account {
	switch dist {
	case "uniform":
		return senders[rand.Intn(accts)] //nolint:gosec
	case "zipf":
		if z == nil {
			z = rand.NewZipf(rand.New(rand.NewSource(0)), 1.1, 2.0, uint64(accts)-1) //nolint:gosec
		}
		return senders[z.Uint64()]
	default:
		panic("invalid dist")
	}
}

// dirSize returns the size of a directory mesured in MB
func dirSize(path string) (float64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return float64(size) / 1024.0 / 1024.0, err
}

func getHalfAverages(v []float64) (float64, float64, float64 /* sum */) {
	var v1, v2, s float64
	for i, item := range v {
		if i < len(v)/2 {
			v1 += item
		} else {
			v2 += item
		}
		s += item
	}
	v1C := float64(len(v) / 2)
	v2C := float64(len(v)/2 + len(v)%2)
	return v1 / v1C, v2 / v2C, s
}
