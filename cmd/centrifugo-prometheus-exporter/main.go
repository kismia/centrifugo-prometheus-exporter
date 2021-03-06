package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kismia/centrifugo-prometheus-exporter/internal/collector"
	"github.com/kismia/centrifugo-prometheus-exporter/internal/pkg/centrifugo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

type options struct {
	centrifugoEndpoint string
	centrifugoSecret   string
	centrifugoNodeName string
	address            string
	metricsPath        string
}

func main() {
	options := &options{}
	command := &cobra.Command{
		Use: "centrifugo-prometheus-exporter",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.centrifugoSecret == "" {
				options.centrifugoSecret = os.Getenv("CENTRIFUGO_SECRET")
			}

			if options.centrifugoNodeName == "" {
				options.centrifugoNodeName = os.Getenv("CENTRIFUGO_NODE_NAME")
			}

			return options.Run()
		},
	}

	command.Flags().StringVar(&options.centrifugoEndpoint, "centrifugo-endpoint", "http://localhost:8000", "centrifugo server endpoint")
	command.Flags().StringVar(&options.centrifugoSecret, "centrifugo-secret", "", "centrifugo api key (or use env CENTRIFUGO_SECRET)")
	command.Flags().StringVar(&options.centrifugoNodeName, "centrifugo-node-name", "", "target centrifugo node name (or use env CENTRIFUGO_NODE_NAME)")
	command.Flags().StringVar(&options.address, "address", ":9564", "address to listen on for web interface and telemetry")
	command.Flags().StringVar(&options.metricsPath, "metrics-path", "/metrics", "path under which to expose metrics")

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func (o *options) Run() error {
	registry := prometheus.NewRegistry()

	client := centrifugo.NewClient(o.centrifugoEndpoint, o.centrifugoSecret)

	exporter := collector.NewExporter(client, o.centrifugoNodeName)

	registry.MustRegister(exporter)

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Centrifugo Exporter</title></head>
             <body>
             <h1>Centrifugo Exporter</h1>
             <p><a href='` + o.metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	httpServer := &http.Server{
		Addr:    o.address,
		Handler: mux,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	defer cancel()

	return httpServer.Shutdown(ctx)
}
