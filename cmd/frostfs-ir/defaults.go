package main

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

func newConfig(path string) (*viper.Viper, error) {
	const innerRingPrefix = "neofs_ir"

	var (
		err error
		v   = viper.New()
	)

	v.SetEnvPrefix(innerRingPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	defaultConfiguration(v)

	if path != "" {
		v.SetConfigFile(path)
		if strings.HasSuffix(path, ".json") {
			v.SetConfigType("json")
		} else {
			v.SetConfigType("yml")
		}
		err = v.ReadInConfig()
	}

	return v, err
}

func defaultConfiguration(cfg *viper.Viper) {
	cfg.SetDefault("logger.level", "info")

	cfg.SetDefault("pprof.address", "localhost:6060")
	cfg.SetDefault("pprof.shutdown_timeout", "30s")

	cfg.SetDefault("prometheus.address", "localhost:9090")
	cfg.SetDefault("prometheus.shutdown_timeout", "30s")

	cfg.SetDefault("without_mainnet", false)

	cfg.SetDefault("node.persistent_state.path", ".frostfs-ir-state")

	cfg.SetDefault("morph.endpoint.client", []string{})
	cfg.SetDefault("morph.dial_timeout", 15*time.Second)
	cfg.SetDefault("morph.validators", []string{})
	cfg.SetDefault("morph.switch_interval", 2*time.Minute)

	cfg.SetDefault("mainnet.endpoint.client", []string{})
	cfg.SetDefault("mainnet.dial_timeout", 15*time.Second)
	cfg.SetDefault("mainnet.switch_interval", 2*time.Minute)

	cfg.SetDefault("wallet.path", "")     // inner ring node NEP-6 wallet
	cfg.SetDefault("wallet.address", "")  // account address
	cfg.SetDefault("wallet.password", "") // password

	cfg.SetDefault("contracts.netmap", "")
	cfg.SetDefault("contracts.frostfs", "")
	cfg.SetDefault("contracts.balance", "")
	cfg.SetDefault("contracts.container", "")
	cfg.SetDefault("contracts.audit", "")
	cfg.SetDefault("contracts.proxy", "")
	cfg.SetDefault("contracts.processing", "")
	cfg.SetDefault("contracts.reputation", "")
	cfg.SetDefault("contracts.subnet", "")
	cfg.SetDefault("contracts.proxy", "")

	cfg.SetDefault("timers.emit", "0")
	cfg.SetDefault("timers.stop_estimation.mul", 1)
	cfg.SetDefault("timers.stop_estimation.div", 4)
	cfg.SetDefault("timers.collect_basic_income.mul", 1)
	cfg.SetDefault("timers.collect_basic_income.div", 2)
	cfg.SetDefault("timers.distribute_basic_income.mul", 3)
	cfg.SetDefault("timers.distribute_basic_income.div", 4)

	cfg.SetDefault("workers.netmap", "10")
	cfg.SetDefault("workers.balance", "10")
	cfg.SetDefault("workers.frostfs", "10")
	cfg.SetDefault("workers.container", "10")
	cfg.SetDefault("workers.alphabet", "10")
	cfg.SetDefault("workers.reputation", "10")
	cfg.SetDefault("workers.subnet", "10")

	cfg.SetDefault("netmap_cleaner.enabled", true)
	cfg.SetDefault("netmap_cleaner.threshold", 3)

	cfg.SetDefault("emit.storage.amount", 0)
	cfg.SetDefault("emit.mint.cache_size", 1000)
	cfg.SetDefault("emit.mint.threshold", 1)
	cfg.SetDefault("emit.mint.value", 20000000) // 0.2 Fixed8
	cfg.SetDefault("emit.gas.balance_threshold", 0)

	cfg.SetDefault("audit.task.exec_pool_size", 10)
	cfg.SetDefault("audit.task.queue_capacity", 100)
	cfg.SetDefault("audit.timeout.get", "5s")
	cfg.SetDefault("audit.timeout.head", "5s")
	cfg.SetDefault("audit.timeout.rangehash", "5s")
	cfg.SetDefault("audit.timeout.search", "10s")
	cfg.SetDefault("audit.pdp.max_sleep_interval", "5s")
	cfg.SetDefault("audit.pdp.pairs_pool_size", "10")
	cfg.SetDefault("audit.por.pool_size", "10")

	cfg.SetDefault("settlement.basic_income_rate", 0)
	cfg.SetDefault("settlement.audit_fee", 0)

	cfg.SetDefault("indexer.cache_timeout", 15*time.Second)

	cfg.SetDefault("locode.db.path", "")

	// extra fee values for working mode without notary contract
	cfg.SetDefault("fee.main_chain", 5000_0000)                  // 0.5 Fixed8
	cfg.SetDefault("fee.side_chain", 2_0000_0000)                // 2.0 Fixed8
	cfg.SetDefault("fee.named_container_register", 25_0000_0000) // 25.0 Fixed8

	cfg.SetDefault("control.authorized_keys", []string{})
	cfg.SetDefault("control.grpc.endpoint", "")

	cfg.SetDefault("governance.disable", false)
}
