package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	BlockGasUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "block_gas_used",
		Help: "Gas used in a block",
	})

	CeloTokenSupply = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "celo_token_supply",
		Help: "Total supply of a supported token",
	}, []string{"token"})

	GasPrice = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gas_price",
		Help: "Gas paid in a tx",
	})

	LastBlockProcessed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "eksportisto_last_block_processed",
		Help: "Last Block Processed by eksportisto",
	})

	VotingGoldFraction = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "epochrewards_votinggoldfraction",
		Help: "Voting Gold Fraction",
	})

	ExchangeCeloBucketSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:    "exchange_celo_bucket_size",
		Help:    "CELO Bucket Size",
	}, []string{"stable_token"})
	ExchangeStableBucketSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:    "exchange_stable_bucket_size",
		Help:    "Stable Token Bucket size",
	}, []string{"stable_token"})
	ExchangeCeloBucketRatio = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "exchange_celo_bucket_ratio",
		Help:    "CELO Bucket Ratio",
		Buckets: prometheus.LinearBuckets(0, 0.1, 10),
	}, []string{"stable_token"})
	ExchangeCeloExchangedRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_celo_exchanged_rate",
		Help: "The implied Stable/CELO rate by exchanges with Exchange.sol",
	}, []string{"stable_token"})
	ExchangeBucketRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_bucket_ratio",
		Help: "The most recent ratio during BucketsExchanged",
	}, []string{"stable_token"})
	ExchangeImpliedStableRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_implied_stable_rate",
		Help: "The implied Stable/USD rate by the exchange and oracles",
	}, []string{"stable_token"})

	SortedOraclesIsOldestReportExpired = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sortedoracles_isoldestreportexpired",
		Help: "True if oldest oracle report is expired",
	}, []string{"token"})
	SortedOraclesNumRates = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sortedoracles_numrates",
		Help: "The number of rates",
	}, []string{"token"})
	SortedOraclesMedianRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sortedoracles_medianrate",
		Help: "The median rate",
	}, []string{"token"})
	SortedOraclesRateMaxDeviation = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sortedoracles_rates_maxdeviation",
		Help: "The max deviation of all rates",
	}, []string{"token"})
	SortedOraclesMedianTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sortedoracles_timestamp_median",
		Help: "The median timestamp difference with the last blocktime",
	}, []string{"token"})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(BlockGasUsed)
	prometheus.MustRegister(CeloTokenSupply)
	prometheus.MustRegister(GasPrice)
	prometheus.MustRegister(VotingGoldFraction)
	prometheus.MustRegister(LastBlockProcessed)

	prometheus.MustRegister(ExchangeCeloBucketSize)
	prometheus.MustRegister(ExchangeStableBucketSize)
	prometheus.MustRegister(ExchangeCeloBucketRatio)
	prometheus.MustRegister(ExchangeCeloExchangedRate)
	prometheus.MustRegister(ExchangeBucketRatio)
	prometheus.MustRegister(ExchangeImpliedStableRate)

	prometheus.MustRegister(SortedOraclesIsOldestReportExpired)
	prometheus.MustRegister(SortedOraclesNumRates)
	prometheus.MustRegister(SortedOraclesMedianRate)
	prometheus.MustRegister(SortedOraclesRateMaxDeviation)
	prometheus.MustRegister(SortedOraclesMedianTimestamp)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
