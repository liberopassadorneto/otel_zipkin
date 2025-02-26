package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type CEPRequest struct {
	CEP string `json:"cep"`
}

type ViaCEPResponse struct {
	Localidade string          `json:"localidade"`
	Erro       json.RawMessage `json:"erro,omitempty"`
}

type WeatherResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type Response struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

var (
	ErrInvalidCEP  = errors.New("invalid cep")
	ErrCEPNotFound = errors.New("cep not found")
)

func initTracer() func() {
	zipkinURL := os.Getenv("ZIPKIN_ENDPOINT")
	if zipkinURL == "" {
		zipkinURL = "http://localhost:9411/api/v2/spans"
	}
	exporter, err := zipkin.New(zipkinURL)
	if err != nil {
		log.Fatalf("failed to create Zipkin exporter: %v", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("error shutting down tracer provider: %v", err)
		}
	}
}

func main() {
	shutdown := initTracer()
	defer shutdown()

	http.HandleFunc("/cep", cepHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("ServiceB listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func cepHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("serviceB")
	ctx, span := tracer.Start(ctx, "cepHandler")
	defer span.End()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CEPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	city, err := getCityByCEP(ctx, req.CEP)
	if err != nil {
		if errors.Is(err, ErrInvalidCEP) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]string{"message": "invalid zipcode"})
			return
		} else if errors.Is(err, ErrCEPNotFound) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "can not find zipcode"})
			return
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	tempC, err := getTemperature(ctx, city)
	if err != nil {
		http.Error(w, "error fetching temperature", http.StatusInternalServerError)
		return
	}
	resp := Response{
		City:  city,
		TempC: tempC,
		TempF: tempC*1.8 + 32,
		TempK: tempC + 273,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getCityByCEP(ctx context.Context, cep string) (string, error) {
	tracer := otel.Tracer("serviceB")
	ctx, span := tracer.Start(ctx, "getCityByCEP")
	defer span.End()

	viaCEPURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	req, err := http.NewRequestWithContext(ctx, "GET", viaCEPURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ErrCEPNotFound
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var viaCEPResp ViaCEPResponse
	if err := json.Unmarshal(body, &viaCEPResp); err != nil {
		return "", err
	}
	if len(viaCEPResp.Erro) > 0 || viaCEPResp.Localidade == "" {
		return "", ErrCEPNotFound
	}
	return viaCEPResp.Localidade, nil
}

func getTemperature(ctx context.Context, city string) (float64, error) {
	encodedCity := url.QueryEscape(city)
	tracer := otel.Tracer("serviceB")
	ctx, span := tracer.Start(ctx, "getTemperature")
	span.SetAttributes(attribute.String("city", city))
	defer span.End()

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("WEATHER_API_KEY not set")
	}
	weatherURL := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, encodedCity)
	req, err := http.NewRequestWithContext(ctx, "GET", weatherURL, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("weather api returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var weatherResp WeatherResponse
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		return 0, err
	}
	return weatherResp.Current.TempC, nil
}
