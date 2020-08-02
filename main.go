package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	terminalTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "collector",
		Name:      "terminal_temperature_fahrenheit_add",
		Help:      "Terminal Sensor Temperature Value Fahrenheit",
	})

	terminalTempSet = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "collector",
		Name:      "terminal_temperature_fahrenheit_set",
		Help:      "Terminal Sensor Temperature Value Fahrenheit",
	})
	valuesTempCollected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "terminal_temperature_total",
			Help: "Number of temperature collected.",
		},
		[]string{"terminal", "sensor"},
	)
)

// CollectorService provides operations on collect.
type CollectorService interface {
	Temperature(context.Context, float32) float32
}

type collectorService struct{}

func (collectorService) Temperature(_ context.Context, v valueRequest) float64 {
	terminalTemp.Add(v.Value)
	terminalTempSet.Add(v.Value)
	valuesTempCollected.With(prometheus.Labels{"terminal": v.Terminal, "sensor": v.Sensor}).Inc()
	return v.Value
}

type valueRequest struct {
	Timestamp int64   `json:"timestamp"`
	Terminal  string  `json:"terminal"`
	Sensor    string  `json:"sensor"`
	Value     float64 `json:"value"`
}

func (v valueRequest) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.Timestamp, validation.Required),
		validation.Field(&v.Terminal, validation.Required),
		validation.Field(&v.Sensor, validation.In("temperature")),
		validation.Field(&v.Value, validation.Required),
	)
}

type valueResponse struct {
	Message string  `json:"message,omitempty"`
	Value   float64 `json:"value,omitempty"`
	Error   string  `json:"error,omitempty"` // errors don't define JSON marshaling
}

func makeValueEndpoint(svc collectorService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(valueRequest)
		log.WithFields(log.Fields{
			"timestamp": req.Timestamp,
			"terminal":  req.Terminal,
			"sensor":    req.Sensor,
			"value":     req.Value,
		}).Info("Data Collected")
		err := req.Validate()
		if err == nil {
			if req.Sensor == "temperature" {
				v := svc.Temperature(ctx, req)
				return valueResponse{Message: "received", Value: v}, nil
			}
		}
		log.Errorf("Error: %s", err.Error())
		return valueResponse{Error: err.Error()}, err
	}
}

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(terminalTemp)
	prometheus.MustRegister(terminalTempSet)
	prometheus.MustRegister(valuesTempCollected)
}

func main() {
	svc := collectorService{}

	valueHTTPHandler := httptransport.NewServer(
		makeValueEndpoint(svc),
		decodeValueHTTPRequest,
		httptransport.EncodeJSONResponse,
	)

	go func() {
		for {
			SendValue()
			time.Sleep(time.Second)
		}
	}()

	http.Handle("/value", valueHTTPHandler)
	http.Handle("/metrics", promhttp.Handler())
	log.Info("API running port: 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func decodeValueHTTPRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request valueRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func SendValue() {
	value := valueRequest{
		Timestamp: time.Now().Unix(),
		Terminal:  "Test Terminal",
		Sensor:    "temperature",
		Value:     rand.Float64(),
	}
	jsonValue, _ := json.Marshal(value)
	fmt.Printf("%+v\n", value)
	u := bytes.NewReader(jsonValue)

	req, err := http.NewRequest("POST", "http://localhost:8080/value", u)
	if err != nil {
		fmt.Println("Error is req: ", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// create a Client
	client := &http.Client{}
	// Do sends an HTTP request and
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("error in send req: ", err.Error())
	}
	defer resp.Body.Close()
}