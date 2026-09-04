import type { AgentRun } from './types';

export function knowledgeStatusLabel(status: AgentRun['knowledge_status']) {
  switch (status) {
    case 'sufficient': return '充分';
    case 'insufficient': return '不足';
    case 'ambiguous': return '不确定';
    default: return '未评估';
  }
}
