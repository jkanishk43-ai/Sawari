// Cloudflare Worker: API Gateway Proxy
// Handles authentication, rate limiting, and request routing

const RATE_LIMIT = 100; // requests per minute
const ALLOWED_ORIGINS = [
  'https://sawaari.in',
  'https://www.sawaari.in',
  'https://app.sawaari.in',
];

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // CORS preflight
    if (request.method === 'OPTIONS') {
      return handleCors(request);
    }

    // Extract client IP for rate limiting
    const clientIP = request.headers.get('CF-Connecting-IP') || 'unknown';

    // Rate limiting using KV
    const rateLimitKey = `rate:${clientIP}:${Math.floor(Date.now() / 60000)}`;
    const current = await env.RATE_LIMIT.get(rateLimitKey);
    const count = parseInt(current || '0', 10);

    if (count >= RATE_LIMIT) {
      return new Response(JSON.stringify({
        error: 'Rate limit exceeded',
        retryAfter: 60 - Math.floor((Date.now() % 60000) / 1000),
      }), {
        status: 429,
        headers: {
          'Content-Type': 'application/json',
          'X-RateLimit-Limit': RATE_LIMIT.toString(),
          'X-RateLimit-Remaining': '0',
          'Retry-After': '60',
        },
      });
    }

    // Increment rate limit counter
    await env.RATE_LIMIT.put(rateLimitKey, (count + 1).toString(), { expirationTtl: 120 });

    // Validate API key from header
    const apiKey = request.headers.get('X-API-Key');
    if (!apiKey) {
      return new Response(JSON.stringify({ error: 'API key required' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    // Verify API key
    const keyData = await env.API_KEYS.get(apiKey);
    if (!keyData) {
      return new Response(JSON.stringify({ error: 'Invalid API key' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    // Forward to Kong backend
    const upstreamUrl = `${env.KONG_URL}${url.pathname}${url.search}`;
    const upstreamRequest = new Request(upstreamUrl, {
      method: request.method,
      headers: {
        'Content-Type': request.headers.get('Content-Type') || 'application/json',
        'X-Real-IP': clientIP,
        'X-API-Key': apiKey,
        'X-Client-ID': JSON.parse(keyData).clientId,
      },
      body: request.method !== 'GET' && request.method !== 'HEAD'
        ? await request.text()
        : undefined,
    });

    const response = await fetch(upstreamRequest);

    // Add response headers
    const newHeaders = new Headers(response.headers);
    newHeaders.set('X-RateLimit-Limit', RATE_LIMIT.toString());
    newHeaders.set('X-RateLimit-Remaining', Math.max(0, RATE_LIMIT - count - 1).toString());
    newHeaders.set('X-CF-Worker', 'api-proxy');

    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers: newHeaders,
    });
  },
};

function handleCors(request) {
  const origin = request.headers.get('Origin');
  const allowedOrigin = ALLOWED_ORIGINS.includes(origin) ? origin : ALLOWED_ORIGINS[0];

  return new Response(null, {
    status: 204,
    headers: {
      'Access-Control-Allow-Origin': allowedOrigin,
      'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-API-Key, X-Request-ID',
      'Access-Control-Max-Age': '86400',
    },
  });
}

export interface Env {
  KONG_URL: string;
  RATE_LIMIT: KVNamespace;
  API_KEYS: KVNamespace;
}
