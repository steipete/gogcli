export interface Env {
  DB: D1Database;
  TRACKING_KEY: string;
  ADMIN_KEY: string;
}

export interface PixelPayload {
  r: string; // recipient
  s: string; // subject hash (first 6 chars)
  t: number; // sent timestamp (unix)
}

export interface OpenRecord {
  id: number;
  recipient: string;
  subject_hash: string;
  sent_at: string;
  opened_at: string;
  ip: string;
  user_agent: string;
  country: string | null;
  region: string | null;
  city: string | null;
  timezone: string | null;
  is_bot: number;
  bot_type: string | null;
}
