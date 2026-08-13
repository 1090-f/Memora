import type { Message } from '../types';

/**
 * groupConsecutiveAssistantMessages 将连续的助手消息按时间顺序合并为版本历史。
 * 
 * 规则：
 * 1. 遍历消息列表，识别用户消息之间的连续助手消息块
 * 2. 每个块内的助手消息按 created_at 升序排序
 * 3. 最旧的消息作为主消息（content），其余作为历史版本（versions）
 * 4. 默认显示最新版本（current_version_index = -1 表示 content）
 * 
 * 示例：
 * 输入：[User1, Asst1, Asst2, Asst3, User2, Asst4, Asst5]
 * 输出：[User1, Asst3_merged(versions=[Asst1, Asst2], content=Asst3), User2, Asst5_merged(versions=[Asst4], content=Asst5)]
 */
export function groupConsecutiveAssistantMessages(messages: Message[]): Message[] {
  if (messages.length === 0) return [];

  const result: Message[] = [];
  let currentBlock: Message[] = [];

  for (let i = 0; i < messages.length; i++) {
    const msg = messages[i];

    if (msg.role === 'assistant') {
      // 收集连续的助手消息
      currentBlock.push(msg);
    } else {
      // 遇到用户消息，先处理之前累积的助手消息块
      if (currentBlock.length > 0) {
        result.push(...mergeAssistantBlock(currentBlock));
        currentBlock = [];
      }
      // 用户消息直接追加
      result.push(msg);
    }
  }

  // 处理最后一个助手消息块（如果存在）
  if (currentBlock.length > 0) {
    result.push(...mergeAssistantBlock(currentBlock));
  }

  return result;
}

/**
 * mergeAssistantBlock 将一组连续的助手消息合并为单条消息+版本历史。
 * - 按 created_at 升序排序
 * - 如果只有一条消息，直接返回（不需要版本）
 * - 多条消息时：最新的作为 content，旧的放入 versions
 */
function mergeAssistantBlock(block: Message[]): Message[] {
  if (block.length === 0) return [];
  if (block.length === 1) return block; // 单条消息无需合并

  // 按创建时间升序排序（最旧的在前）
  const sorted = [...block].sort((a, b) => 
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  );

  // 最新的消息（最后一个）作为主消息
  const latestMsg = sorted[sorted.length - 1];
  // 其余的作为历史版本
  const olderVersions = sorted.slice(0, -1).map(m => ({
    content: m.content,
    agent_run_id: m.agent_run_id || '',
    status: m.status,
    created_at: m.created_at,
  }));

  // 返回合并后的消息：只保留最新的一条，但附带历史版本
  return [{
    ...latestMsg,
    versions: olderVersions,
    current_version_index: -1, // -1 表示显示最新版本（content）
  }];
}
