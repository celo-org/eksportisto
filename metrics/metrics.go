package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	TotalCUSDSupply = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cusd_supply",
		Help:    "Total cUSD supply",
		Buckets: prometheus.LinearBuckets(20, 5, 5), // 5 buckets, each 5 centigrade wide.
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
	prometheus.MustRegister(TotalCUSDSupply)
	prometheus.MustRegister(ExchangeGoldBucketRatio)
	prometheus.MustRegister(LastBlockProcessed)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
