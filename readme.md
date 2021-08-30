# Eksportisto

Eksportisto (meaning 'exporter' in Esperanto) is a lightweight Celo blockchain parser we've built for internal and external use at cLabs. It will print all transactions (take a look in the [monitor directory](./monitor) to see exactly how it parses these) to standard out and additionally exposes Prometheus compatible metrics on port 8080.

Eksportisto uses SQLite to keep track of the last block parsed, so it is safe to start and stop without having to reparse the whole chain.

At cLabs we often rely on (Google's Operations (formerly Stackdriver))[https://cloud.google.com/products/operations] to collect these standard out logs and derive insights.

## How do I use it?

### Running a full node

We'd recommend running a Celo full node on the same network as Eksportisto. Taks a look at our [documentation for running a full node](https://docs.celo.org/getting-started/mainnet/running-a-full-node-in-mainnet) if you haven't already.

In addition to the steps in the above guide, you'll also need to make sure you run your full node with the following command line arguments:

- `--ws`
- `--wsapi eth,net,web3,debug`
- `--wsaddr 0.0.0.0`
- `--gcmode archive`

### Starting up Eksportisto

Eksportisto consists of two services that intermediate through Redis queues (lists not pub-sub).

- `publisher`: is responsible for queueing blocks that need to be processed
- `worker`: is responsible for processing blocks from a queue

#### Configuration

The services use [spf13/viper](https://github.com/spf13/viper) and [spf13/cobra](https://github.com/spf13/cobra) to handle commands and configuration. So in order to configure the service you need to copy `config.yaml.example` to `config.yaml` and setup the relevant configuration there.

##### global

```yaml
monitoring:
  port: 8080
  address: localhost
  requestTimeoutSeconds: 25

celoNodeURI: ws://localhost:8546
sensitiveAccountsFilePath: ~/sensitiveAccounts.json
profiling: true

redis:
  address: localhost:6379
  password: ""
  db: 5
```

Pretty self explanatory, the most important things to configure here when running locally are:

- `celoNodeURI` which needs to be an archive node if you want to process an arbitrary block, or a full-node if you're following the tip of the chain.
- `redis` which needs to point to a local redis server

##### publisher

```yaml
publisher
  backfill: 
    enabled: true
    startBlock: 0
    batchSize: 100
    tipBuffer: 5
    sleepIntervalMilliseconds: 100
  chainFollower:
    enabled: false
```

The publisher has two modes of operation which can be enabled/disabled:

- `backfill` - will queue historical blocks which aren't marked as "indexed" in Redis starting from the `startBlock` in `batchSize` chunks. It maintains a cursor of the max(blockNumber) where all blocks between startBlock and the cursor are marked as successfully indexed in Redis. It will continuously attempt to enqueue blocks that fail for whatever reason, causing the system to stall. This is a desired effect and should result in humans getting involved to see what's causing that block to fail. Blocks are queued on the `blocks:queue:backfill` queue. When the cursor is at the tip of the chain, the backfill has a buffer of `tipBuffer` that it uses to not queue blocks close to the tip, to avoid both indexers processing the same block, even though that shouldn't be problematic if it happens. 

- `chainFollower` - maintains a subscription to a celo node and publishes blocks as they show up to the `blocks:queue:tip` queue

##### indexer

```yaml
indexer:
  bigquery:
    projectID: celo-testnet-production
    dataset: rc1_eksportisto_14
    table: test
  source: backfill
  destination: bigquery
  sleepIntervalMilliseconds: 100
  blockRetryAttempts: 3
  blockRetryDelayMilliseconds: 100
```

The indexer gets blocks from the configured `source`, processes them and writes them to the `destination`.
The `source` can be either `backfill` or `tip`, representing the two queues that the `publisher` writes to, and the `destination` can be either `bigquery` or `stdout`. `Destinations` implement a simple interface and can be easily extended.

When in `bigquery` mode additional fields are required and the `GOOGLE_APPLICATION_CREDENTIALS` must be set to a key file that has permissions to write to that table.

##### CLI arguments vs config file

Most of the configuration comes from the config file but some values there are bound to CLI arguments and can be overridden. Run `go run main.go --help` for more information.

#### Running locally

To run `eksportisto` locally you need an instance of Redis running and then you have flexibility depending on what you want to achieve:

##### Process a single block

Sstart the `indexer` with `source: backfill`:

```bash
> go run main.go indexer --config ./config.yaml --monitoring-port 8080 --indexer-source=backfill
``` 

Using a redis client push a block number to the backfill queue:

```redis
> rpush blocks:queue:backfill <block number>
```

##### Follow the chain

Start the `indexer` with `source: tip`:

```bash
> go run main.go indexer --config ./config.yaml --monitoring-port 8080 --indexer-source=backfill
```

And then start the publisher:

```bash
> go run main.go publisher --config ./config.yaml --monitoring-port 8081 
```

### Deployment steps

0. Switch to the right project with gcloud cli `gcloud config set project <project name>`
1. Update the env file of the network you want to deploy to (.env, or env.baklava, env.alfajores, etc) with the docker image hash
2. Update suffix (so you don't overwrite)
3. Make sure to have this env variables set in your terminal
  . `GETH_ENABLE_METRICS=false`
  . `GOOGLE_APPLICATION_CREDENTIALS=false`
4. Install terraform v0.12 if you haven't already
  . Download it from this [this link](https://releases.hashicorp.com/terraform/0.12.28/terraform_0.12.28_darwin_amd64.zip)
  . Install it with`mv ~/Downloads/terraform /usr/local/bin/`
5. It's a know issue that `celo_tf_state` should be replaced for `celo_tf_state_prod` in [this file](https://github.com/celo-org/celo-monorepo/blob/master/packages/terraform-modules/testnet/main.tf#L15)
6. Finally deploy with celotool: `celotooljs deploy initial eksportisto -e <env_name> --verbose --yesreally`
