import { describe, expect, it } from 'vitest';
import { knowledgeStatusLabel } from './knowledgeStatus';

describe('knowledgeStatusLabel', () => {
  it.each([
    ['sufficient', '充分'],
    ['insufficient', '不足'],
    ['ambiguous', '不确定'],
    [null, '未评估'],
    [undefined, '未评估'],
  ] as const)('maps %s to %s', (status, label) => {
    expect(knowledgeStatusLabel(status)).toBe(label);
  });
});
