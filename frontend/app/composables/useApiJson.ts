type ApiFetch = ReturnType<typeof useApiFetch>
type ApiFetchOptions = NonNullable<Parameters<ApiFetch>[1]>

type JsonRequestOptions = Omit<ApiFetchOptions, 'body' | 'headers'> & {
  headers?: Record<string, string>
}

// useApiJson returns an API helper for JSON request bodies.
export function useApiJson() {
  const apiFetch = useApiFetch()

  // apiJson serializes the request body and adds JSON headers.
  return async function apiJson<T>(
    request: string,
    body: unknown,
    options: JsonRequestOptions = {},
  ) {
    return await apiFetch<T>(request, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      body: JSON.stringify(body),
    } as ApiFetchOptions)
  }
}
