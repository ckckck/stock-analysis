package agent

import (
	"github.com/run-bigpig/jcp/internal/models"
)

// ExpertAgent 专家Agent封装
type ExpertAgent struct {
	Config  *models.AgentConfig
	Enabled bool
}

// NewExpertAgent 创建专家Agent
func NewExpertAgent(config *models.AgentConfig) *ExpertAgent {
	return &ExpertAgent{
		Config:  config,
		Enabled: config.Enabled,
	}
}

// GetID 获取Agent ID
func (e *ExpertAgent) GetID() string {
	return e.Config.ID
}

// GetName 获取Agent名称
func (e *ExpertAgent) GetName() string {
	return e.Config.Name
}

// GetRole 获取Agent角色
func (e *ExpertAgent) GetRole() string {
	return e.Config.Role
}

// GetInstruction 获取Agent指令
func (e *ExpertAgent) GetInstruction() string {
	return e.Config.Instruction
}
