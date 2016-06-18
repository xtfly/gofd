package server

import (
	"fmt"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits"
)

func getServer(c echo.Context) (*Server, error) {
	s, ok := c.Get(CXT_SERVER).(*Server)
	if !ok {
		return nil, fmt.Errorf("Server errror.")
	}
	return s, nil
}

// POST /api/v1/server/tasks
func CreateTask(c echo.Context) error {
	// retrive body
	t := new(p2p.Task)
	if err := c.Bind(t); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL(), err)
		return err
	}

	// retrive server obj
	s, err := getServer(c)
	if err != nil {
		return err
	}

	// check whether taks is existed
	if _, ok := s.cache.Get(t.Id); ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskExist.String())
	}

	ti := NewTaskInfo(t)
	s.cache.Set(ti.Id, ti, gokits.NoExpiration)

	return nil
}

// DELETE /api/v1/server/tasks/:id
func CancelTask(c echo.Context) error {
	return nil
}

// GET /api/v1/server/tasks/:id
func QueryTask(c echo.Context) error {
	return nil
}
