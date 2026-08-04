import { describe, expect, it } from 'vitest';
import html from '../../index.html?raw';

describe('Memora frontend baseline', () => {
  it('presents Memora as the browser product title', () => {
    const page = new DOMParser().parseFromString(html, 'text/html');

    expect(page.title).toBe('Memora');
  });
});
