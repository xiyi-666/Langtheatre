export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const target = new URL(env.ORIGIN_API);
    target.pathname = url.pathname;
    target.search = url.search;

    const response = await fetch(target.toString(), {
      method: request.method,
      headers: request.headers,
      body: request.body
    });

    return new Response(response.body, response);
  }
};
