export interface SwaggerRequest {
  url: string
  [key: string]: unknown
}

// rebaseSwaggerRequest moves a Swagger request onto the configured API base
// while preserving the generated operation path, query, and request options.
export function rebaseSwaggerRequest<T extends SwaggerRequest>(
  request: T,
  apiBaseUrl: string,
  currentOrigin: string,
): T {
  const sourceUrl = new URL(request.url, currentOrigin)
  const targetBase = new URL(`${apiBaseUrl.replace(/\/$/, '')}/`, currentOrigin)
  const targetUrl = new URL(sourceUrl.pathname.replace(/^\/+/, ''), targetBase)
  targetUrl.search = sourceUrl.search

  return {
    ...request,
    url: targetUrl.toString(),
  }
}
