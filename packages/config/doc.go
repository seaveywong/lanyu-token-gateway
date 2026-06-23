// Package config provides configuration loading, validation, and defaults
// for the Lanyu Token Gateway services.
//
// Configuration is loaded from YAML files and covers server, database, Redis,
// observability, authentication, routing, and billing settings.
//
// Usage:
//
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal(err)
//	}
package config
