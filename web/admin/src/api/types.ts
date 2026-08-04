export interface ApiEnvelope<T> {
  code: string;
  message: string;
  data?: T;
  details?: unknown;
  request_id: string;
}
