package agent

import (
	"sync"

	"github.com/run-bigpig/jcp/internal/models"
)

// Container 专家容器
type Container struct {
	agents map[string]*ExpertAgent
	mu     sync.RWMutex
}

// NewContainer 创建专家容器
func NewContainer() *Container {
	return &Container{
		agents: make(map[string]*ExpertAgent),
	}
}

// LoadAgents 加载Agent配置到容器
func (c *Container) LoadAgents(configs []models.AgentConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range configs {
		c.agents[configs[i].ID] = NewExpertAgent(&configs[i])
	}
}

// GetAgent 获取指定Agent
func (c *Container) GetAgent(id string) *ExpertAgent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agents[id]
}

// GetAgentsByIDs 根据ID列表获取Agent
func (c *Container) GetAgentsByIDs(ids []string) []*ExpertAgent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*ExpertAgent
	for _, id := range ids {
		if agent, ok := c.agents[id]; ok {
			result = append(result, agent)
		}
	}
	return result
}

// GetAllAgents 获取所有Agent
func (c *Container) GetAllAgents() []*ExpertAgent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*ExpertAgent, 0, len(c.agents))
	for _, agent := range c.agents {
		result = append(result, agent)
	}
	return result
}
