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
	SortedOraclesIsOldestReportExpired = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sortedoracles_isoldestreportexpired",
		Help: "True if oldest oracle report is expired",
	})
	SortedOraclesNumRates = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sortedoracles_numrates",
		Help: "The number of rates",
	})
	SortedOraclesMedianRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sortedoracles_medianrate",
		Help: "The median rate",
	})
	SortedOraclesRateMaxDeviation = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sortedoracles_rates_maxdeviation",
		Help: "The max deviation of all rates",
	})
	SortedOraclesMedianTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sortedoracles_timestamp_median",
		Help: "The median timestamp difference with the last blocktime",
	})
	ExchangeCeloExchangedRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exchange_celo_exchanged_rate",
		Help: "The implied cUSD/CELO rate by exchanges with Exchange.sol",
	})
	ExchangeBucketRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exchange_bucket_ratio",
		Help: "The most recent ratio during BucketsExchanged",
	})
	ExchangeImpliedStableRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exchange_implied_stable_rate",
		Help: "The implied cUSD/USD rate by the exchange and oracles",
	})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(TotalCGLDSupply)
	prometheus.MustRegister(TotalCUSDSupply)
	prometheus.MustRegister(VotingGoldFraction)
	prometheus.MustRegister(ExchangeGoldBucketRatio)
	prometheus.MustRegister(LastBlockProcessed)
	prometheus.MustRegister(SortedOraclesIsOldestReportExpired)
	prometheus.MustRegister(SortedOraclesNumRates)
	prometheus.MustRegister(SortedOraclesMedianRate)
	prometheus.MustRegister(SortedOraclesRateMaxDeviation)
	prometheus.MustRegister(SortedOraclesMedianTimestamp)
	prometheus.MustRegister(ExchangeCeloExchangedRate)
	prometheus.MustRegister(ExchangeBucketRatio)
	prometheus.MustRegister(ExchangeImpliedStableRate)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
