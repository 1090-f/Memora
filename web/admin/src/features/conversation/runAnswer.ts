import { getAgentRun } from '@/features/agent-run/api';
import type { AgentRun } from '@/features/agent-run/types';

/** 轮询间隔与上限：跨过「SSE 终态事件先于 DB 落库」的竞态窗口。 */
const RETRY_INTERVAL_MS = 300;
const MAX_RETRY = 8; // 最坏 ≈ 2.4s，仅在后端异常慢时才走满

/**
 * 终态后取回答：统一 finalizeRun / runAgentStream / send 三处取值逻辑。
 *
 * 取值优先级：REST 的 final_result（权威、完整） > 事件流累积的 streamAnswer（兜底）。
 *
 * 为什么 REST 优先：ReAct 模式下 answer.delta 每轮发的是「当轮完整 Content」，
 * 而前端是字符串累加，多轮都产文本时 streamAnswer 会重复拼接。因此只要有 final_result，
 * 就必须以它为准。
 *
 * 为什么需要轮询：后端先发 `agent.run.completed` 事件（payload 刻意不含 final_result），
 * 之后才 MarkCompleted 写 DB。前端收到终态事件会立即掐断 SSE 并查询，
 * 可能拿到 status=running + final_result=null。轮询用于跨过这个窗口。
 */
export async function fetchRunAnswer(
  runId: string,
  streamAnswer?: string,
): Promise<{ answer: string; run: AgentRun }> {
  let run = await getAgentRun(runId);
  let answer = pickAnswer(run.final_result, streamAnswer);

  for (let attempt = 0; attempt < MAX_RETRY && !answer; attempt += 1) {
    // 已确定没有回答（失败/取消），后端不会再写入，无需继续等待。
    if (run.status === 'failed' || run.status === 'cancelled') break;
    await sleep(RETRY_INTERVAL_MS);
    run = await getAgentRun(runId);
    answer = pickAnswer(run.final_result, streamAnswer);
  }

  return { answer, run };
}

/** 非空（忽略纯空白）优先；返回值保持原样，不做 trim。 */
function pickAnswer(
  finalResult: string | null | undefined,
  streamAnswer: string | undefined,
): string {
  if (typeof finalResult === 'string' && finalResult.trim() !== '') return finalResult;
  if (typeof streamAnswer === 'string' && streamAnswer.trim() !== '') return streamAnswer;
  return '';
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
