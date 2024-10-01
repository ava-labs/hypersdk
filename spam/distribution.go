package spam

import (
	"fmt"
	"math/rand"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/utils"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type ZipfDistribution struct {
	// zipf tuning params
	s float64
	v float64
	// whether to plot the distribution
	plot bool
}

func NewZipfDistribution(s, v float64, plot bool) *ZipfDistribution {
	return &ZipfDistribution{s: s, v: v, plot: plot}
}

// Expected number of unique participants every 60s
func (z *ZipfDistribution) ExpectedParticipants(numAccounts int, txsPerSecond int) {
	// Log Zipf participants
	zipfGenerator := rand.NewZipf(rand.New(rand.NewSource(0)), z.s, z.v, uint64(numAccounts)-1) //nolint:gosec
	trials := txsPerSecond * 60 * 2                                                      /* sender/receiver */
	unique := set.NewSet[uint64](trials)
	for i := 0; i < trials; i++ {
		unique.Add(zipfGenerator.Uint64())
	}
	utils.Outf("{{blue}}unique participants expected every 60s:{{/}} %d\n", unique.Len())
}

func (z *ZipfDistribution) Plot() error {
	if !z.plot {
		return nil
	}

	if err := ui.Init(); err != nil {
		return err
	}
	zb := rand.NewZipf(rand.New(rand.NewSource(0)), z.s, z.v, plotIdentities-1) //nolint:gosec
	distribution := make([]float64, plotIdentities)
	for i := 0; i < plotSamples; i++ {
		distribution[zb.Uint64()]++
	}
	labels := make([]string, plotIdentities)
	for i := 0; i < plotIdentities; i++ {
		labels[i] = fmt.Sprintf("%d", i)
	}
	bc := widgets.NewBarChart()
	bc.Title = fmt.Sprintf("Account Issuance Distribution (s=%.2f v=%.2f [(v+k)^(-s)])", z.s, z.v)
	bc.Data = distribution
	bc.Labels = labels
	bc.BarWidth = plotBarWidth
	bc.SetRect(0, 0, plotOverhead+plotBarWidth*plotIdentities, plotHeight)
	ui.Render(bc)
	utils.Outf("\ntype any character to continue...\n")
	for range ui.PollEvents() {
		break
	}
	ui.Close()
	return nil
}
