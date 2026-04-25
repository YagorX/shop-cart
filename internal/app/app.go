package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcapp "github.com/YagorX/shop-cart-service/internal/app/grpcapp"
	httpapp "github.com/YagorX/shop-cart-service/internal/app/httpapp"
	"github.com/YagorX/shop-cart-service/internal/config"
	"github.com/YagorX/shop-cart-service/internal/lock"
	"github.com/YagorX/shop-cart-service/internal/observability"
	"github.com/YagorX/shop-cart-service/internal/repository/mongodb"
	cartsvc "github.com/YagorX/shop-cart-service/internal/service/cart"
	grpcHandlers "github.com/YagorX/shop-cart-service/internal/transport/grpc/v1/handlers"
	httpv1 "github.com/YagorX/shop-cart-service/internal/transport/http/v1"
	cartv1 "github.com/YagorX/shop-contracts/gen/go/proto/cart/v1"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type App struct {
	logger *slog.Logger

	httpApp      *httpapp.App
	grpcApp      *grpcapp.App
	healthServer *health.Server
	errCh        chan error

	db          *mongo.Database
	redisClient *goredis.Client

	shutdownTracing func(context.Context) error
}

type readinessChecker struct {
	db    *mongo.Client
	redis *goredis.Client
}

func (c *readinessChecker) Check(ctx context.Context) error {
	if c == nil {
		return errors.New("readiness checker is nil")
	}
	if c.db == nil {
		return errors.New("mongodb is not initialized")
	}
	if c.redis == nil {
		return errors.New("redis is not initialized")
	}
	if err := c.db.Ping(ctx, nil); err != nil {
		return fmt.Errorf("mongodb not ready: %w", err)
	}
	if err := c.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis not ready: %w", err)
	}
	return nil
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	runtimeLogger := observability.NewLogger(observability.LoggerOptions{
		Service: cfg.ServiceName,
		Env:     cfg.Env,
		Version: cfg.Version,
		Level:   cfg.LogLevel,
	})
	observability.SetDefaultLogger(runtimeLogger.Logger)

	shutdownTracing, err := observability.InitTracing(
		ctx,
		cfg.ServiceName,
		cfg.Version,
		cfg.Env,
		cfg.OTLP.Endpoint,
	)
	if err != nil {
		runtimeLogger.Logger.Warn("tracing is disabled", slog.String("error", err.Error()))
		shutdownTracing = nil
	}

	stopTracing := func() {
		if shutdownTracing != nil {
			_ = shutdownTracing(context.Background())
		}
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	pingRedisCtx, redisPingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer redisPingCancel()

	if err := redisClient.Ping(pingRedisCtx).Err(); err != nil {
		_ = redisClient.Close()
		stopTracing()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	mutex := lock.NewDistributedLock(redisClient, cfg.Lock.TTL, cfg.Lock.RetryInterval, cfg.Lock.MaxRetries)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		stopTracing()
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := mongoClient.Ping(pingCtx, nil); err != nil {
		_ = mongoClient.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	db := mongoClient.Database(cfg.MongoDB.DBName)
	repo := mongodb.NewCartRepository(db, mutex)

	// создаём индексы при старте
	if err := repo.EnsureIndexes(ctx); err != nil {
		_ = mongoClient.Disconnect(ctx)
		stopTracing()
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}

	cartService := cartsvc.NewCartService(repo, runtimeLogger.Logger)

	grpcHandler, err := grpcHandlers.NewHandler(cartService)
	if err != nil {
		_ = mongoClient.Disconnect(ctx)
		stopTracing()
		return nil, fmt.Errorf("create grpc handler: %w", err)
	}

	serverOpts := []grpc.ServerOption{
		grpc.StatsHandler(observability.GRPCServerStatsHandler()),
	}
	if tlsOpt, err := buildServerTLS(cfg.TLS); err != nil {
		_ = mongoClient.Disconnect(ctx)
		stopTracing()
		return nil, fmt.Errorf("build grpc tls: %w", err)
	} else if tlsOpt != nil {
		serverOpts = append(serverOpts, tlsOpt)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	cartv1.RegisterCartServiceServer(grpcServer, grpcHandler)
	grpcHealth := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, grpcHealth)
	grpcHealth.SetServingStatus("proto.cart.v1.CartService", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	httpRouter := httpv1.NewRouter(httpv1.RouterDeps{
		LogLevelController: runtimeLogger,
		ReadinessChecker: &readinessChecker{
			db:    mongoClient,
			redis: redisClient,
		},
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr(),
		Handler:           httpRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	grpcRuntime, err := grpcapp.New(runtimeLogger.Logger, grpcServer, cfg.GRPCAddr())
	if err != nil {
		_ = mongoClient.Disconnect(ctx)
		stopTracing()
		return nil, fmt.Errorf("create grpc app: %w", err)
	}

	httpRuntime, err := httpapp.New(runtimeLogger.Logger, httpServer)
	if err != nil {
		_ = mongoClient.Disconnect(ctx)
		stopTracing()
		return nil, fmt.Errorf("create http app: %w", err)
	}

	return &App{
		logger:          runtimeLogger.Logger,
		httpApp:         httpRuntime,
		grpcApp:         grpcRuntime,
		db:              db,
		redisClient:     redisClient,
		shutdownTracing: shutdownTracing,
		healthServer:    grpcHealth,
		errCh:           make(chan error, 2),
	}, nil
}

func (a *App) Run() error {
	if a == nil {
		return errors.New("app is nil")
	}

	go func() {
		if err := a.grpcApp.Run(); err != nil {
			a.errCh <- fmt.Errorf("grpc app failed: %w", err)
		}
	}()

	go func() {
		if err := a.httpApp.Run(); err != nil {
			a.errCh <- fmt.Errorf("http app failed: %w", err)
		}
	}()

	a.logger.Info("cart service bootstrap completed",
		slog.String("grpc_addr", a.grpcApp.Addr()),
		slog.String("http_addr", a.httpApp.Addr()),
		slog.String("repository_backend", "mongodb"),
	)

	return nil
}

func (a *App) Errors() <-chan error {
	if a == nil {
		return nil
	}
	return a.errCh
}

// buildServerTLS строит серверные TLS credentials для gRPC.
// Если TLS выключен — возвращает nil, nil (сервер поднимается без шифрования).
// Если включён — загружает сертификат сервера и CA для проверки клиентских сертификатов (mTLS).
func buildServerTLS(cfg config.TLSConfig) (grpc.ServerOption, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Загружаем сертификат и ключ самого сервера
	serverCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key: %w", err)
	}

	// Загружаем CA — им будем проверять сертификаты входящих клиентов
	caPEM, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read ca file: %w", err)
	}
	clientCA := x509.NewCertPool()
	if !clientCA.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("failed to parse ca certificate")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // mTLS: клиент обязан предъявить сертификат
		ClientCAs:    clientCA,
		MinVersion:   tls.VersionTLS12,
	}

	return grpc.Creds(credentials.NewTLS(tlsCfg)), nil
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	var shutdownErr error

	if a.grpcApp != nil {
		a.healthServer.SetServingStatus("proto.cart.v1.CartService", healthpb.HealthCheckResponse_NOT_SERVING)
		a.grpcApp.Stop()
	}

	if a.httpApp != nil {
		if err := a.httpApp.Stop(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("stop http app: %w", err))
		}
	}

	if a.db != nil {
		if err := a.db.Client().Disconnect(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("disconnect mongo client: %w", err))
		}
	}

	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close redis: %w", err))
		}
	}

	if a.shutdownTracing != nil {
		if err := a.shutdownTracing(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown tracing: %w", err))
		}
	}

	a.logger.Info("cart service stopped")

	return shutdownErr
}
