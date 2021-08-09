package controller

import "context"

//go:generate moq -fmt goimports -pkg mock -out ../instance/mock/logstash_instance_mock.go . Instance

type Instance interface {
	Start(ctx context.Context, controller *Controller, workdir string) error
	ConfigReload() error
}
