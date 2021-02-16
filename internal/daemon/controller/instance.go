package controller

//go:generate moq -fmt goimports -pkg mock -out ../instance/mock/logstash_instance_mock.go . Instance

type Instance interface {
	Start(controller *Controller, workdir string) error
	Shutdown()
	ConfigReload() error
}
