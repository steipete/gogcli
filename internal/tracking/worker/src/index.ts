import type { Env, PixelPayload } from './types';
import { importKey, decrypt } from './crypto';
import { detectBot } from './bot';
import { pixelResponse } from './pixel';

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    const path = url.pathname;

    try {
      // Pixel endpoint: GET /p/:blob.gif
      if (path.startsWith('/p/') && path.endsWith('.gif')) {
        return await handlePixel(request, env, path);
      }

      // Query endpoint: GET /q/:blob
      if (path.startsWith('/q/')) {
        return await handleQuery(request, env, path);
      }

      // Admin opens endpoint: GET /opens
      if (path === '/opens') {
        return await handleAdminOpens(request, env, url);
      }

      // Health check
      if (path === '/health') {
        return new Response('ok', { status: 200 });
      }

      return new Response('Not Found', { status: 404 });
    } catch (error) {
      console.error('Handler error:', error);
      return new Response('Internal Error', { status: 500 });
    }
  },
};

async function handlePixel(request: Request, env: Env, path: string): Promise<Response> {
  // Extract blob from /p/:blob.gif
  const blob = path.slice(3, -4); // Remove '/p/' and '.gif'

  const key = await importKey(env.TRACKING_KEY);
  let payload: PixelPayload;

  try {
    payload = await decrypt(blob, key);
  } catch {
    // Still return pixel even if decryption fails (don't break email display)
    return pixelResponse();
  }

  // Get request metadata
  const ip = request.headers.get('CF-Connecting-IP') || 'unknown';
  const userAgent = request.headers.get('User-Agent') || 'unknown';
  const cf = (request as any).cf || {};

  // Calculate time since delivery
  const now = Date.now();
  const sentAt = payload.t * 1000; // Convert to ms
  const timeSinceDelivery = now - sentAt;

  // Detect bots
  const { isBot, botType } = detectBot(userAgent, ip, timeSinceDelivery);

  // Log to D1
  await env.DB.prepare(`
    INSERT INTO opens (
      recipient, subject_hash, sent_at, opened_at,
      ip, user_agent, country, region, city, timezone,
      is_bot, bot_type
    ) VALUES (?, ?, ?, datetime('now'), ?, ?, ?, ?, ?, ?, ?, ?)
  `).bind(
    payload.r,
    payload.s,
    new Date(sentAt).toISOString(),
    ip,
    userAgent,
    cf.country || null,
    cf.region || null,
    cf.city || null,
    cf.timezone || null,
    isBot ? 1 : 0,
    botType
  ).run();

  return pixelResponse();
}

async function handleQuery(request: Request, env: Env, path: string): Promise<Response> {
  const blob = path.slice(3); // Remove '/q/'

  const key = await importKey(env.TRACKING_KEY);
  let payload: PixelPayload;

  try {
    payload = await decrypt(blob, key);
  } catch {
    return new Response('Invalid tracking ID', { status: 400 });
  }

  const result = await env.DB.prepare(`
    SELECT
      opened_at, ip, city, region, country, timezone, is_bot, bot_type
    FROM opens
    WHERE recipient = ? AND subject_hash = ? AND sent_at = ?
    ORDER BY opened_at ASC
  `).bind(
    payload.r,
    payload.s,
    new Date(payload.t * 1000).toISOString()
  ).all();

  const opens = result.results.map((row: any) => ({
    at: row.opened_at,
    is_bot: row.is_bot === 1,
    bot_type: row.bot_type,
    location: row.city ? {
      city: row.city,
      region: row.region,
      country: row.country,
      timezone: row.timezone,
    } : null,
  }));

  const humanOpens = opens.filter((o: any) => !o.is_bot);

  return Response.json({
    recipient: payload.r,
    sent_at: new Date(payload.t * 1000).toISOString(),
    opens,
    total_opens: opens.length,
    human_opens: humanOpens.length,
    first_human_open: humanOpens[0] || null,
  });
}

async function handleAdminOpens(request: Request, env: Env, url: URL): Promise<Response> {
  // Verify admin key
  const authHeader = request.headers.get('Authorization');
  if (!authHeader || authHeader !== `Bearer ${env.ADMIN_KEY}`) {
    return new Response('Unauthorized', { status: 401 });
  }

  const recipient = url.searchParams.get('recipient');
  const since = url.searchParams.get('since');
  const limit = parseInt(url.searchParams.get('limit') || '100', 10);

  let query = 'SELECT * FROM opens WHERE 1=1';
  const params: any[] = [];

  if (recipient) {
    query += ' AND recipient = ?';
    params.push(recipient);
  }

  if (since) {
    query += ' AND opened_at >= ?';
    params.push(since);
  }

  query += ' ORDER BY opened_at DESC LIMIT ?';
  params.push(limit);

  const result = await env.DB.prepare(query).bind(...params).all();

  return Response.json({
    opens: result.results.map((row: any) => ({
      recipient: row.recipient,
      subject_hash: row.subject_hash,
      sent_at: row.sent_at,
      opened_at: row.opened_at,
      is_bot: row.is_bot === 1,
      bot_type: row.bot_type,
      location: row.city ? {
        city: row.city,
        region: row.region,
        country: row.country,
      } : null,
    })),
  });
}
