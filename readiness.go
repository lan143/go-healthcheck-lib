package go_healthcheck_lib

type ReadinessProbe interface {
	IsReady() bool
}
