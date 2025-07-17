package config

import (
	"github.com/XanderD99/disruptor/internal/lavalink"
	"github.com/XanderD99/disruptor/internal/metrics"
	"github.com/XanderD99/disruptor/pkg/db/file"
	"github.com/XanderD99/disruptor/pkg/db/mongo"
	"github.com/XanderD99/disruptor/pkg/logging"

	"github.com/caarlos0/env/v11"
)

//go:generate envdoc -output README.md -types * -files ./internal/config/config.go -dir ../..  -env-prefix CONFIG_ -tag-default default
//go:generate envdoc -output ../../configs/.env.example -types * -files ./internal/config/config.go -dir ../..  -env-prefix CONFIG_ -tag-default default -format dotenv
type Config struct {
	// 🔑 The bot token used to connect to Discord
	Token string `env:"TOKEN,required"`

	// 🔢 Shard ID to use, 0 for automatic assignment
	ShardID int `env:"SHARD_ID" default:"0"`
	// 🔢 Total number of shards to use, 0 for automatic calculation
	ShardCount int `env:"SHARD_COUNT" default:"1"`

	// 📜 Logging configuration for the bot
	Logging logging.Config `envPrefix:"LOGGING_"`
	// 📊 Metrics configuration for Prometheus
	Metrics metrics.Config `envPrefix:"METRICS_" `

	// 🔗 List of Lavalink nodes to connect to
	LavalinkNodes []lavalink.Node `envPrefix:"LAVALINK_NODE"`

	// 🗄️ Configuration for the database
	Database struct {
		Type  string       `env:"TYPE" default:"mongo"`
		Mongo mongo.Config `envPrefix:"MONGO_"`
		File  file.Config  `envPrefix:"FILE_"`
	} `envPrefix:"DATABASE_"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{
		Prefix:              "CONFIG_",
		DefaultValueTagName: "default",
	})

	return cfg, err
}
