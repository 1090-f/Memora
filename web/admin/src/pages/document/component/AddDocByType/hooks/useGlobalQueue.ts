import { useCallback, useRef, useState } from 'react';

interface QueueTask {
  fn: () => Promise<any>;
  resolve: (value: any) => void;
  reject: (reason?: any) => void;
}
/**
 * 全局队列管理 Hook
 * 统一管理所有异步操作的并发控制
 * @param maxConcurrency 最大并发数，默认为 5
 * @returns {
 *   enqueue: 将任务加入队列并执行,
 *   clearQueue: 清空队列,
 *   getStatus: 获取队列状态,
 *   running: 正在运行的任务数,
 *   queueLength: 队列中的任务数,
 *   isIdle: 队列是否空闲
 * }
 */
export const useGlobalQueue = (maxConcurrency: number = 5) => {
  const [running, setRunning] = useState(0);
  const [queueLength, setQueueLength] = useState(0);
  const runningRef = useRef(0);
  const queueRef = useRef<QueueTask[]>([]);

  const next = useCallback(() => {
    if (queueRef.current.length > 0 && runningRef.current < maxConcurrency) {
      runningRef.current++;
      setRunning(runningRef.current);

      const task = queueRef.current.shift()!;
      setQueueLength(queueRef.current.length);

      task
        .fn()
        .then(result => {
          task.resolve(result);
        })
        .catch(error => {
          task.reject(error);
        })
        .finally(() => {
          runningRef.current--;
          setRunning(runningRef.current);
          next();
        });
    }
  }, [maxConcurrency]);

  const enqueue = useCallback(
    <R>(fn: () => Promise<R>): Promise<R> => {
      return new Promise((resolve, reject) => {
        queueRef.current.push({
          fn,
          resolve,
          reject,
        });
        setQueueLength(queueRef.current.length);
        next();
      });
    },
    [next],
  );

  /**
   * 清空队列（不会中断正在执行的任务）
   */
  const clearQueue = useCallback(() => {
    queueRef.current = [];
    setQueueLength(0);
  }, []);

  /**
   * 获取队列状态
   */
  const getStatus = useCallback(() => {
    return {
      running: runningRef.current,
      queueLength: queueRef.current.length,
      isIdle: runningRef.current === 0 && queueRef.current.length === 0,
    };
  }, []);

  return {
    enqueue,
    clearQueue,
    getStatus,
    running,
    queueLength,
    isIdle: running === 0 && queueLength === 0,
  };
};
