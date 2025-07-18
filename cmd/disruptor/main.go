package main

import (
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/sharding"

	"github.com/XanderD99/disruptor/internal/commands"
	"github.com/XanderD99/disruptor/internal/config"
	"github.com/XanderD99/disruptor/internal/disruptor"
	"github.com/XanderD99/disruptor/internal/handlers"
	"github.com/XanderD99/disruptor/internal/lavalink"
	"github.com/XanderD99/disruptor/internal/metrics"
	"github.com/XanderD99/disruptor/internal/scheduler"
	"github.com/XanderD99/disruptor/pkg/db"
	"github.com/XanderD99/disruptor/pkg/db/mongo"
	"github.com/XanderD99/disruptor/pkg/logging"
	"github.com/XanderD99/disruptor/pkg/processes"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	logger, err := logging.New(cfg.Logging)
	if err != nil {
		log.Fatalf("Error initializing logger: %v", err)
	}

	pm := processes.NewManager(logger)

	pg, err := httpServers(cfg)
	if err != nil {
		log.Fatalf("Error initializing HTTP servers: %v", err)
	}
	pm.AddProcessGroup(pg)

	pg, database, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	pm.AddProcessGroup(pg)

	pg, err = initDiscordProcesses(cfg, logger, database)
	if err != nil {
		log.Fatalf("Error initializing Discord processes: %v", err)
	}

	pm.AddProcessGroup(pg)

	if err := pm.Run(); err != nil {
		log.Fatalf("Error running process manager: %v", err)
	}
}

func httpServers(cfg config.Config) (*processes.ProcessGroup, error) {
	group := processes.NewGroup("http", time.Second*5)

	// Initialize metrics server
	metricsServer, err := metrics.NewServer(cfg.Metrics)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics server: %w", err)
	}
	group.AddProcessWithCtx("metrics-server", metricsServer.Run, false, nil)

	return group, nil
}

func initDatabase(cfg config.Config) (*processes.ProcessGroup, db.Database, error) {
	group := processes.NewGroup("database", time.Second*5)

	switch cfg.Database.Type {
	case "mongo":
		db := mongo.New(cfg.Database.Mongo)
		group.AddProcessWithCtx("mongo", db.Connect, false, db.Disconnect)
		return group, db, nil
	}

	return group, nil, fmt.Errorf("invalid database type: %s", cfg.Database.Type)
}

func initDiscordProcesses(cfg config.Config, logger *slog.Logger, database db.Database) (*processes.ProcessGroup, error) {
	group := processes.NewGroup("discord", time.Second*5)

	session, err := disruptor.New(cfg.Token,
		bot.WithShardManagerConfigOpts(
			sharding.WithLogger(logger),
			sharding.WithShardCount(2),
			sharding.WithShardIDs(0, 1),
			sharding.WithAutoScaling(true),
			sharding.WithGatewayConfigOpts(
				gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates, gateway.IntentGuildExpressions),
				gateway.WithCompress(true),
				gateway.WithPresenceOpts(
					gateway.WithListeningActivity("to your commands"),
				),
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagsAll),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord bot: %w", err)
	}
	group.AddProcessWithCtx("bot", session.Open, false, session.Close)

	lava := lavalink.New(cfg.LavalinkNodes, session, logger)
	group.AddProcessWithCtx("disgolink", lava.Start, false, nil)

	manager := scheduler.NewManager(logger, session, database, lava)
	group.AddProcess("voice-audio-scheduler", manager.Start, false, manager.Stop)

	err = session.AddCommands(
		commands.Play(lava),
		commands.Disconnect(lava),
		commands.Next(manager),
		commands.Interval(database, manager),
		commands.Chance(database),
	)
	if err != nil {
		return nil, fmt.Errorf("error adding commands: %w", err)
	}

	session.AddEventListeners(
		bot.NewListenerFunc(handlers.VoiceStateUpdate(logger, lava)),
		bot.NewListenerFunc(handlers.VoiceServerUpdate(logger, lava)),

		bot.NewListenerFunc(handlers.GuildJoin(logger, database, manager)),
		bot.NewListenerFunc(handlers.GuildLeave(logger, database, manager)),

		bot.NewListenerFunc(handlers.GuildReady(logger, database, manager)),
	)

	return group, nil
}
