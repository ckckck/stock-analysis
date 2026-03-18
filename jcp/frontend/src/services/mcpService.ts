import { models } from '../../wailsjs/go/models';
import { GetMCPServers, AddMCPServer, UpdateMCPServer, DeleteMCPServer, GetMCPStatus, TestMCPConnection, GetMCPServerTools } from '../../wailsjs/go/main/App';

export type MCPServerConfig = models.MCPServerConfig;

// MCP 服务器状态
export interface MCPServerStatus {
  id: string;
  connected: boolean;
  error: string;
}

// MCP 工具信息
export interface MCPToolInfo {
  name: string;
  description: string;
  serverId: string;
  serverName: string;
}

export async function getMCPServers(): Promise<MCPServerConfig[]> {
  return await GetMCPServers();
}

export async function addMCPServer(server: MCPServerConfig): Promise<string> {
  return await AddMCPServer(server as any);
}

export async function updateMCPServer(server: MCPServerConfig): Promise<string> {
  return await UpdateMCPServer(server as any);
}

export async function deleteMCPServer(id: string): Promise<string> {
  return await DeleteMCPServer(id);
}

// 获取所有 MCP 服务器状态
export async function getMCPStatus(): Promise<MCPServerStatus[]> {
  return await GetMCPStatus();
}

// 测试指定 MCP 服务器连接
export async function testMCPConnection(serverID: string): Promise<MCPServerStatus> {
  return await TestMCPConnection(serverID);
}

// 获取指定 MCP 服务器的工具列表
export async function getMCPServerTools(serverID: string): Promise<MCPToolInfo[]> {
  return await GetMCPServerTools(serverID);
}
