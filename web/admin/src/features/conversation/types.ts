export interface Conversation {
  id: string;
  knowledge_base_id: string;
  chat_model_id: string;
  title: string;
  created_at: string;
  updated_at?: string;
}

export interface Citation {
  source_type: 'knowledge_base' | 'network';
  document_id?: string;
  document_title?: string;
  quoted_text?: string;
  title?: string;
  url?: string;
  site_name?: string;
}

/**
 * MessageVersion 消息历史版本，记录重试产生的旧版本内容。
 * 仅前端维护，不持久化到后端。
 */
export interface MessageVersion {
  /** 历史版本内容 */
  content: string;
  /** 对应版本的 agent_run_id */
  agent_run_id: string;
  /** 版本状态 */
  status?: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
  /** 版本创建时间 */
  created_at: string;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  agent_run_id: string | null;
  status?: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
  citations?: Citation[];
  /** 重试历史版本列表（仅前端维护，不持久化）。current_version_index >= 0 时显示对应版本内容 */
  versions?: MessageVersion[];
  /** 当前展示的历史版本索引。undefined/-1 表示显示 content（最新版本） */
  current_version_index?: number;
  created_at: string;
}

export interface QuestionResponse {
  run_id: string;
  user_message_id: string;
  status: 'queued';
  events_url: string;
}
