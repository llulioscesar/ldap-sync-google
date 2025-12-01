package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/startcodex/ldap-google-sync/internal/config"
	"github.com/startcodex/ldap-google-sync/internal/sync"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)

	log.Println("LDAP to Google Workspace Sync Tool")
	log.Println("Version: 2.1.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	interval := parseSyncInterval()
	printConfigSummary(cfg, interval)

	if interval == 0 {
		// Ejecución única
		runOnce(ctx, cfg)
	} else {
		// Ejecución periódica
		runPeriodic(ctx, cfg, interval)
	}
}

// parseSyncInterval parsea la variable SYNC_INTERVAL
// Formatos soportados: 1h, 30m, 1h30m, 2h, etc.
// Si está vacío o es 0, ejecuta una sola vez
func parseSyncInterval() time.Duration {
	intervalStr := os.Getenv("SYNC_INTERVAL")
	if intervalStr == "" || intervalStr == "0" {
		return 0
	}

	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Printf("Warning: Invalid SYNC_INTERVAL '%s', running once. Use format: 1h, 30m, 1h30m", intervalStr)
		return 0
	}

	if duration < time.Minute {
		log.Printf("Warning: SYNC_INTERVAL too short (%v), minimum is 1m", duration)
		return time.Minute
	}

	return duration
}

func runOnce(ctx context.Context, cfg *config.Config) {
	syncer, err := sync.NewSyncer(cfg)
	if err != nil {
		log.Fatalf("Failed to create syncer: %v", err)
	}

	stats, err := syncer.Run(ctx)
	if err != nil {
		log.Fatalf("Synchronization failed: %v", err)
	}

	if stats.Errors > 0 {
		log.Printf("Completed with %d errors", stats.Errors)
		os.Exit(1)
	}

	log.Println("Synchronization completed successfully")
}

func runPeriodic(ctx context.Context, cfg *config.Config, interval time.Duration) {
	log.Printf("Running in daemon mode, sync every %v", interval)

	// Primera ejecución inmediata
	runSync(ctx, cfg)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown requested, stopping periodic sync")
			return
		case <-ticker.C:
			runSync(ctx, cfg)
		}
	}
}

func runSync(ctx context.Context, cfg *config.Config) {
	log.Println("\n========== Starting sync cycle ==========")

	syncer, err := sync.NewSyncer(cfg)
	if err != nil {
		log.Printf("Failed to create syncer: %v", err)
		return
	}

	stats, err := syncer.Run(ctx)
	if err != nil {
		log.Printf("Synchronization failed: %v", err)
		return
	}

	if stats.Errors > 0 {
		log.Printf("Sync cycle completed with %d errors", stats.Errors)
	} else {
		log.Println("Sync cycle completed successfully")
	}
}

func printConfigSummary(cfg *config.Config, interval time.Duration) {
	log.Println("\nConfiguration Summary:")
	log.Println("----------------------")
	log.Printf("LDAP Host:         %s:%d", cfg.LDAP.Host, cfg.LDAP.Port)
	log.Printf("LDAP Base DN:      %s", cfg.LDAP.BaseDN)
	log.Printf("LDAP TLS:          %v", cfg.LDAP.UseTLS)
	log.Printf("Google Domain:     %s", cfg.Google.Domain)
	log.Printf("Google Admin:      %s", cfg.Google.AdminEmail)
	log.Println("--- Sync Options ---")
	log.Printf("Dry Run:           %v", cfg.Sync.DryRun)
	log.Printf("Sync Users:        %v", cfg.Sync.SyncUsers)
	log.Printf("Sync Groups:       %v", cfg.Sync.SyncGroups)
	log.Printf("Sync OrgUnits:     %v", cfg.Sync.SyncOrgUnits)
	log.Printf("Suspend Missing:   %v", cfg.Sync.SuspendMissingUsers)
	log.Printf("Default OU:        %s", cfg.Sync.DefaultOrgUnit)
	if interval > 0 {
		log.Printf("Sync Interval:     %v", interval)
	} else {
		log.Printf("Sync Interval:     once (no repeat)")
	}
	log.Println("----------------------")
}
