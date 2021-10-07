package tasks

import (
	"context"

	"github.com/teamwork/log"
)

type TaskService struct {
	UnimplementedTasksServer
}

func NewTaskService() *TaskService {
	return &TaskService{}
}

func (s *TaskService) Create(c context.Context, t *TaskRequest) (*TaskResponse, error) {
	log.Infof("Recieved new task %s", t.Title)

	resp := TaskResponse{
		Id: 1,
	}
	return &resp, nil
}
