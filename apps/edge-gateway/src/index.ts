export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const requestId = crypto.randomUUID();
    const url = new URL(request.url);

    // Health check passthrough
    if (url.pathname === '/health') {
      return new Response(JSON.stringify({ status: 'ok', request_id: requestId }), {
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': requestId }
      });
    }

    // Rate limit check (basic)
    // ... (placeholder)

    // Build origin URL
    const originUrl = new URL(url.pathname + url.search, `http://${env.ORIGIN_HOST}:${env.DATA_PLANE_PORT}`);

    // Forward request to origin
    const headers = new Headers(request.headers);
    headers.set('X-Request-ID', requestId);
    headers.set('X-Forwarded-For', request.headers.get('CF-Connecting-IP') || '');
    headers.delete('CF-Access-Client-Id');  // Never leak Cloudflare Access identity

    const originResponse = await fetch(originUrl.toString(), {
      method: request.method,
      headers: headers,
      body: request.body,
      redirect: 'manual',  // Do NOT follow redirects from origin
    });

    // Build response
    const response = new Response(originResponse.body, originResponse);
    response.headers.set('X-Request-ID', requestId);

    // Security headers
    response.headers.set('X-Content-Type-Options', 'nosniff');
    response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
    response.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
    response.headers.set('X-Frame-Options', 'DENY');

    // Strip any origin headers
    response.headers.delete('Server');
    response.headers.delete('X-Powered-By');

    return response;
  },
};

interface Env {
  ORIGIN_HOST: string;
  DATA_PLANE_PORT: string;
}
