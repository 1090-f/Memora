import SSEClient from '@/utils/fetch';
import { DomainNodeSummaryReq } from '@/request/types';

export type StreamSummaryEvent = {
  type: 'data' | 'done' | 'error';
  content?: string;
  error?: string;
};

export const createNodeSummaryStream = (options?: {
  onOpen?: () => void;
  onError?: (error: Error) => void;
  onComplete?: () => void;
}) =>
  new SSEClient<StreamSummaryEvent>({
    url: '/api/v1/node/summary/stream',
    responseMode: 'sse-json',
    onOpen: options?.onOpen,
    onError: options?.onError,
    onComplete: options?.onComplete,
  });

export const subscribeNodeSummaryStream = (
  client: SSEClient<StreamSummaryEvent>,
  body: DomainNodeSummaryReq,
  onMessage: (event: StreamSummaryEvent) => void,
) => {
  client.subscribe(JSON.stringify(body), onMessage);
};
