package interfaces

type SchedulerInterface interface {
	Init()
	Stop()
	Restore() error
	Persist() error
}
