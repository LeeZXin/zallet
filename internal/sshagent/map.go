package sshagent

import (
	"github.com/LeeZXin/zallet/internal/action"
	"os/exec"
	"sync"
)

type graphMap struct {
	sync.Mutex
	container map[string]*action.Graph
}

func newGraphMap() *graphMap {
	return &graphMap{
		container: make(map[string]*action.Graph),
	}
}

func (m *graphMap) GetAll() map[string]*action.Graph {
	m.Lock()
	defer m.Unlock()
	ret := make(map[string]*action.Graph, len(m.container))
	for k, v := range m.container {
		ret[k] = v
	}
	return ret
}

func (m *graphMap) PutIfAbsent(id string, graph *action.Graph) bool {
	m.Lock()
	defer m.Unlock()
	_, b := m.container[id]
	if b {
		return false
	}
	m.container[id] = graph
	return true
}

func (m *graphMap) GetById(id string) *action.Graph {
	m.Lock()
	defer m.Unlock()
	return m.container[id]
}

func (m *graphMap) Remove(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.container, id)
}

type cmdMap struct {
	sync.Mutex
	container map[string]*exec.Cmd
}

func newCmdMap() *cmdMap {
	return &cmdMap{
		container: make(map[string]*exec.Cmd),
	}
}

func (m *cmdMap) PutIfAbsent(id string, cmd *exec.Cmd) bool {
	m.Lock()
	defer m.Unlock()
	_, b := m.container[id]
	if b {
		return false
	}
	m.container[id] = cmd
	return true
}

func (m *cmdMap) GetById(id string) *exec.Cmd {
	m.Lock()
	defer m.Unlock()
	return m.container[id]
}

func (m *cmdMap) Remove(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.container, id)
}

func (m *cmdMap) GetAll() map[string]*exec.Cmd {
	m.Lock()
	defer m.Unlock()
	ret := make(map[string]*exec.Cmd, len(m.container))
	for k, v := range m.container {
		ret[k] = v
	}
	return ret
}
