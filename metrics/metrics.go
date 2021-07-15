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
		Name: "last_block_processed",
		Help: "Last Block Processed by eksportisto",
	})

	ProcessBlockDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "process_block_duration",
		Help:    "Time it takes to process a block",
		Buckets: prometheus.LinearBuckets(0, 0.2, 20),
	})

	VotingGoldFraction = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "epochrewards_votinggoldfraction",
		Help: "Voting Gold Fraction",
	})

	ExchangeCeloBucketSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_celo_bucket_size",
		Help: "CELO Bucket Size",
	}, []string{"stable_token"})
	ExchangeStableBucketSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_stable_bucket_size",
		Help: "Stable Token Bucket size",
	}, []string{"stable_token"})
	ExchangeCeloBucketRatio = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "exchange_celo_bucket_ratio",
		Help:    "CELO Bucket Ratio",
		Buckets: prometheus.LinearBuckets(0, 0.1, 10),
	}, []string{"stable_token"})
	ExchangeCeloExchangedRate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_celo_exchanged_rate",
		Help: "The implied Stable/CELO rate by exchanges",
	}, []string{"stable_token"})
	ExchangeBucketRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "exchange_bucket_ratio",
		Help: "The most recent ratio during BucketsUpdated",
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
	registerer := prometheus.WrapRegistererWithPrefix("eksportisto_", prometheus.DefaultRegisterer)
	// Register application metrics with the eksportisto_ prefix
	registerer.MustRegister(BlockGasUsed)
	registerer.MustRegister(CeloTokenSupply)
	registerer.MustRegister(GasPrice)
	registerer.MustRegister(VotingGoldFraction)
	registerer.MustRegister(LastBlockProcessed)
	registerer.MustRegister(ProcessBlockDuration)

	registerer.MustRegister(ExchangeCeloBucketSize)
	registerer.MustRegister(ExchangeStableBucketSize)
	registerer.MustRegister(ExchangeCeloBucketRatio)
	registerer.MustRegister(ExchangeCeloExchangedRate)
	registerer.MustRegister(ExchangeBucketRatio)
	registerer.MustRegister(ExchangeImpliedStableRate)

	registerer.MustRegister(SortedOraclesIsOldestReportExpired)
	registerer.MustRegister(SortedOraclesNumRates)
	registerer.MustRegister(SortedOraclesMedianRate)
	registerer.MustRegister(SortedOraclesRateMaxDeviation)
	registerer.MustRegister(SortedOraclesMedianTimestamp)

	// Add Go module build info with the default registry
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
