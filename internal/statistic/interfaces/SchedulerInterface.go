package interfaces

type SchedulerInterface interface {
	Init()
	Stop()
	Close()
	Restore() error
	Persist() error
}
