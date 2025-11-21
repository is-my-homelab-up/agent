package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	cloudAddress  string
	cloudEndpoint string
	cloudUrl      string
	cloudId       string
	cloudApiKey   string
	interval      time.Duration

	serverAddress string
}

type HealthResponse struct {
	Repeater string `json:"repeater"`
	Id       string `json:"id"`
}

func getEnvVariable(name string, defaultValue string) (string, error) {
	value := os.Getenv(name)
	if len(value) == 0 {
		if len(defaultValue) == 0 {
			return "", fmt.Errorf("missing environment variable '%s'", name)
		}

		return defaultValue, nil
	}

	return value, nil
}

func parseConfig() (config, error) {
	cloudAddress, err := getEnvVariable("CLOUD_ADDRESS", "")
	if err != nil {
		return config{}, err
	}

	cloudEndpoint, err := getEnvVariable("CLOUD_ENDPOINT", "v1/announce")
	if err != nil {
		return config{}, err
	}

	cloudId, err := getEnvVariable("CLOUD_ID", "")
	if err != nil {
		return config{}, err
	}

	cloudApiKey, err := getEnvVariable("CLOUD_APIKEY", "")
	if err != nil {
		return config{}, err
	}

	rawIntervalSeconds, err := getEnvVariable("INTERVAL", "10")
	if err != nil {
		return config{}, err
	}

	intervalSeconds, err := strconv.ParseInt(rawIntervalSeconds, 10, 64)
	if err != nil {
		return config{}, fmt.Errorf("failed to parse interval '%s' as integer: %w", rawIntervalSeconds, err)
	}

	interval := time.Second * time.Duration(intervalSeconds)

	serverAddress, err := getEnvVariable("SERVER_ADDRESS", "0.0.0.0:8080")
	if err != nil {
		return config{}, err
	}

	return config{
		cloudAddress:  cloudAddress,
		cloudEndpoint: cloudEndpoint,
		cloudUrl:      fmt.Sprintf("%s/%s", cloudAddress, cloudEndpoint),
		cloudId:       cloudId,
		cloudApiKey:   cloudApiKey,
		interval:      interval,
		serverAddress: serverAddress,
	}, nil
}

func parseFormValue(name string, logger *slog.Logger, w http.ResponseWriter, r *http.Request) (string, bool) {
	value := r.PostFormValue(name)
	if len(value) != 0 {
		return value, true
	}

	logger.Error("request is missing form value", "value", name)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, "missing form value '%s'", name)
	return "", false
}

func runServer(logger *slog.Logger, config *config, done <-chan bool) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("handling request")

		if r.Method != http.MethodPost {
			logger.Error("request is not a post request", "method", r.Method)
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		cloudId, didParse := parseFormValue("cloud_id", logger, w, r)
		if !didParse {
			return
		}

		idMatches := strings.EqualFold(cloudId, config.cloudId)
		if !idMatches {
			logger.Error("request has wrong id", "request_id", cloudId)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("id doesn't match"))
			return
		}

		repeater, didParse := parseFormValue("repeater", logger, w, r)
		if !didParse {
			return
		}

		response := &HealthResponse{
			Repeater: repeater,
			Id:       config.cloudId,
		}

		jsonBytes, err := json.Marshal(response)
		if err != nil {
			logger.Error("error marshaling JSON data", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Debug("responding to valid request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
	})

	logger.Info("starting health server", "address", config.serverAddress)
	server := &http.Server{
		Handler: mux,
		Addr:    config.serverAddress,
	}

	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("server closed")
		} else if err != nil {
			logger.Error("error listening and serving", "err", err)
		}
	}()

	<-done
	logger.Info("shutting down server")
	err := server.Shutdown(context.Background())
	if err != nil {
		logger.Error("error while shutting down server", "err", err)
	}
}

func notifyCloud(logger *slog.Logger, config *config) {
	logger.Debug("notifying the cloud")

	v := url.Values{}
	v.Set("id", config.cloudId)
	v.Set("random", rand.Text())

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, config.cloudUrl, strings.NewReader(v.Encode()))
	if err != nil {
		logger.Error("error creating request for cloud", "error", err)
		return
	}

	request.Header.Add("X-API-KEY", config.cloudApiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("error sending request to cloud", "error", err)
		return
	}

	if response.StatusCode == http.StatusOK {
		return
	}

	logger.Error("unexpected status code", "status", response.Status)
}

func runTicker(logger *slog.Logger, config *config, done <-chan bool) {
	logger.Info("setting up ticker", "interval_ms", config.interval)
	ticker := time.NewTicker(config.interval)

	for {
		select {
		case <-done:
			logger.Info("logger stopped")
			return
		case <-ticker.C:
			notifyCloud(logger, config)
		}
	}
}

func convertLogLevel(rawLogLevel string) slog.Level {
	if rawLogLevel == "DEBUG" {
		return slog.LevelDebug
	}

	if rawLogLevel == "WARN" {
		return slog.LevelWarn
	}

	if rawLogLevel == "ERROR" {
		return slog.LevelError
	}

	return slog.LevelInfo
}

func main() {
	rawLogLevel, _ := getEnvVariable("LOG_LEVEL", "INFO")
	jsonHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: convertLogLevel(rawLogLevel),
	})

	logger := slog.New(jsonHandler)

	config, err := parseConfig()
	if err != nil {
		logger.Error("error parsing config", "err", err)
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		sig := <-signals
		logger.Info("received signal", "signal", sig)
		done <- true
	}()

	go runServer(logger, &config, done)
	go runTicker(logger, &config, done)
	<-done
}
