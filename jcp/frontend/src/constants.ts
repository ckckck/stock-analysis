import { Agent, AgentRole } from './types';

export const AVAILABLE_AGENTS: Agent[] = [
  { id: 'bull', name: '大牛', role: AgentRole.BULL, avatar: '牛', color: 'bg-green-600' },
  { id: 'bear', name: '空头博士', role: AgentRole.BEAR, avatar: '熊', color: 'bg-red-600' },
  { id: 'quant', name: '算法师', role: AgentRole.QUANT, avatar: '算', color: 'bg-blue-600' },
  { id: 'macro', name: '宏观姐', role: AgentRole.MACRO, avatar: '宏', color: 'bg-purple-600' },
  { id: 'news', name: '情报员', role: AgentRole.NEWS, avatar: '报', color: 'bg-yellow-600' },
];
