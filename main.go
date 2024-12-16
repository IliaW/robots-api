package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/IliaW/robots-api/config"
	docs "github.com/IliaW/robots-api/docs"
	"github.com/IliaW/robots-api/handler"
	cacheClient "github.com/IliaW/robots-api/internal/cache"
	"github.com/IliaW/robots-api/internal/persistence"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/lmittmann/tint"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	cfg        *config.Config
	log        *slog.Logger
	cache      cacheClient.CachedClient
	db         *sql.DB
	ruleRepo   persistence.RuleStorage
	httpClient *http.Client
)

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg = config.MustLoad()
	log = setupLogger()
	db = setupDatabase()
	defer closeDatabase()
	ruleRepo = persistence.NewRuleRepository(db, log)
	cache = cacheClient.NewMemcachedClient(cfg.CacheSettings, log)
	defer cache.Close()
	httpClient = setupHttpClient()
	log.Info("starting application on port "+cfg.Port, slog.String("env", cfg.Env))

	go func() {
		port := fmt.Sprintf(":%v", cfg.Port)
		if err := httpServer().Run(port); err != nil {
			slog.Error("can't start server", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("stopping server...")
}

func httpServer() *gin.Engine {
	setupGinMod()
	r := gin.New()
	r.UseH2C = true
	r.Use(gin.Recovery())
	r.Use(setCORS())
	r.Use(limitBodySize())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/ping"}}))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	robotsHandler := handler.NewRobotsHandler(cache, ruleRepo, httpClient)

	scrapeAllowed := r.Group(cfg.RobotsUrlPath)
	scrapeAllowed.GET("/scrape-allowed", robotsHandler.GetAllowedScrape)

	customRule := r.Group(cfg.RobotsUrlPath)
	customRule.Use(apiKeyCheck())
	customRule.GET("/custom-rule", robotsHandler.GetCustomRule)
	customRule.POST("/custom-rule", robotsHandler.CreateCustomRule)
	customRule.PUT("/custom-rule", robotsHandler.UpdateCustomRule)
	customRule.DELETE("/custom-rule", robotsHandler.DeleteCustomRule)

	docs.SwaggerInfo.Title = fmt.Sprintf("Robots.txt API (%s)", cfg.ServiceName)
	docs.SwaggerInfo.Description = "This is a simple API to control scrape permissions and create custom rules for specific domains."
	docs.SwaggerInfo.Version = cfg.Version
	docs.SwaggerInfo.BasePath = cfg.RobotsUrlPath
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	r.NoRoute(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound,
			gin.H{"message": fmt.Sprintf("no route found for %s %s", c.Request.Method, c.Request.URL)})
	})

	return r
}

func setCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool { //allow all origins and echoes back the caller domain
			return true
		},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"Content-Type", "Content-Length", "Accept-Encoding", "Authorization", "X-Forwarded-For",
			"X-CSRF-Token", "X-Max"},
		AllowCredentials: true,
		MaxAge:           cfg.CorsMaxAgeHours,
	})
}

func limitBodySize() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBodySize*1024*1024)
	}
}

func apiKeyCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "X-API-Key header is missing"})
			c.Abort()
			return
		}

		apiKeyHash := hashAPIKey(apiKey)
		var isActive bool

		err := db.QueryRow("SELECT is_active FROM assessor_api_key WHERE api_key = ?", apiKeyHash).
			Scan(&isActive)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api-key"})
				c.Abort()
				return
			}
			log.Error("failed to query api key", slog.String("err", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "api-key check failed"})
			c.Abort()
			return
		}

		if !isActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "api-key is not active"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

func setupLogger() *slog.Logger {
	resolvedLogLevel := func() slog.Level {
		envLogLevel := strings.ToLower(cfg.LogLevel)
		switch envLogLevel {
		case "info":
			return slog.LevelInfo
		case "error":
			return slog.LevelError
		default:
			return slog.LevelDebug
		}
	}

	replaceAttrs := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.SourceKey {
			source := a.Value.Any().(*slog.Source)
			source.File = filepath.Base(source.File)
		}
		return a
	}

	var logger *slog.Logger
	if strings.ToLower(cfg.LogType) == "json" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource:   true,
			Level:       resolvedLogLevel(),
			ReplaceAttr: replaceAttrs}))
	} else {
		logger = slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			AddSource:   true,
			Level:       resolvedLogLevel(),
			ReplaceAttr: replaceAttrs,
			NoColor:     false}))
	}

	slog.SetDefault(logger)
	logger.Debug("debug messages are enabled.")

	return logger
}

func setupGinMod() {
	env := strings.ToLower(cfg.Env)
	if env == "dev" || env == "" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}

func setupDatabase() *sql.DB {
	log.Info("connecting to the database...")
	sqlCfg := mysql.Config{
		User:                 cfg.DbSettings.User,
		Passwd:               cfg.DbSettings.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%s", cfg.DbSettings.Host, cfg.DbSettings.Port),
		DBName:               cfg.DbSettings.Name,
		AllowNativePasswords: true,
		ParseTime:            true,
	}
	database, err := sql.Open("mysql", sqlCfg.FormatDSN())
	if err != nil {
		log.Error("failed to establish database connection.", slog.String("err", err.Error()))
		os.Exit(1)
	}
	database.SetConnMaxLifetime(cfg.DbSettings.ConnMaxLifetime)
	database.SetMaxOpenConns(cfg.DbSettings.MaxOpenConns)
	database.SetMaxIdleConns(cfg.DbSettings.MaxIdleConns)

	maxRetry := 6
	for i := 1; i <= maxRetry; i++ {
		log.Info("ping the database.", slog.String("attempt", fmt.Sprintf("%d/%d", i, maxRetry)))
		pingErr := database.Ping()
		if pingErr != nil {
			log.Error("not responding.", slog.String("err", pingErr.Error()))
			if i == maxRetry {
				log.Error("failed to establish database connection.")
				os.Exit(1)
			}
			log.Info(fmt.Sprintf("wait %d seconds", 5*i))
			time.Sleep(time.Duration(5*i) * time.Second)
		} else {
			break
		}
	}
	log.Info("connected to the database!")

	return database
}

func closeDatabase() {
	log.Info("closing database connection.")
	err := db.Close()
	if err != nil {
		log.Error("failed to close database connection.", slog.String("err", err.Error()))
	}
}

func setupHttpClient() *http.Client {
	return &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   cfg.HttpClientSettings.RequestTimeout,
	}
}
