package app

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/mcp"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/secrets"
)

// App holds all shared resources for the application lifetime.
type App struct {
	Config    config.Config
	DB        *gorm.DB
	Users     *users.Service
	Tokens    *tokens.Service
	Firewall  *firewall.Service
	Groups    *groups.Service
	Providers *provider.Service
	MCP       *mcp.Service
	Proxy     *proxy.Service
	OIDC      *auth.OIDCService
	Validator *auth.Validator
	Sessions  *auth.SessionStore
	Redis     *redisstore.Store
}

// New initializes the database, services, and auth components and returns a ready App.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	slog.Info("initializing postgres connection")
	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	slog.Info("postgres connection ready")

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.JWTSecret)
	userService.SetTokenRevoker(tokenService)
	firewallService := firewall.NewService(db)
	groupService := groups.NewService(db)
	secretCipher, err := secrets.NewCipher(cfg.SecretsKey)
	if err != nil {
		return nil, err
	}
	providerService := provider.NewService(db, secretCipher)
	mcpService := mcp.NewService(db, secretCipher)
	proxyService := proxy.NewService(db)
	slog.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}
	slog.Info("redis connection ready")
	userService.SetNotifier(redisStore)
	tokenService.SetNotifier(redisStore)
	firewallService.SetNotifier(redisStore)
	groupService.SetNotifier(redisStore)
	providerService.SetNotifier(redisStore)
	mcpService.SetNotifier(redisStore)

	var validator *auth.Validator
	var sessionStore *auth.SessionStore
	var oidcService *auth.OIDCService
	if cfg.KeycloakIssuerURL != "" || cfg.KeycloakJWKSURL != "" {
		keycloakHTTPClient, err := auth.NewKeycloakHTTPClient(cfg.KeycloakCACertPath)
		if err != nil {
			return nil, err
		}
		if keycloakHTTPClient != nil {
			slog.Info("loaded Keycloak CA certificate", "path", cfg.KeycloakCACertPath)
		}

		slog.Info("initializing token validator", "issuer", cfg.KeycloakIssuerURL)
		validator, err = auth.NewValidator(ctx, cfg.KeycloakIssuerURL, cfg.KeycloakJWKSURL, auth.WithValidatorHTTPClient(keycloakHTTPClient))
		if err != nil {
			return nil, err
		}
		slog.Info("token validator ready")

		slog.Info("initializing session store")
		if redisStore.Enabled() {
			sessionStore = auth.NewRedisSessionStore(userService, cfg.SessionTTL, redisStore)
		} else {
			sessionStore = auth.NewSessionStore(userService, cfg.SessionTTL)
		}
		slog.Info("session store ready")

		slog.Info("initializing OIDC service")
		oidcService, err = auth.NewOIDCService(ctx, cfg, validator, sessionStore, userService, keycloakHTTPClient)
		if err != nil {
			return nil, err
		}
		slog.Info("OIDC service ready")
	} else {
		slog.Info("skipping OIDC initialization")
	}

	return &App{
		Config:    cfg,
		DB:        db,
		Users:     userService,
		Tokens:    tokenService,
		Firewall:  firewallService,
		Groups:    groupService,
		Providers: providerService,
		MCP:       mcpService,
		Proxy:     proxyService,
		OIDC:      oidcService,
		Validator: validator,
		Sessions:  sessionStore,
		Redis:     redisStore,
	}, nil
}

// StartBackgroundJobs launches long-running goroutines tied to the context lifetime.
func (a *App) StartBackgroundJobs(ctx context.Context) {
	slog.Info("starting token cleanup goroutine", "interval", a.Config.TokenCleanupInterval)
	a.Tokens.StartCleanup(ctx, a.Config.TokenCleanupInterval)
	slog.Info("starting user access expiration goroutine", "interval", a.Config.UserAccessExpirationInterval)
	a.Users.StartAccessExpiration(ctx, a.Config.UserAccessExpirationInterval)
}
