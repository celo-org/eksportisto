package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile     string
	userLicense string

	rootCmd = &cobra.Command{
		Use:   "eksportisto",
		Short: "Services to index the celo blockchain",
		Long: `Eksportisto is a set of services designed to index data from the 
celo blockchain in a reliable and distributed manner, and output that data
to multiple storage types per usecase.`,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.eksportisto.yaml)")
	rootCmd.PersistentFlags().Int("monitoring-port", 8080, "Port for the prometheus server")
	rootCmd.PersistentFlags().String("celo-node-uri", "", "URI for the Celo Blockchain Node")
	rootCmd.PersistentFlags().Bool("profiling", false, "Enable pprof on the http server")

	rootCmd.AddCommand(publisherCmd)
	rootCmd.AddCommand(indexerCmd)
	rootCmd.AddCommand(legacyCmd)
	rootCmd.AddCommand(monitorCmd)
}

func initConfig() {
	viper.SetDefault("monitoring.port", 8080)
	viper.SetDefault("monitoring.address", "127.0.0.1")
	viper.SetDefault("monitoring.requestTimeoutSeconds", 24)
	viper.SetDefault("indexer.indexingTimeout", "30s")
	viper.BindPFlag("monitoring.port", rootCmd.Flags().Lookup("monitoring-port"))
	viper.BindPFlag("indexer.mode", indexerCmd.Flags().Lookup("indexer-mode"))
	viper.BindPFlag("celoNodeURI", rootCmd.Flags().Lookup("celo-node-uri"))
	viper.BindPFlag("profiling", rootCmd.Flags().Lookup("profiling"))

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".eksportisto")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
