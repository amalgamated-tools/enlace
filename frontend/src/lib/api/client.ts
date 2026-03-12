const API_BASE = "/api/v1";

interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  fields?: Record<string, string>;
}

interface RequestOptions {
  overrideToken?: string;
}

class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public fields?: Record<string, string>,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  endpoint: string,
  options: RequestInit = {},
  requestOptions: RequestOptions = {},
): Promise<T> {
  const token =
    requestOptions.overrideToken ?? localStorage.getItem("access_token");

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  const data: ApiResponse<T> = await response.json();

  if (!response.ok || !data.success) {
    throw new ApiError(
      data.error || "Request failed",
      response.status,
      data.fields,
    );
  }

  return data.data as T;
}

export const api = {
  get: <T>(endpoint: string, requestOptions?: RequestOptions) =>
    request<T>(endpoint, {}, requestOptions),
  post: <T>(endpoint: string, body: unknown, requestOptions?: RequestOptions) =>
    request<T>(
      endpoint,
      { method: "POST", body: JSON.stringify(body) },
      requestOptions,
    ),
  patch: <T>(
    endpoint: string,
    body: unknown,
    requestOptions?: RequestOptions,
  ) =>
    request<T>(
      endpoint,
      { method: "PATCH", body: JSON.stringify(body) },
      requestOptions,
    ),
  put: <T>(endpoint: string, body: unknown, requestOptions?: RequestOptions) =>
    request<T>(
      endpoint,
      { method: "PUT", body: JSON.stringify(body) },
      requestOptions,
    ),
  delete: <T>(endpoint: string, requestOptions?: RequestOptions) =>
    request<T>(endpoint, { method: "DELETE" }, requestOptions),
};

export { ApiError };
