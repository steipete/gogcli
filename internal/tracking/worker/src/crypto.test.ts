import { describe, it, expect } from 'vitest';
import { importKey, encrypt, decrypt } from './crypto';

describe('crypto', () => {
  const testKey = 'MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE='; // 32 bytes base64

  it('encrypts and decrypts payload', async () => {
    const key = await importKey(testKey);
    const payload = { r: 'test@example.com', s: 'abc123', t: 1704067200 };

    const encrypted = await encrypt(payload, key);
    const decrypted = await decrypt(encrypted, key);

    expect(decrypted).toEqual(payload);
  });

  it('produces URL-safe base64', async () => {
    const key = await importKey(testKey);
    const payload = { r: 'test@example.com', s: 'abc123', t: 1704067200 };

    const encrypted = await encrypt(payload, key);

    expect(encrypted).not.toMatch(/[+/=]/);
  });

  it('throws on invalid ciphertext', async () => {
    const key = await importKey(testKey);

    await expect(decrypt('invalid', key)).rejects.toThrow();
  });
});
