package go_healthcheck_lib

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthCheck struct {
	server *http.Server

	wg      *sync.WaitGroup
	probes  []ReadinessProbe
	readyMu sync.RWMutex
	isReady bool
}

func (healthCheck *HealthCheck) Init(listenAddr string, wg *sync.WaitGroup) {
	log.Printf("health-check: init")

	healthCheck.wg = wg
	healthCheck.server = &http.Server{
		Addr: listenAddr,
	}

	http.HandleFunc("/health-check", healthCheck.healthCheck)
	http.HandleFunc("/ready-check", healthCheck.readyCheck)

	log.Printf("health-check.init: complete")
}

func (healthCheck *HealthCheck) AddReadinessProbe(probe ReadinessProbe) {
	healthCheck.probes = append(healthCheck.probes, probe)
}

func (healthCheck *HealthCheck) Run(ctx context.Context) {
	log.Printf("health-check: run")
	httpShutdownCh := make(chan struct{})

	go healthCheck.runReadinessProbes()

	go func() {
		<-ctx.Done()

		log.Println("health-check.shutdown: init")

		graceCtx, graceCancel := context.WithTimeout(ctx, 1*time.Second)
		defer graceCancel()

		if err := healthCheck.server.Shutdown(graceCtx); err != nil {
			log.Println(err)
		}

		httpShutdownCh <- struct{}{}
	}()

	go func() {
		healthCheck.wg.Add(1)
		defer healthCheck.wg.Done()

		err := healthCheck.server.ListenAndServe()
		<-httpShutdownCh

		if err != http.ErrServerClosed {
			panic(err)
		}

		log.Println("health-check.shutdown: complete")
	}()
}

func (healthCheck *HealthCheck) IsReady() bool {
	return healthCheck.isReady
}

func (healthCheck *HealthCheck) runReadinessProbes() {
	for {
		isReady := true
		for _, probe := range healthCheck.probes {
			if !probe.IsReady() {
				isReady = false
				break
			}
		}

		healthCheck.readyMu.Lock()
		healthCheck.isReady = isReady
		healthCheck.readyMu.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func (healthCheck *HealthCheck) healthCheck(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
}

func (healthCheck *HealthCheck) readyCheck(w http.ResponseWriter, req *http.Request) {
	healthCheck.readyMu.RLock()
	isReady := healthCheck.isReady
	healthCheck.readyMu.RUnlock()

	if isReady {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(503)
	}
}

func NewHealthCheck() *HealthCheck {
	healthCheck := &HealthCheck{}

	return healthCheck
}
