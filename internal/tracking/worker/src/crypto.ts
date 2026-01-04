import type { PixelPayload } from './types';

const ALGORITHM = 'AES-GCM';
const IV_LENGTH = 12;

export async function importKey(base64Key: string): Promise<CryptoKey> {
  const keyBytes = Uint8Array.from(atob(base64Key), c => c.charCodeAt(0));
  return crypto.subtle.importKey(
    'raw',
    keyBytes,
    { name: ALGORITHM },
    false,
    ['encrypt', 'decrypt']
  );
}

export async function decrypt(blob: string, key: CryptoKey): Promise<PixelPayload> {
  // URL-safe base64 decode
  const base64 = blob.replace(/-/g, '+').replace(/_/g, '/');
  const padded = base64 + '='.repeat((4 - base64.length % 4) % 4);
  const combined = Uint8Array.from(atob(padded), c => c.charCodeAt(0));

  const iv = combined.slice(0, IV_LENGTH);
  const ciphertext = combined.slice(IV_LENGTH);

  const decrypted = await crypto.subtle.decrypt(
    { name: ALGORITHM, iv },
    key,
    ciphertext
  );

  const text = new TextDecoder().decode(decrypted);
  return JSON.parse(text) as PixelPayload;
}

export async function encrypt(payload: PixelPayload, key: CryptoKey): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(IV_LENGTH));
  const encoded = new TextEncoder().encode(JSON.stringify(payload));

  const ciphertext = await crypto.subtle.encrypt(
    { name: ALGORITHM, iv },
    key,
    encoded
  );

  const combined = new Uint8Array(IV_LENGTH + ciphertext.byteLength);
  combined.set(iv);
  combined.set(new Uint8Array(ciphertext), IV_LENGTH);

  // URL-safe base64 encode
  const base64 = btoa(String.fromCharCode(...combined));
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
