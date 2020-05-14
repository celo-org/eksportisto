package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	TotalCGLDSupply = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cgld_supply",
		Help: "Total cGLD supply",
	})
	TotalCUSDSupply = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cusd_supply",
		Help: "Total cUSD supply",
	})
	VotingGoldFraction = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "epochrewards_votinggoldfraction",
		Help: "Voting Gold Fraction",
	})
	ExchangeGoldBucketRatio = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "exchange_gold_bucket_ratio",
		Help:    "Gold Bucket Ratio",
		Buckets: prometheus.LinearBuckets(0, 0.1, 10),
	})
	LastBlockProcessed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "eksportisto_last_block_processed",
		Help: "Last Block Processed by eksportisto",
	})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(TotalCGLDSupply)
	prometheus.MustRegister(TotalCUSDSupply)
	prometheus.MustRegister(VotingGoldFraction)
	prometheus.MustRegister(ExchangeGoldBucketRatio)
	prometheus.MustRegister(LastBlockProcessed)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
