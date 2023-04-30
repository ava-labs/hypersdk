// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/workers"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
)

const (
	genesisBalance  uint64 = hconsts.MaxUint64
	transferTxUnits        = 472
	maxTxsPerBlock  int    = 1_800_000 /* max block units */ / transferTxUnits
)

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
	priv    crypto.PrivateKey
	factory *auth.ED25519Factory
	rsender crypto.PublicKey
	sender  string
}

var (
	dist  string
	vms   int
	accts int
	txs   int
	trace bool

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
}

func TestLoad(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "indexvm load test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	gomega.Ω(dist).Should(gomega.BeElementOf([]string{"uniform", "zipf"}))
	gomega.Ω(vms).Should(gomega.BeNumerically(">", 1))

	var err error
	priv, err := crypto.GeneratePrivateKey()
	gomega.Ω(err).Should(gomega.BeNil())
	rsender := priv.PublicKey()
	sender := utils.Address(rsender)
	root = &account{priv, auth.NewED25519Factory(priv), rsender, sender}
	log.Debug(
		"generated root key",
		zap.String("addr", sender),
		zap.String("pk", hex.EncodeToString(priv[:])),
	)

	// create embedded VMs
	instances = make([]*instance, vms)
	gen = genesis.Default()
	gen.WindowTargetUnits = 1_000_000_000  // disable unit price increase
	gen.WindowTargetBlocks = 1_000_000_000 // disable block cost increase
	gen.ValidityWindow = 100_000           // txs shouldn't expire
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
		pdb, err := pebble.New(dname, pebble.NewDefaultConfig())
		gomega.Ω(err).Should(gomega.BeNil())
		db, err := manager.NewManagerFromDBs([]*manager.VersionedDatabase{
			{
				Database: pdb,
				Version:  avago_version.CurrentDatabase,
			},
		})
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
		err = c.Initialize(
			context.TODO(),
			snowCtx,
			db,
			genesisBytes,
			nil,
			[]byte(
				fmt.Sprintf(
					`{%s"parallelism":%d, "mempoolSize":%d, "mempoolPayerSize":%d, "testMode":true}`,
					tracePrefix,
					numWorkers,
					txs,
					txs,
				),
			),
			toEngine,
			nil,
			app,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		var hd map[string]*common.HTTPHandler
		hd, err = c.CreateHandlers(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		jsonRPCServer := httptest.NewServer(hd[rpc.JSONRPCEndpoint].Handler)
		tjsonRPCServer := httptest.NewServer(hd[trpc.JSONRPCEndpoint].Handler)
		instances[i] = &instance{
			chainID:            snowCtx.ChainID,
			nodeID:             snowCtx.NodeID,
			vm:                 c,
			toEngine:           toEngine,
			JSONRPCServer:      jsonRPCServer,
			TokenJSONRPCServer: tjsonRPCServer,
			cli:                rpc.NewJSONRPCClient(jsonRPCServer.URL),
			tcli:               trpc.NewJSONRPCClient(tjsonRPCServer.URL, snowCtx.ChainID),
			dbDir:              dname,
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		c.ForceReady()
	}

	// Verify genesis allocations loaded correctly (do here otherwise test may
	// check during and it will be inaccurate)
	for _, inst := range instances {
		cli := inst.tcli
		g, err := cli.Genesis(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())

		for _, alloc := range g.CustomAllocation {
			bal, err := cli.Balance(context.Background(), alloc.Address, ids.Empty)
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

var _ = ginkgo.Describe("load tests vm", func() {
	ginkgo.It("distributes funds", func() {
		ginkgo.By("create accounts", func() {
			senders = make([]*account, accts)
			for i := 0; i < accts; i++ {
				tpriv, err := crypto.GeneratePrivateKey()
				gomega.Ω(err).Should(gomega.BeNil())
				trsender := tpriv.PublicKey()
				tsender := utils.Address(trsender)
				senders[i] = &account{tpriv, auth.NewED25519Factory(tpriv), trsender, tsender}
			}
		})

		ginkgo.By("load accounts", func() {
			// sending 1 tx to each account
			remainder := uint64(accts)*transferTxUnits + uint64(1_000_000)
			// leave some left over for root
			fundSplit := (genesisBalance - remainder) / uint64(accts)
			gomega.Ω(fundSplit).Should(gomega.Not(gomega.BeZero()))
			requiredBlocks := accts / maxTxsPerBlock
			if accts%maxTxsPerBlock > 0 {
				requiredBlocks++
			}
			requiredTxs := map[ids.ID]struct{}{}
			for _, acct := range senders {
				id, err := issueSimpleTx(instances[0], acct.rsender, fundSplit, root.factory)
				gomega.Ω(err).Should(gomega.BeNil())
				requiredTxs[id] = struct{}{}
			}

			for i := 0; i < requiredBlocks; i++ {
				blk := produceBlock(instances[0])
				log.Debug("block produced", zap.Int("txs", len(blk.Txs)))
				for _, result := range blk.Results() {
					if !result.Success {
						// Used for debugging
						fmt.Println(string(result.Output), i, requiredBlocks)
					}
					gomega.Ω(result.Success).Should(gomega.BeTrue())
				}
				for _, tx := range blk.Txs {
					delete(requiredTxs, tx.ID())
				}
				for _, instance := range instances[1:] {
					addBlock(instance, blk)
				}
			}

			gomega.Ω(requiredTxs).To(gomega.BeEmpty())
		})
	})

	ginkgo.It("creates blocks", func() {
		l := sync.Mutex{}
		allTxs := map[ids.ID]struct{}{}
		ginkgo.By("generate txs", func() {
			start := time.Now()
			w := workers.New(numWorkers, 10) // parallelize generation to speed things up
			j, err := w.NewJob(512)
			gomega.Ω(err).Should(gomega.BeNil())
			for i := 0; i < txs; i++ {
				j.Go(func() error {
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
			requiredBlocks := txs / maxTxsPerBlock
			if txs%maxTxsPerBlock > 0 {
				requiredBlocks++
			}
			for i := 0; i < requiredBlocks; i++ {
				blk := produceBlock(instances[0])
				log.Debug("block produced", zap.Int("txs", len(blk.Txs)))
				for _, tx := range blk.Txs {
					delete(allTxs, tx.ID())
				}
				blks = append(blks, blk)
			}
			gomega.Ω(allTxs).To(gomega.BeEmpty())
			blockGen = time.Since(start)
		})
	})

	ginkgo.It("verifies blocks", func() {
		for i, instance := range instances[1:] {
			log.Warn("sleeping 10s before starting verification", zap.Int("instance", i+1))
			time.Sleep(10 * time.Second)
			ginkgo.By(fmt.Sprintf("sync instance %d", i+1), func() {
				for _, blk := range blks {
					addBlock(instance, blk)
				}
			})
		}
	})
})

func issueSimpleTx(
	i *instance,
	to crypto.PublicKey,
	amount uint64,
	factory chain.AuthFactory,
) (ids.ID, error) {
	tx := chain.NewTx(
		&chain.Base{
			Timestamp: time.Now().Unix() + 100_000,
			ChainID:   i.chainID,
			UnitPrice: 1,
		},
		nil,
		&actions.Transfer{
			To:    to,
			Value: amount,
		},
	)
	tx, err := tx.Sign(factory, consts.ActionRegistry, consts.AuthRegistry)
	gomega.Ω(err).To(gomega.BeNil())
	verify := tx.AuthAsyncVerify()
	gomega.Ω(verify()).To(gomega.BeNil())
	_, err = i.cli.SubmitTx(context.TODO(), tx.Bytes())
	return tx.ID(), err
}

func produceBlock(i *instance) *chain.StatelessBlock {
	ctx := context.TODO()

	blk, err := i.vm.BuildBlock(ctx)
	gomega.Ω(err).To(gomega.BeNil())
	gomega.Ω(blk).To(gomega.Not(gomega.BeNil()))

	gomega.Ω(blk.Verify(ctx)).To(gomega.BeNil())
	gomega.Ω(blk.Status()).To(gomega.Equal(choices.Processing))

	err = i.vm.SetPreference(ctx, blk.ID())
	gomega.Ω(err).To(gomega.BeNil())

	gomega.Ω(blk.Accept(ctx)).To(gomega.BeNil())
	gomega.Ω(blk.Status()).To(gomega.Equal(choices.Accepted))

	lastAccepted, err := i.vm.LastAccepted(ctx)
	gomega.Ω(err).To(gomega.BeNil())
	gomega.Ω(lastAccepted).To(gomega.Equal(blk.ID()))

	return blk.(*chain.StatelessBlock)
}

func addBlock(i *instance, blk *chain.StatelessBlock) {
	ctx := context.TODO()
	start := time.Now()
	tblk, err := i.vm.ParseBlock(ctx, blk.Bytes())
	i.parse = append(i.parse, time.Since(start).Seconds())
	gomega.Ω(err).Should(gomega.BeNil())
	start = time.Now()
	gomega.Ω(tblk.Verify(ctx)).Should(gomega.BeNil())
	i.verify = append(i.verify, time.Since(start).Seconds())
	start = time.Now()
	gomega.Ω(tblk.Accept(ctx)).Should(gomega.BeNil())
	i.accept = append(i.accept, time.Since(start).Seconds())
}

var _ common.AppSender = &appSender{}

type appSender struct {
	next      int
	instances []*instance
}

func (app *appSender) SendAppGossip(ctx context.Context, appGossipBytes []byte) error {
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

func (*appSender) SendAppGossipSpecific(context.Context, set.Set[ids.NodeID], []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppRequest(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
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
