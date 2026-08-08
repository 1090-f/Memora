export class AppError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly httpStatus: number,
    public readonly details: unknown,
    public readonly requestId: string,
  ) {
    super(message);
    this.name = 'AppError';
    Object.setPrototypeOf(this, AppError.prototype);
  }
}

/** 返回错误消息，若后端在 details 中附带了原因说明则一并展示。 */
export function errorMessage(error: unknown): string {
  if (!(error instanceof AppError)) {
    return error instanceof Error ? error.message : '操作失败';
  }
  const details = error.details as { reason?: string } | null | undefined;
  if (details?.reason) return `${error.message}：${details.reason}`;
  return error.message;
}
