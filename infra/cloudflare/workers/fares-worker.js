// Cloudflare Worker: Transit Tile Caching
// Caches GTFS-RT tiles and map tiles at the edge for low-latency access

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // Cache TTLs by tile type
    const cacheRules = {
      'tiles': { ttl: 86400, sw: 3600 },        // Static map tiles: 24h cache, 1h stale-while-revalidate
      'gtfs-rt': { ttl: 30, sw: 10 },            // Real-time updates: 30s cache
      'fare-tables': { ttl: 3600, sw: 300 },    // Fare tables: 1h cache
      'stops': { ttl: 3600, sw: 600 },           // Stop data: 1h cache
    };

    const cacheKey = new Request(url.toString(), request);
    const cache = caches.default;

    // Try cache first
    let response = await cache.match(cacheKey);

    if (response) {
      // Add cache hit header
      const newHeaders = new Headers(response.headers);
      newHeaders.set('X-Cache', 'HIT');
      return new Response(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers: newHeaders,
      });
    }

    // Fetch from origin
    response = await fetch(request);

    // Only cache successful responses
    if (response.status === 200) {
      // Determine cache tier based on path
      let tier = 'fare-tables';
      if (url.pathname.includes('/tiles/')) tier = 'tiles';
      else if (url.pathname.includes('/gtfs-rt')) tier = 'gtfs-rt';
      else if (url.pathname.includes('/stops')) tier = 'stops';

      const rule = cacheRules[tier];

      // Create new response with cache headers
      const newResponse = new Response(response.body, response);
      newResponse.headers.set('Cache-Control', `public, max-age=${rule.ttl}, stale-while-revalidate=${rule.sw}`);
      newResponse.headers.set('X-Cache', 'MISS');
      newResponse.headers.set('X-Cache-Tier', tier);

      ctx.waitUntil(cache.put(cacheKey, newResponse.clone()));
      return newResponse;
    }

    return response;
  },
};

// TypeScript bindings for Cloudflare Workers
export interface Env {
  UPSTREAM_HOST: string;
  KONG_URL: string;
}
