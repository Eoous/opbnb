package service

import (
	"context"
	"fmt"
	"op-ufm/pkg/config"
	"op-ufm/pkg/metrics"
	"op-ufm/pkg/provider"

	"github.com/ethereum/go-ethereum/log"
)

type Service struct {
	Config    *config.Config
	Healthz   *HealthzServer
	Metrics   *MetricsServer
	Providers map[string]*provider.Provider
}

func New(cfg *config.Config) *Service {
	s := &Service{
		Config:    cfg,
		Healthz:   &HealthzServer{},
		Metrics:   &MetricsServer{},
		Providers: make(map[string]*provider.Provider, len(cfg.Providers)),
	}
	return s
}

func (s *Service) Start(ctx context.Context) {
	log.Info("service starting")
	if s.Config.Healthz.Enabled {
		addr := fmt.Sprintf("%s:%d", s.Config.Healthz.Host, s.Config.Healthz.Port)
		log.Info("starting healthz server", "addr", addr)
		go func() {
			if err := s.Healthz.Start(ctx, s.Config.Healthz.Host, s.Config.Healthz.Port); err != nil {
				log.Error("error starting healthz server", "err", err)
			}
		}()
	}

	metrics.Debug = s.Config.Metrics.Debug
	if s.Config.Metrics.Enabled {
		addr := fmt.Sprintf("%s:%d", s.Config.Metrics.Host, s.Config.Metrics.Port)
		log.Info("starting metrics server", "addr", addr)
		go func() {
			if err := s.Metrics.Start(ctx, addr); err != nil {
				log.Error("error starting metrics server", "err", err)
			}
		}()
	}

	// map networks to its providers
	networks := make(map[string][]string)
	for name, providerConfig := range s.Config.Providers {
		networks[providerConfig.Network] = append(networks[providerConfig.Network], name)
	}

	txpool := &provider.TransactionPool{}
	for name, providers := range networks {
		if len(providers) == 1 {
			log.Warn("can't measure first seen for network, please another provider", "network", name)
		}
		(*txpool)[name] = &provider.NetworkTransactionPool{}
		(*txpool)[name].Transactions = make(map[string]*provider.TransactionState)
		// set expected number of providers for this network
		// -1 since we don't wait for acking from the same provider
		(*txpool)[name].Expected = len(providers) - 1
	}

	for name, providerConfig := range s.Config.Providers {
		s.Providers[name] = provider.New(name,
			providerConfig,
			&s.Config.Signer,
			s.Config.Wallets[providerConfig.Wallet],
			(*txpool)[providerConfig.Network])
		s.Providers[name].Start(ctx)
		log.Info("provider started", "provider", name)
	}

	log.Info("service started")
}

func (s *Service) Shutdown() {
	log.Info("service shutting down")
	if s.Config.Healthz.Enabled {
		s.Healthz.Shutdown()
		log.Info("healthz stopped")
	}
	if s.Config.Metrics.Enabled {
		s.Metrics.Shutdown()
		log.Info("metrics stopped")
	}
	for name, provider := range s.Providers {
		provider.Shutdown()
		log.Info("provider stopped", "provider", name)
	}
	log.Info("service stopped")
}
