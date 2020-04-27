package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	TotalCUSDSupply = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cusd_supply",
		Help:    "Total cUSD supply",
		Buckets: prometheus.LinearBuckets(20, 5, 5), // 5 buckets, each 5 centigrade wide.
	})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(TotalCUSDSupply)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
