package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/config"
	appdb "github.com/pacorreia/fifa-wc-2026-card-trading/internal/db"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/handlers"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/services"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/ws"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type limiterStore struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	entries map[string]*visitor
}

func newLimiterStore(perMinute int) *limiterStore {
	return &limiterStore{limit: rate.Every(time.Minute / time.Duration(perMinute)), burst: perMinute, entries: make(map[string]*visitor)}
}

func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.entries[key]; ok {
		entry.lastSeen = time.Now()
		return entry.limiter
	}
	limiter := rate.NewLimiter(s.limit, s.burst)
	s.entries[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
	for key, entry := range s.entries {
		if time.Since(entry.lastSeen) > 10*time.Minute {
			delete(s.entries, key)
		}
	}
	return limiter
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))

	ctx := context.Background()
	store, err := appdb.Connect(ctx, cfg, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.RunMigrations(ctx, filepath.Join("migrations")); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	hub := ws.NewHub(logger, cfg.WSMaxConnectionsPerUser)
	go hub.Run()

	authService := services.NewAuthService(store.DB, tokenManager, logger)
	collectionService := services.NewCollectionService(store.DB, logger, hub)
	tradeService := services.NewTradeService(store.DB, logger, hub)

	authHandler := handlers.NewAuthHandler(authService)
	collectionHandler := handlers.NewCollectionHandler(collectionService)
	stickerHandler := handlers.NewStickerHandler(store.DB)
	tradeHandler := handlers.NewTradeHandler(tradeService)
	userHandler := handlers.NewUserHandler(store.DB, collectionService)
	healthHandler := handlers.NewHealthHandler(store.DB)
	wsHandler := handlers.NewWSHandler(hub, tokenManager, cfg.CORSOrigins, logger)
	notificationHandler := handlers.NewNotificationHandler(store.DB, hub)

	router := mux.NewRouter()
	router.Use(loggingMiddleware(logger))
	router.Use(recoveryMiddleware(logger))
	router.Use(corsMiddleware(cfg.CORSOrigins))

	router.HandleFunc("/healthz", healthHandler.Healthz).Methods(http.MethodGet)
	router.HandleFunc("/readyz", healthHandler.Readyz).Methods(http.MethodGet)
	router.Handle("/ws", wsHandler).Methods(http.MethodGet)

	loginLimiter := newLimiterStore(cfg.LoginRateLimitPerMinute)
	registerLimiter := newLimiterStore(cfg.RegisterRateLimitMinute)
	apiLimiter := newLimiterStore(cfg.APIRateLimitPerMinute)

	router.Handle("/api/auth/register", limitByIP(registerLimiter, http.HandlerFunc(authHandler.Register))).Methods(http.MethodPost)
	router.Handle("/api/auth/login", limitByIP(loginLimiter, http.HandlerFunc(authHandler.Login))).Methods(http.MethodPost)
	router.HandleFunc("/api/auth/refresh", authHandler.Refresh).Methods(http.MethodPost)
	router.HandleFunc("/api/auth/logout", authHandler.Logout).Methods(http.MethodPost)

	router.HandleFunc("/api/stickers", stickerHandler.List).Methods(http.MethodGet)
	router.HandleFunc("/api/stickers/{code}", stickerHandler.Get).Methods(http.MethodGet)

	protected := router.PathPrefix("/api").Subrouter()
	protected.Use(auth.AuthMiddleware(tokenManager))
	protected.Use(limitByUser(apiLimiter))

	protected.HandleFunc("/collections/me", collectionHandler.GetMine).Methods(http.MethodGet)
	protected.HandleFunc("/collections/stickers/{code}", collectionHandler.UpdateItem).Methods(http.MethodPut)
	protected.HandleFunc("/collections/stats", collectionHandler.GetStats).Methods(http.MethodGet)
	protected.HandleFunc("/collections/missing", collectionHandler.GetMissing).Methods(http.MethodGet)
	protected.HandleFunc("/collections/duplicates", collectionHandler.GetDuplicates).Methods(http.MethodGet)

	protected.HandleFunc("/trades/matches", tradeHandler.GetMatches).Methods(http.MethodGet)
	protected.HandleFunc("/trades/matches/{userId}", tradeHandler.GetMatchWithUser).Methods(http.MethodGet)
	protected.HandleFunc("/trades", tradeHandler.CreateTrade).Methods(http.MethodPost)
	protected.HandleFunc("/trades", tradeHandler.ListTrades).Methods(http.MethodGet)
	protected.HandleFunc("/trades/{id}/respond", tradeHandler.RespondTrade).Methods(http.MethodPut)

	protected.HandleFunc("/users", userHandler.ListUsers).Methods(http.MethodGet)
	protected.HandleFunc("/users/{id}/collection", userHandler.GetCollection).Methods(http.MethodGet)
	protected.HandleFunc("/notifications", notificationHandler.List).Methods(http.MethodGet)
	protected.HandleFunc("/notifications/{id}/read", notificationHandler.MarkRead).Methods(http.MethodPut)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("starting api server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func loggingMiddleware(logger *slog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("http request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}

func recoveryMiddleware(logger *slog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered", "error", recovered)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(allowedOrigins []string) mux.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func limitByIP(store *limiterStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !store.get(host).Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func limitByUser(store *limiterStore) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			key := r.RemoteAddr
			if ok {
				key = fmt.Sprintf("user:%d", claims.UserID)
			}
			if !store.get(key).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
